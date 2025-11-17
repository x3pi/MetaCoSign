package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// BlockRequestProtocol định danh cho protocol yêu cầu block.
const BlockRequestProtocol = "/block-request/1.0.0"

func (node *HostNode) blockRequestHandler(stream network.Stream) {
	// remotePeer := stream.Conn().RemotePeer()

	defer func() {
		_ = stream.Close()
	}()

	reader := bufio.NewReader(stream)
	writer := bufio.NewWriter(stream)

	if err := stream.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_, _ = writer.WriteString("ERROR: Internal server error (deadline)\n")
		_ = writer.Flush()
		return
	}

	blockNumberStr, err := reader.ReadString('\n')
	if err != nil {
		_, _ = writer.WriteString("ERROR: Could not read request\n")
		_ = writer.Flush()
		return
	}
	_ = stream.SetReadDeadline(time.Time{})

	blockNumberStr = strings.TrimSpace(blockNumberStr)
	if blockNumberStr == "" {
		_, _ = writer.WriteString("ERROR: Empty block number\n")
		_ = writer.Flush()
		return
	}

	blockNumber, err := strconv.ParseUint(blockNumberStr, 10, 64)
	if err != nil {
		_, _ = writer.WriteString("ERROR: Invalid block number format\n")
		_ = writer.Flush()
		return
	}

	blockData, err := node.GetBlockStorageLocal(blockNumber)
	if err != nil {
		errorMsg := fmt.Sprintf("ERROR: Block %d not found or unavailable\n", blockNumber)
		_, writeErr := writer.WriteString(errorMsg)
		if writeErr != nil {
			return
		}
		_ = writer.Flush()
		return
	}

	if err := stream.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}

	_, err = writer.Write(blockData)
	if err != nil {
		_ = stream.SetWriteDeadline(time.Time{})
		return
	}

	err = writer.Flush()
	if err != nil {
		_ = stream.SetWriteDeadline(time.Time{})
		return
	}
	_ = stream.SetWriteDeadline(time.Time{})
}

func (node *HostNode) RequestBlock(ctx context.Context, peerID peer.ID, blockNumber uint64) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	stream, err := node.Host.NewStream(reqCtx, peerID, BlockRequestProtocol)
	if err != nil {
		addrInfo := node.Host.Peerstore().PeerInfo(peerID)
		if len(addrInfo.Addrs) == 0 {
			node.reconnectMutex.Lock()
			pInfo, exists := node.Peers[peerID.String()]
			node.reconnectMutex.Unlock()
			if exists {
				addrInfo = pInfo.Info
			} else {
				return nil, fmt.Errorf("cannot open block request stream and no address info found for peer %s", peerID)
			}
		}

		connectCtx, connectCancel := context.WithTimeout(reqCtx, 15*time.Second)
		errConnect := node.Host.Connect(connectCtx, addrInfo)
		connectCancel()
		if errConnect != nil {
			return nil, fmt.Errorf("failed to connect to peer %s before block request: %w", peerID, errConnect)
		}

		stream, err = node.Host.NewStream(reqCtx, peerID, BlockRequestProtocol)
		if err != nil {
			return nil, fmt.Errorf("failed to open block request stream to %s after connecting: %w", peerID, err)
		}
	}
	defer stream.Close()

	writer := bufio.NewWriter(stream)
	reader := bufio.NewReader(stream)

	requestPayload := fmt.Sprintf("%d\n", blockNumber)

	if err := stream.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, fmt.Errorf("error setting write deadline for block request to %s: %w", peerID, err)
	}
	_, err = writer.WriteString(requestPayload)
	if err != nil {
		_ = stream.SetWriteDeadline(time.Time{})
		return nil, fmt.Errorf("failed to write block request %d: %w", blockNumber, err)
	}
	err = writer.Flush()
	if err != nil {
		_ = stream.SetWriteDeadline(time.Time{})
		return nil, fmt.Errorf("failed to flush block request %d: %w", blockNumber, err)
	}
	_ = stream.SetWriteDeadline(time.Time{})

	if err := stream.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, fmt.Errorf("error setting read deadline for block response from %s: %w", peerID, err)
	}

	responseData, err := io.ReadAll(reader)
	if err != nil {
		if ctxErr := reqCtx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("context error reading block response: %w", ctxErr)
		}
		return nil, fmt.Errorf("network error reading block response: %w", err)
	}
	_ = stream.SetReadDeadline(time.Time{})

	if len(responseData) > 6 && string(responseData[:6]) == "ERROR:" {
		errorMsg := strings.TrimSpace(string(responseData[6:]))
		return nil, fmt.Errorf("peer error: %s", errorMsg)
	}

	return responseData, nil
}

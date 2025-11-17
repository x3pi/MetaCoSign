package node

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

func (node *HostNode) SetFeeRequestHandler() {
	node.Host.SetStreamHandler(common.FreeFeeRequestProtocol, node.handleFeeRequestStream)
	logger.Info(fmt.Sprintf("Đã đăng ký stream handler cho protocol: %s", common.FreeFeeRequestProtocol))
}

func (node *HostNode) handleFeeRequestStream(s network.Stream) {
	defer func() {
		if err := s.Close(); err != nil {
			logger.Error(fmt.Sprintf("Lỗi khi đóng stream yêu cầu phí: %v", err))
		}
	}()

	addresses := node.GetFeeAddresses()

	addressListData, err := json.Marshal(addresses)
	if err != nil {
		errorMessage := fmt.Sprintf("Error marshaling fee addresses: %v", err)
		_, _ = s.Write([]byte(errorMessage))
		s.Reset()
		return
	}

	writer := io.Writer(s)
	_, err = writer.Write(addressListData)
	if err != nil {
		s.Reset()
		return
	}
}

func (node *HostNode) GetFeeAddressesFromMaster(ctx context.Context, masterPeerIDStr string) ([]string, error) {
	addr, err := peer.AddrInfoFromString(masterPeerIDStr)
	if err != nil {
		return nil, fmt.Errorf("không thể phân tích địa chỉ peer '%s': %w", masterPeerIDStr, err)
	}
	masterPeerID := addr.ID

	// Explicitly connect to the master peer's addresses to ensure connectivity.
	// This helps the Connectedness check pass more reliably.
	err = node.Host.Connect(ctx, *addr)
	if err != nil {
		logger.Error("Lỗi khi chủ động kết nối tới master: %v", err)
	}

	const maxRequestAttempts = 10
	const requestRetryDelay = 10 * time.Second

	var lastErr error

	for attempt := 0; attempt < maxRequestAttempts; attempt++ {
		logger.Info("Đang thử lần thứ %d...", attempt+1)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context đã bị hủy trước khi gửi yêu cầu: %w", ctx.Err())
		}

		if node.Host.Network().Connectedness(masterPeerID) != network.Connected {
			lastErr = fmt.Errorf("master peer %s chưa kết nối", masterPeerID.String())
			logger.Error(lastErr.Error())
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context bị hủy trong lúc chờ kết nối: %w", ctx.Err())
			case <-time.After(requestRetryDelay):
				_ = node.Host.Connect(ctx, *addr) // Try to reconnect
				continue
			}
		}

		streamCtx, streamCancel := context.WithTimeout(ctx, 5*time.Second)
		stream, err := node.Host.NewStream(streamCtx, masterPeerID, common.FreeFeeRequestProtocol)
		streamCancel() // Cancel the context as soon as the stream is opened or fails
		if err != nil {
			lastErr = fmt.Errorf("không thể mở stream tới master '%s': %w", masterPeerID, err)
			logger.Error(lastErr.Error())
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context bị hủy khi mở stream: %w", ctx.Err())
			case <-time.After(requestRetryDelay):
				continue
			}
		}

		readCtx, readCancel := context.WithTimeout(ctx, 15*time.Second)
		defer readCancel()

		var feeAddresses []string
		success := false

		// Using a block with a closure to manage scope and defer
		func() {
			defer func() {
				if stream != nil && !success {
					_ = stream.Reset()
				} else if stream != nil {
					_ = stream.Close()
				}
			}()

			reader := bufio.NewReader(stream)
			addressListDataChan := make(chan []byte)
			errChan := make(chan error)

			go func() {
				data, err := io.ReadAll(reader)
				if err != nil {
					errChan <- err
					return
				}
				addressListDataChan <- data
			}()

			var addressListData []byte
			select {
			case <-readCtx.Done():
				err = fmt.Errorf("hết thời gian chờ khi đọc dữ liệu từ stream hoặc context bị hủy: %w", readCtx.Err())
				lastErr = err
				logger.Error(err.Error())
				return
			case err = <-errChan:
				err = fmt.Errorf("lỗi khi đọc dữ liệu từ stream: %w", err)
				lastErr = err
				logger.Error(err.Error())
				return
			case addressListData = <-addressListDataChan:
			}

			err = json.Unmarshal(addressListData, &feeAddresses)
			if err != nil {
				err = fmt.Errorf("lỗi khi giải mã JSON phản hồi: %w", err)
				lastErr = err
				logger.Error(err.Error())
				panic("lỗi khi giải mã JSON phản hồi")
			}

			success = true
		}()

		if success {
			logger.Info("Lấy thành công danh sách địa chỉ: %v", feeAddresses)
			return feeAddresses, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context bị hủy sau lần thử %d: %w", attempt+1, ctx.Err())
		case <-time.After(requestRetryDelay):
			continue
		}
	}

	finalErr := fmt.Errorf("không thể lấy danh sách địa chỉ phí từ master '%s' sau %d lần thử. Lỗi cuối: %w",
		masterPeerIDStr, maxRequestAttempts, lastErr)
	logger.Error(finalErr.Error())
	// In a real application, a panic might be used if this is a critical, unrecoverable error at startup.
	panic("Get list free fee error!")
}

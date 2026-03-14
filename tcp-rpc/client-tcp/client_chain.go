package client

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	p_network "github.com/meta-node-blockchain/meta-node/pkg/network"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/tcp-rpc/client-tcp/command"
)

// ===================== Chain-Direct Methods =====================
// Gửi thẳng lên chain, dùng header ID matching, không qua RPC proxy

// sendChainRequest gửi command trực tiếp lên chain và đợi response theo header ID
func (client *Client) sendChainRequest(cmd string, body []byte, timeout time.Duration) ([]byte, error) {
	parentConn := client.clientContext.ConnectionsManager.ParentConnection()
	if parentConn == nil || !parentConn.IsConnect() {
		return nil, fmt.Errorf("parent connection not available")
	}

	id := uuid.New().String()
	respCh := make(chan []byte, 1)

	client.pendingChainRequests.Store(id, respCh)

	msg := p_network.NewMessage(&pb.Message{
		Header: &pb.Header{
			Command: cmd,
			ID:      id,
		},
		Body: body,
	})

	if err := parentConn.SendMessage(msg); err != nil {
		client.pendingChainRequests.Delete(id)
		return nil, fmt.Errorf("failed to send %s: %w", cmd, err)
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(timeout):
		client.pendingChainRequests.Delete(id)
		return nil, fmt.Errorf("timeout waiting for %s (id=%s)", cmd, id)
	}
}

// ChainGetChainId lấy chain ID trực tiếp từ chain (raw uint64)
func (client *Client) ChainGetChainId() (uint64, error) {
	resp, err := client.sendChainRequest(command.GetChainId, nil, 10*time.Second)
	if err != nil {
		return 0, err
	}
	if len(resp) < 8 {
		return 0, fmt.Errorf("invalid chain id response: %d bytes", len(resp))
	}
	chainId := binary.BigEndian.Uint64(resp)
	logger.Info("✅ ChainGetChainId: %d", chainId)
	return chainId, nil
}

// ChainGetBlockNumber lấy block number trực tiếp từ chain (raw uint64)
func (client *Client) ChainGetBlockNumber() (uint64, error) {
	resp, err := client.sendChainRequest(command.GetBlockNumber, nil, 10*time.Second)
	if err != nil {
		return 0, err
	}
	if len(resp) < 8 {
		return 0, fmt.Errorf("invalid block number response: %d bytes", len(resp))
	}
	bn := binary.BigEndian.Uint64(resp)
	logger.Info("✅ ChainGetBlockNumber: %d", bn)
	return bn, nil
}

// ChainGetTransactionReceipt lấy receipt trực tiếp từ chain theo txHash
// Trả về raw response bytes — caller tự unmarshal nếu cần
func (client *Client) ChainGetTransactionReceipt(txHash common.Hash) ([]byte, error) {
	resp, err := client.sendChainRequest(command.GetTransactionReceipt, txHash.Bytes(), 15*time.Second)
	if err != nil {
		return nil, err
	}
	logger.Info("✅ ChainGetTransactionReceipt: %s (%d bytes)", txHash.Hex(), len(resp))
	return resp, nil
}
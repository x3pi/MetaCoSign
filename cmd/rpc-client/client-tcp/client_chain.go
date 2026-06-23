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
	"github.com/meta-node-blockchain/meta-node/pkg/state"
	mt_transaction "github.com/meta-node-blockchain/meta-node/pkg/transaction"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp/command"
	"github.com/meta-node-blockchain/meta-node/types"
	t_network "github.com/meta-node-blockchain/meta-node/types/network"
)

// Chain-direct command constants
const (
	chainCmdGetChainId     = "GetChainId"
	chainCmdGetBlockNumber = "GetBlockNumber"
	defaultChainTimeout    = 60 * time.Second
)

// ===================== Chain-Direct Methods =====================
// Gửi thẳng lên chain qua TCP connection, dùng header ID matching

// sendChainRequest gửi command trực tiếp lên chain và đợi response theo header ID
func (client *Client) sendChainRequest(cmd string, body []byte, timeout time.Duration) (t_network.Message, error) {
	parentConn := client.clientContext.ConnectionsManager.ParentConnection()
	if parentConn == nil || !parentConn.IsConnect() {
		return nil, fmt.Errorf("parent connection not available")
	}

	id := uuid.New().String()
	respCh := make(chan t_network.Message, 1)

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
	respMsg, err := client.sendChainRequest(chainCmdGetChainId, nil, defaultChainTimeout)
	if err != nil {
		return 0, err
	}
	resp := respMsg.Body()
	if len(resp) < 8 {
		return 0, fmt.Errorf("invalid chain id response: %d bytes", len(resp))
	}
	chainId := binary.BigEndian.Uint64(resp)
	logger.Info("✅ ChainGetChainId: %d", chainId)
	return chainId, nil
}

// ChainGetBlockNumber lấy block number trực tiếp từ chain (raw uint64)
func (client *Client) ChainGetBlockNumber() (uint64, error) {
	respMsg, err := client.sendChainRequest(chainCmdGetBlockNumber, nil, defaultChainTimeout)
	if err != nil {
		return 0, err
	}
	resp := respMsg.Body()
	if len(resp) < 8 {
		return 0, fmt.Errorf("invalid block number response: %d bytes", len(resp))
	}
	bn := binary.BigEndian.Uint64(resp)
	logger.Info("✅ ChainGetBlockNumber: %d", bn)
	return bn, nil
}

// GetAccountState lấy account state, chờ phản hồi AccountState hoặc TransactionError qua ID-matching
func (client *Client) GetAccountState(address common.Address, timeout time.Duration) (types.AccountState, error) {
	respMsg, err := client.sendChainRequest(command.GetAccountState, address.Bytes(), timeout)
	if err != nil {
		return nil, fmt.Errorf("send error: %w", err)
	}

	cmd := respMsg.Command()
	if cmd == command.TransactionError {
		txErr := &mt_transaction.TransactionHashWithError{}
		if err := txErr.Unmarshal(respMsg.Body()); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction error: %w", err)
		}
		return nil, fmt.Errorf("transaction error: %s", txErr.Proto().Description)
	}

	if cmd == command.AccountState {
		accountState := &state.AccountState{}
		if err := accountState.Unmarshal(respMsg.Body()); err != nil {
			return nil, fmt.Errorf("failed to unmarshal account state: %w", err)
		}
		return accountState, nil
	}

	return nil, fmt.Errorf("unexpected command: %s", cmd)
}

// ChainGetNonce lấy nonce trực tiếp từ chain
func (client *Client) ChainGetNonce(address common.Address) (uint64, error) {
	respMsg, err := client.sendChainRequest(command.GetNonce, address.Bytes(), 10*time.Second)
	if err != nil {
		return 0, err
	}
	resp := respMsg.Body()
	var nonce uint64
	if len(resp) >= 8 {
		nonce = binary.BigEndian.Uint64(resp)
	}
	return nonce, nil
}

// ChainGetDeviceKey lấy last device key trực tiếp từ chain
func (client *Client) ChainGetDeviceKey(lastHash []byte) (types.LastDeviceKey, error) {
	respMsg, err := client.sendChainRequest(command.GetDeviceKey, lastHash, 10*time.Second)
	if err != nil {
		return types.LastDeviceKey{}, err
	}
	data := respMsg.Body()
	if len(data) != 64 && len(data) != 32 {
		return types.LastDeviceKey{}, fmt.Errorf("unable to parse wrong len: %d", len(data))
	}

	transactionHash := data[:32]
	var lastDeviceKeyFromServer []byte

	if len(data) == 32 {
		lastDeviceKeyFromServer = common.Hash{}.Bytes()
	} else {
		lastDeviceKeyFromServer = data[32:]
	}

	lastDeviceKey := types.LastDeviceKey{
		TransactionHash:         transactionHash,
		LastDeviceKeyFromServer: lastDeviceKeyFromServer,
	}
	return lastDeviceKey, nil
}

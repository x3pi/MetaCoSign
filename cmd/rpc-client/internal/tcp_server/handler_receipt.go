package tcp_server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/receipt"
	"github.com/meta-node-blockchain/meta-node/types"
	t_network "github.com/meta-node-blockchain/meta-node/types/network"
	"google.golang.org/protobuf/proto"
)

// handleGetTransactionReceipt - lấy receipt qua TCP trực tiếp từ chain
func (srv *RpcTcpServer) handleGetTransactionReceipt(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	var params []string
	body := request.Message().Body()
	if len(body) > 0 {
		_ = json.Unmarshal(body, &params)
	}

	if len(params) == 0 || params[0] == "" {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: missing transaction hash",
		})
	}

	txHash := params[0]
	connKey := request.Message().ToAddress().Hex()
	chainClient, err := srv.getOrCreateChainConn(connKey)
	if err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to get chain connection: " + err.Error(),
		})
	}

	txHashBytes := common.HexToHash(txHash)
	reqProto := &pb.GetTransactionReceiptRequest{
		TransactionHash: txHashBytes.Bytes(),
	}
	reqBytes, err := proto.Marshal(reqProto)
	if err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to marshal request: " + err.Error(),
		})
	}

	respBytes, err := chainClient.GetTransactionReceipt(reqBytes, 30*time.Second)
	if err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "GetTransactionReceipt TCP error: " + err.Error(),
		})
	}

	respProto := &pb.GetTransactionReceiptResponse{}
	if err := proto.Unmarshal(respBytes, respProto); err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to unmarshal receipt response: " + err.Error(),
		})
	}

	if respProto.Error != "" {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Chain error: " + respProto.Error,
		})
	}

	if respProto.Receipt == nil {
		return srv.sendRpcResponse(conn, msgID, nil, nil)
	}

	receiptProtoBytes, err := proto.Marshal(respProto.Receipt)
	if err != nil {
		logger.Error("❌ handleGetTransactionReceipt: proto marshal error: %v", err)
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to marshal receipt: " + err.Error(),
		})
	}

	logger.Info("✅ TCP eth_getTransactionReceipt (TCP-direct): %s (%d bytes proto)", txHash, len(receiptProtoBytes))

	resp := &pb.RpcResponse{
		Jsonrpc: "2.0",
		Id:      msgID,
		Result:  receiptProtoBytes,
	}
	return srv.sendTcpResponse(conn, resp)
}

// handleGetTransactionCount — lấy nonce (pending) từ chain qua TCP
func (srv *RpcTcpServer) handleGetTransactionCount(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	var params []interface{}
	body := request.Message().Body()
	if len(body) > 0 {
		_ = json.Unmarshal(body, &params)
	}

	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: missing address",
		})
	}

	addrStr, ok := params[0].(string)
	if !ok || addrStr == "" {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: address must be a string",
		})
	}

	address := common.HexToAddress(addrStr)
	logger.Info("🔄 TCP eth_getTransactionCount from %s (addr=%s)", conn.RemoteAddrSafe(), address.Hex())

	connKey := request.Message().ToAddress().Hex()
	chainClient, err := srv.getOrCreateChainConn(connKey)
	if err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "Failed to get chain connection: " + err.Error(),
		})
	}

	nonce, err := chainClient.GetNonce(address.Bytes(), 30*time.Second)
	if err != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32603,
			Message: "GetNonce TCP error: " + err.Error(),
		})
	}

	nonceHex := fmt.Sprintf("0x%x", nonce)
	resultBytes, _ := json.Marshal(nonceHex)

	logger.Info("✅ TCP eth_getTransactionCount: %s nonce=%s", address.Hex(), nonceHex)
	return srv.sendRpcResponse(conn, msgID, resultBytes, nil)
}

// receiptToRpcReceipt converts internal receipt to pb.RpcReceipt
func receiptToRpcReceipt(rcp *receipt.Receipt, tx types.Transaction) *pb.RpcReceipt {
	rpcReceipt := &pb.RpcReceipt{
		Status: fmt.Sprintf("0x%x", rcp.Status()),
	}
	if tx != nil {
		rpcReceipt.TransactionHash = tx.Hash().Hex()
	}
	// GasUsed
	rpcReceipt.GasUsed = fmt.Sprintf("0x%x", rcp.GasUsed())

	return rpcReceipt
}

// jsonToRpcReceipt convert JSON receipt map sang pb.RpcReceipt
func jsonToRpcReceipt(m map[string]interface{}) *pb.RpcReceipt {
	receipt := &pb.RpcReceipt{
		TransactionHash:   getStr(m, "transactionHash"),
		From:              getStr(m, "from"),
		To:                getStr(m, "to"),
		ContractAddress:   getStr(m, "contractAddress"),
		Status:            getStr(m, "status"),
		GasUsed:           getStr(m, "gasUsed"),
		CumulativeGasUsed: getStr(m, "cumulativeGasUsed"),
		EffectiveGasPrice: getStr(m, "effectiveGasPrice"),
		BlockHash:         getStr(m, "blockHash"),
		BlockNumber:       getStr(m, "blockNumber"),
		TransactionIndex:  getStr(m, "transactionIndex"),
		Type:              getStr(m, "type"),
		LogsBloom:         getStr(m, "logsBloom"),
	}

	// Parse logs
	if logsRaw, ok := m["logs"].([]interface{}); ok {
		for _, logRaw := range logsRaw {
			if logMap, ok := logRaw.(map[string]interface{}); ok {
				log := &pb.RpcLogEntry{
					Address:          getStr(logMap, "address"),
					Data:             getStr(logMap, "data"),
					BlockNumber:      getStr(logMap, "blockNumber"),
					TransactionHash:  getStr(logMap, "transactionHash"),
					BlockHash:        getStr(logMap, "blockHash"),
					TransactionIndex: getStr(logMap, "transactionIndex"),
					LogIndex:         getStr(logMap, "logIndex"),
					Removed:          getBoolVal(logMap, "removed"),
				}
				// Topics
				if topicsRaw, ok := logMap["topics"].([]interface{}); ok {
					for _, t := range topicsRaw {
						if ts, ok := t.(string); ok {
							log.Topics = append(log.Topics, ts)
						}
					}
				}
				receipt.Logs = append(receipt.Logs, log)
			}
		}
	}
	return receipt
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBoolVal(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

package tcp_server

import (
	"encoding/json"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	t_network "github.com/meta-node-blockchain/meta-node/types/network"
	"google.golang.org/protobuf/proto"
)

// HandleRequest xử lý tất cả request từ TCP client
func (srv *RpcTcpServer) HandleRequest(request t_network.Request) error {
	if request == nil || request.Message() == nil {
		return nil
	}

	cmd := request.Message().Command()

	switch cmd {
	// === Bỏ qua ===
	case "InitConnection":
		return nil

	// === Xử lý trực tiếp (không cần gọi chain) ===
	case CmdNetVersion:
		return srv.handleNetVersion(request)

	// === Method cần xử lý đặc biệt (giống HTTP handler) ===
	case "eth_sendRawTransaction":
		return srv.handleSendRawTransaction(request)
	case "eth_call":
		return srv.handleEthCall(request)
	case "eth_estimateGas":
		return srv.handleEstimateGas(request)
	case "rpc_registerBlsKeyWithSignature":
		return srv.handleRegisterBlsKey(request)
	case "eth_deployContract":
		return srv.handleDeployContract(request)
	case "rpc_pushArtifact":
		return srv.handlePushArtifact(request)

	// === Subscription ===
	case "eth_subscribe":
		return srv.handleEthSubscribe(request)
	case "eth_unsubscribe":
		return srv.handleEthUnsubscribe(request)

	// === Receipt (pure protobuf) ===
	case "eth_getTransactionReceipt":
		return srv.handleGetTransactionReceipt(request)

	// === Default: Forward lên chain qua HTTP ===
	default:
		return srv.forwardToChain(request, cmd)
	}
}

// ===================== Helper functions =====================

// httpRespToTcpResp chuyển đổi rpc_client.JSONRPCResponse → pb.RpcResponse
func httpRespToTcpResp(httpResp rpc_client.JSONRPCResponse, msgID string) *pb.RpcResponse {
	resp := &pb.RpcResponse{
		Jsonrpc: "2.0",
		Id:      msgID,
	}
	if httpResp.Error != nil {
		resp.Error = &pb.RpcError{
			Code:    int32(httpResp.Error.Code),
			Message: httpResp.Error.Message,
			Data:    httpResp.Error.Data,
		}
	} else if httpResp.Result != nil {
		resultBytes, err := json.Marshal(httpResp.Result)
		if err != nil {
			resp.Error = &pb.RpcError{
				Code:    -32603,
				Message: "Internal error: failed to marshal result",
			}
		} else {
			resp.Result = resultBytes
		}
	}
	return resp
}

// sendTcpResponse serialize và gửi pb.RpcResponse về client
func (srv *RpcTcpServer) sendTcpResponse(conn t_network.Connection, resp *pb.RpcResponse) error {
	if resp.Error != nil {
		logger.Error("❌ TCP %s error: %s", "response", resp.Error.Message)
	}
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		logger.Error("TCP sendTcpResponse: marshal error: %v", err)
		return err
	}
	return srv.messageSender.SendBytes(conn, CmdRpcResponse, respBytes)
}

// sendRpcResponse helper gửi RpcResponse về client (backward compat)
func (srv *RpcTcpServer) sendRpcResponse(conn t_network.Connection, id string, result []byte, rpcErr *pb.RpcError) error {
	resp := &pb.RpcResponse{
		Jsonrpc: "2.0",
		Result:  result,
		Error:   rpcErr,
		Id:      id,
	}
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		logger.Error("TCP sendRpcResponse: marshal error: %v", err)
		return err
	}
	return srv.messageSender.SendBytes(conn, CmdRpcResponse, respBytes)
}

// parseParamsRaw parse body thành []json.RawMessage
func parseParamsRaw(body []byte) []json.RawMessage {
	var params []json.RawMessage
	if len(body) > 0 {
		_ = json.Unmarshal(body, &params)
	}
	return params
}

// ===================== Handlers =====================

// handleNetVersion - trả về chainId trực tiếp
func (srv *RpcTcpServer) handleNetVersion(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	chainId := srv.AppCtx.ClientRpc.ChainId.String()
	logger.Info("✅ TCP NetVersion from %s -> chainId=%s", conn.RemoteAddrSafe(), chainId)

	resultBytes, _ := json.Marshal(chainId)
	return srv.sendRpcResponse(conn, msgID, resultBytes, nil)
}

// handleSendRawTransaction - build BLS key, convert Ethereum tx → MetaTx
func (srv *RpcTcpServer) handleSendRawTransaction(request t_network.Request) error {
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
			Message: "Invalid params: missing raw transaction hex",
		})
	}

	rawTxHex := params[0]

	httpResp := handlers.ProcessSendRawTransaction(srv.AppCtx, rawTxHex, msgID)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP eth_sendRawTransaction result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// handleEthCall - xử lý giống HTTP handler (interceptor logic + forward to chain)
func (srv *RpcTcpServer) handleEthCall(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: missing call object for eth_call",
		})
	}

	httpResp := handlers.HandleEthCallRaw(srv.AppCtx, params[0], msgID)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP eth_call result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// handleEstimateGas - xử lý giống HTTP handler
func (srv *RpcTcpServer) handleEstimateGas(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params: missing call object for eth_estimateGas",
		})
	}

	logger.Info("🔄 TCP eth_estimateGas from %s", conn.RemoteAddrSafe())
	httpResp := handlers.HandleEstimateGasRaw(srv.AppCtx, params[0], msgID)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP eth_estimateGas result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// handleRegisterBlsKey - xử lý giống HTTP handler
func (srv *RpcTcpServer) handleRegisterBlsKey(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params for rpc_registerBlsKeyWithSignature",
		})
	}

	logger.Info("🔄 TCP rpc_registerBlsKeyWithSignature from %s", conn.RemoteAddrSafe())
	httpResp := handlers.HandleRpcRegisterBlsKeyWithSignatureRaw(srv.AppCtx, params[0], msgID)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP rpc_registerBlsKeyWithSignature result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// handleDeployContract - xử lý giống HTTP handler
func (srv *RpcTcpServer) handleDeployContract(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params for eth_deployContract",
		})
	}

	logger.Info("🔄 TCP eth_deployContract from %s", conn.RemoteAddrSafe())
	req := models.JSONRPCRequestRaw{
		Jsonrpc: "2.0",
		Method:  "eth_deployContract",
		Params:  marshalParams(params),
		Id:      msgID,
	}
	httpResp := handlers.HandleDeployContract(srv.AppCtx, req)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP eth_deployContract result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// handlePushArtifact - xử lý giống HTTP handler
func (srv *RpcTcpServer) handlePushArtifact(request t_network.Request) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	params := parseParamsRaw(request.Message().Body())
	if len(params) == 0 {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    -32602,
			Message: "Invalid params for rpc_pushArtifact",
		})
	}

	logger.Info("🔄 TCP rpc_pushArtifact from %s", conn.RemoteAddrSafe())
	req := models.JSONRPCRequestRaw{
		Jsonrpc: "2.0",
		Method:  "rpc_pushArtifact",
		Params:  marshalParams(params),
		Id:      msgID,
	}
	httpResp := handlers.HandlePushArtifact(srv.AppCtx, req)
	resp := httpRespToTcpResp(httpResp, msgID)

	if resp.Error == nil {
		logger.Info("✅ TCP rpc_pushArtifact result: %s", string(resp.Result))
	}
	return srv.sendTcpResponse(conn, resp)
}

// marshalParams convert []json.RawMessage thành json.RawMessage (JSON array)
func marshalParams(params []json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(params)
	return json.RawMessage(b)
}

// forwardToChain forward request lên chain qua HTTP JSON-RPC
func (srv *RpcTcpServer) forwardToChain(request t_network.Request, cmd string) error {
	conn := request.Connection()
	msgID := request.Message().ID()

	logger.Info("🔄 TCP %s -> forwarding to chain via HTTP (from %s)", cmd, conn.RemoteAddrSafe())

	params := make([]interface{}, 0)
	body := request.Message().Body()
	if len(body) > 0 {
		_ = json.Unmarshal(body, &params)
	}

	httpReq := &rpc_client.JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  cmd,
		Params:  params,
		Id:      msgID,
	}
	httpResp := srv.AppCtx.ClientRpc.SendHTTPRequest(httpReq)
	resp := httpRespToTcpResp(*httpResp, msgID)

	logger.Info("✅ TCP %s result: %s", cmd, string(resp.Result))
	return srv.sendTcpResponse(conn, resp)
}

// handleGetTransactionReceipt - trả về receipt dạng protobuf RpcReceipt (pure proto)
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
	logger.Info("🔄 TCP eth_getTransactionReceipt from %s (txHash=%s)", conn.RemoteAddrSafe(), txHash)

	// Gọi chain lấy receipt JSON
	httpReq := &rpc_client.JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  "eth_getTransactionReceipt",
		Params:  []interface{}{txHash},
		Id:      msgID,
	}
	httpResp := srv.AppCtx.ClientRpc.SendHTTPRequest(httpReq)

	if httpResp.Error != nil {
		return srv.sendRpcResponse(conn, msgID, nil, &pb.RpcError{
			Code:    int32(httpResp.Error.Code),
			Message: httpResp.Error.Message,
		})
	}

	if httpResp.Result == nil {
		// Receipt chưa có (pending)
		return srv.sendRpcResponse(conn, msgID, nil, nil)
	}

	// Parse JSON receipt
	resultBytes, _ := json.Marshal(httpResp.Result)
	var receiptJSON map[string]interface{}
	if err := json.Unmarshal(resultBytes, &receiptJSON); err != nil {
		// Fallback: gửi raw JSON như cũ
		return srv.sendRpcResponse(conn, msgID, resultBytes, nil)
	}

	// Convert JSON → pb.RpcReceipt
	rpcReceipt := jsonToRpcReceipt(receiptJSON)
	receiptProtoBytes, err := proto.Marshal(rpcReceipt)
	if err != nil {
		logger.Error("❌ handleGetTransactionReceipt: proto marshal error: %v", err)
		return srv.sendRpcResponse(conn, msgID, resultBytes, nil)
	}

	logger.Info("✅ TCP eth_getTransactionReceipt: %s (%d bytes proto)", txHash, len(receiptProtoBytes))

	// Gửi qua command riêng CmdRpcReceipt (pure protobuf)
	resp := &pb.RpcResponse{
		Jsonrpc: "2.0",
		Id:      msgID,
		Result:  receiptProtoBytes,
	}
	return srv.sendTcpResponse(conn, resp)
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

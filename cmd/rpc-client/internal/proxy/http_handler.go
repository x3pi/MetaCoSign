package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/constants"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/tidwall/gjson"
)

func (p *RpcReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, releaseBody, err := ReadBodyWithLimit(r)
	if err != nil {
		releaseBody()
		logger.Error("Failed to read request body: %v", err)
		if errors.Is(err, constants.ErrRequestBodyTooLarge) {
			http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		}
		return
	}
	defer releaseBody()

	// Log request body preview để debug
	if len(body) > 0 {
		bodyPreview := string(body)
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}
		logger.Info("📥 [http_handler] Request body preview: %s", bodyPreview)
	}

	methodResult := gjson.GetBytes(body, "method")
	if !methodResult.Exists() {
		logger.Info("📥 [http_handler] No method in request, forwarding to upstream")
		r.Body = io.NopCloser(bytes.NewReader(body))
		p.ReverseProxy.ServeHTTP(w, r)
		return
	}

	method := methodResult.String()
	id := utils.ExtractRequestID(body)
	logger.Info("📥 [http_handler] Parsed request: method=%s, id=%v (type=%T)", method, id, id)

	switch method {
	case "eth_sendRawTransaction":
		rawTx := gjson.GetBytes(body, "params.0")
		if !rawTx.Exists() {
			logger.Warn("⚠️ [http_handler] eth_sendRawTransaction missing params: id=%v", id)
			resp := utils.MakeInvalidParamError(id, "Invalid params for sendRawTransaction")
			utils.WriteJSON(w, resp)
			return
		}
		rawTxStr := rawTx.String()
		// Log một phần của rawTx để debug (không log toàn bộ vì quá dài)
		if len(rawTxStr) > 100 {
			logger.Info("🔵 [http_handler] Received eth_sendRawTransaction request: id=%v, rawTx preview=%s... (len=%d)", id, rawTxStr[:100], len(rawTxStr))
		} else {
			logger.Info("🔵 [http_handler] Received eth_sendRawTransaction request: id=%v, rawTx=%s (len=%d)", id, rawTxStr, len(rawTxStr))
		}

		// Log trước khi xử lý
		logger.Info("🔵 [http_handler] Processing eth_sendRawTransaction: id=%v", id)

		resp := handlers.ProcessSendRawTransaction(
			p.AppCtx,
			rawTxStr,
			id,
		)

		logger.Info("🔵 [http_handler] ProcessSendRawTransaction response: id=%v, hasResult=%v, hasError=%v, result=%v (type=%T), error=%v",
			id, resp.Result != nil, resp.Error != nil, resp.Result, resp.Result, resp.Error)

		// Log JSON response trước khi gửi
		if resp.Result != nil {
			if resultStr, ok := resp.Result.(string); ok {
				logger.Info("🔵 [http_handler] Sending response with result: id=%v, result=%s (len=%d)", id, resultStr, len(resultStr))
			}
		}

		utils.WriteJSON(w, resp)
		logger.Info("🔵 [http_handler] Response sent for eth_sendRawTransaction: id=%v", id)
		return

	case "net_version":
		resp := rpc_client.JSONRPCResponse{
			Jsonrpc: "2.0",
			Result:  p.AppCtx.ClientRpc.ChainId.String(),
			Id:      id,
		}
		utils.WriteJSON(w, resp)
		return

	case "eth_estimateGas":
		logger.Info("eth_estimateGas body: %v", string(body))
		callParam := gjson.GetBytes(body, "params.0")
		if !callParam.Exists() {
			logger.Info("Cannot unmarshal params for eth_estimateGas")
			resp := utils.MakeInvalidParamError(id, "Cannot unmarshal params for eth_estimateGas")
			utils.WriteJSON(w, resp)
			return
		}
		resp := handlers.HandleEstimateGasRaw(p.AppCtx, json.RawMessage(callParam.Raw), id)
		utils.WriteJSON(w, resp)
		return

	case "eth_call":
		callParam := gjson.GetBytes(body, "params.0")
		if !callParam.Exists() {
			resp := utils.MakeInvalidParamError(id, "Cannot unmarshal params for eth_call")
			utils.WriteJSON(w, resp)
			return
		}
		resp := handlers.HandleEthCallRaw(p.AppCtx, json.RawMessage(callParam.Raw), id)
		utils.WriteJSON(w, resp)
		return

	case "rpc_registerBlsKeyWithSignature":
		registerParam := gjson.GetBytes(body, "params.0")
		if !registerParam.Exists() {
			resp := utils.MakeInvalidParamError(id, "Cannot unmarshal params for rpc_registerBlsKeyWithSignature")
			utils.WriteJSON(w, resp)
			return
		}
		resp := handlers.HandleRpcRegisterBlsKeyWithSignatureRaw(
			p.AppCtx,
			json.RawMessage(registerParam.Raw),
			id,
		)
		utils.WriteJSON(w, resp)
		return

	case "eth_deployContract":
		var req models.JSONRPCRequestRaw
		if err := json.Unmarshal(body, &req); err != nil {
			resp := utils.MakeInvalidParamError(id, "Invalid JSON-RPC request")
			utils.WriteJSON(w, resp)
			return
		}
		resp := handlers.HandleDeployContract(p.AppCtx, req)
		utils.WriteJSON(w, resp)
		return

	case "eth_getTransactionByHash":
		txHash := gjson.GetBytes(body, "params.0")
		if !txHash.Exists() {
			logger.Warn("⚠️ [http_handler] eth_getTransactionByHash missing params: id=%v", id)
			resp := utils.MakeInvalidParamError(id, "Invalid params for eth_getTransactionByHash")
			utils.WriteJSON(w, resp)
			return
		}
		txHashStr := txHash.String()
		logger.Info("🔵 [http_handler] Received eth_getTransactionByHash request: id=%v, txHash=%s", id, txHashStr)
		// Forward request to upstream RPC server
		r.Body = io.NopCloser(bytes.NewReader(body))
		logger.Info("🔵 [http_handler] Forwarding eth_getTransactionByHash to upstream: id=%v, txHash=%s", id, txHashStr)
		p.ReverseProxy.ServeHTTP(w, r)
		logger.Info("🔵 [http_handler] Forwarded eth_getTransactionByHash response: id=%v, txHash=%s", id, txHashStr)
		return

	case "eth_getTransactionReceipt":
		txHash := gjson.GetBytes(body, "params.0")
		if !txHash.Exists() {
			logger.Warn("⚠️ [http_handler] eth_getTransactionReceipt missing params: id=%v", id)
			resp := utils.MakeInvalidParamError(id, "Invalid params for eth_getTransactionReceipt")
			utils.WriteJSON(w, resp)
			return
		}
		txHashStr := txHash.String()
		logger.Info("🔵 [http_handler] Received eth_getTransactionReceipt request: id=%v, txHash=%s", id, txHashStr)
		// Forward request to upstream RPC server
		r.Body = io.NopCloser(bytes.NewReader(body))
		logger.Info("🔵 [http_handler] Forwarding eth_getTransactionReceipt to upstream: id=%v, txHash=%s", id, txHashStr)
		p.ReverseProxy.ServeHTTP(w, r)
		logger.Info("🔵 [http_handler] Forwarded eth_getTransactionReceipt response: id=%v, txHash=%s", id, txHashStr)
		return

	case "eth_getTransactionCount":
		address := gjson.GetBytes(body, "params.0")
		blockTag := gjson.GetBytes(body, "params.1")
		if !address.Exists() {
			logger.Warn("⚠️ [http_handler] eth_getTransactionCount missing params: id=%v", id)
			resp := utils.MakeInvalidParamError(id, "Invalid params for eth_getTransactionCount")
			utils.WriteJSON(w, resp)
			return
		}
		addressStr := address.String()
		blockTagStr := "latest"
		if blockTag.Exists() {
			blockTagStr = blockTag.String()
		}
		logger.Info("🔵 [http_handler] Received eth_getTransactionCount request: id=%v, address=%s, blockTag=%s", id, addressStr, blockTagStr)

		// Gọi upstream RPC trực tiếp để có thể kiểm tra và sửa response
		params := []interface{}{addressStr}
		if blockTag.Exists() {
			params = append(params, blockTagStr)
		}
		request := &rpc_client.JSONRPCRequest{
			Jsonrpc: "2.0",
			Method:  "eth_getTransactionCount",
			Params:  params,
			Id:      id,
		}

		response := p.AppCtx.ClientRpc.SendHTTPRequest(request)

		// Kiểm tra và sửa response nếu có null
		if response.Error != nil {
			logger.Warn("⚠️ [http_handler] eth_getTransactionCount error: id=%v, error=%v", id, response.Error)
		} else if response.Result != nil {
			// Đảm bảo Result là hex string hợp lệ (không null)
			resultStr, ok := response.Result.(string)
			if !ok || resultStr == "" || resultStr == "null" {
				logger.Warn("⚠️ [http_handler] eth_getTransactionCount result is null or invalid: id=%v, result=%v (type=%T), setting to 0x0", id, response.Result, response.Result)
				response.Result = "0x0"
			} else {
				logger.Info("🔵 [http_handler] eth_getTransactionCount response: id=%v, result=%s", id, resultStr)
			}
		} else {
			// Nếu không có result và không có error, set default
			logger.Warn("⚠️ [http_handler] eth_getTransactionCount no result and no error: id=%v, setting to 0x0", id)
			// response.Result = "0x0"
		}

		utils.WriteJSON(w, *response)
		logger.Info("🔵 [http_handler] Sent eth_getTransactionCount response: id=%v, address=%s", id, addressStr)
		return

	default:
		r.Body = io.NopCloser(bytes.NewReader(body))
		p.ReverseProxy.ServeHTTP(w, r)
		return
	}
}

func (p *RpcReverseProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	logger.Error("ReverseProxy error for %s %s: %v", r.Method, r.URL, err)
	http.Error(w, "Upstream server error", http.StatusBadGateway)
}

func (p *RpcReverseProxy) readonlyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	logger.Error("Readonly ReverseProxy error for %s %s: %v", r.Method, r.URL, err)
	http.Error(w, "Readonly upstream server error", http.StatusBadGateway)
}

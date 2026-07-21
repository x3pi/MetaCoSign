package proxy

import (
	"encoding/json"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/tidwall/gjson"
)

// JSONRPCRequestRaw represents a raw JSON-RPC request
type JSONRPCRequestRaw struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Id      interface{}     `json:"id"`
}

// RouteWebSocketMessage routes incoming WebSocket JSON-RPC requests
func (p *RpcReverseProxy) RouteWebSocketMessage(req models.JSONRPCRequestRaw) (*rpc_client.JSONRPCResponse, bool) {
	// Panic recovery cho tất cả RPC operations
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic in RouteWebSocketMessage for method %s: %v", req.Method, r)
			// Panic đã được recover, không crash server
		}
	}()

	switch req.Method {

	case "eth_sendRawTransaction":
		resp := p.handleWithPanicRecovery(func() interface{} {
			return handlers.HandleSendRawTransaction(p.AppCtx, req)
		}, req.Method).(rpc_client.JSONRPCResponse)
		return &resp, true

	case "net_version":
		resp := rpc_client.JSONRPCResponse{
			Jsonrpc: "2.0",
			Result:  p.AppCtx.ClientRpc.ChainId.String(),
			Id:      req.Id,
		}
		return &resp, true

	case "eth_estimateGas":
		resp := p.handleWithPanicRecovery(func() interface{} {
			return handlers.HandleEstimateGas(p.AppCtx, req)
		}, req.Method).(rpc_client.JSONRPCResponse)
		return &resp, true

	case "eth_call":
		resp := p.handleWithPanicRecovery(func() interface{} {
			return handlers.HandleEthCall(p.AppCtx, req)
		}, req.Method).(rpc_client.JSONRPCResponse)
		return &resp, true

	case "rpc_registerBlsKeyWithSignature":
		resp := p.handleWithPanicRecovery(func() interface{} {
			return handlers.HandleRpcRegisterBlsKeyWithSignature(p.AppCtx, req)
		}, req.Method).(rpc_client.JSONRPCResponse)
		return &resp, true

	case "mtn_sendRawTransactionWithDeviceKey":
		resp := p.handleWithPanicRecovery(func() interface{} {
			rawTxResult := gjson.GetBytes(req.Params, "0")
			if !rawTxResult.Exists() {
				return utils.MakeInvalidParamError(req.Id, "Invalid params for sendRawTransactionWithDeviceKey")
			}
			isPrevent, hash, err := handlers.ProcessSendRawTransactionWithDeviceKey(p.AppCtx, rawTxResult.String(), req.Id)
			if isPrevent {
				if err != nil {
					return utils.MakeInternalError(req.Id, err.Error())
				}
				return rpc_client.JSONRPCResponse{Jsonrpc: "2.0", Id: req.Id, Result: hash}
			}
			// Return nil to indicate it should be forwarded
			return nil
		}, req.Method)

		if resp == nil {
			return nil, false
		}
		r := resp.(rpc_client.JSONRPCResponse)
		return &r, true

	default:
		// Not handled by proxy - forward to upstream
		return nil, false
	}
}

// handleWithPanicRecovery wraps function calls with panic recovery
func (p *RpcReverseProxy) handleWithPanicRecovery(fn func() interface{}, method string) interface{} {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic recovered in %s handler: %v", method, r)
			// Return error response instead of crashing
		}
	}()
	return fn()
}

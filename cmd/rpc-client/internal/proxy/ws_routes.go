package proxy

import (
	"encoding/json"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
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
	switch req.Method {

	case "eth_sendRawTransaction":
		resp := handlers.HandleSendRawTransaction(
			p.AppCtx,
			req,
		)
		return &resp, true

	case "net_version":
		resp := rpc_client.JSONRPCResponse{
			Jsonrpc: "2.0",
			Result:  p.AppCtx.ClientRpc.ChainId.String(),
			Id:      req.Id,
		}
		return &resp, true

	case "eth_estimateGas":
		resp := handlers.HandleEstimateGas(p.AppCtx, req)
		return &resp, true

	case "eth_call":
		resp := handlers.HandleEthCall(p.AppCtx, req)
		return &resp, true

	case "rpc_registerBlsKeyWithSignature":
		resp := handlers.HandleRpcRegisterBlsKeyWithSignature(p.AppCtx, req)
		return &resp, true

	default:
		// Not handled by proxy - forward to upstream
		return nil, false
	}
}

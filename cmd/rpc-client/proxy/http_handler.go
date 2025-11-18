package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/constants"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/handlers"
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

	methodResult := gjson.GetBytes(body, "method")
	if !methodResult.Exists() {
		r.Body = io.NopCloser(bytes.NewReader(body))
		p.ReverseProxy.ServeHTTP(w, r)
		return
	}

	method := methodResult.String()
	id := utils.ExtractRequestID(body)

	switch method {
	case "eth_sendRawTransaction":
		rawTx := gjson.GetBytes(body, "params.0")
		logger.Info(" eth_sendRawTransaction rawTx: %s", rawTx.String())
		if !rawTx.Exists() {
			resp := utils.MakeInvalidParamError(id, "Invalid params for sendRawTransaction")
			utils.WriteJSON(w, resp)
			return
		}
		resp := handlers.ProcessSendRawTransaction(
			p.AppCtx,
			rawTx.String(),
			id,
		)
		utils.WriteJSON(w, resp)
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
		callParam := gjson.GetBytes(body, "params.0")
		if !callParam.Exists() {
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

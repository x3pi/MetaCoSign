package handlers

import (
	"encoding/json"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
)

func HandleEthCall(appCtx *app.Context, req models.JSONRPCRequestRaw) rpc_client.JSONRPCResponse {
	var callParamsList []json.RawMessage
	if err := json.Unmarshal(req.Params, &callParamsList); err != nil || len(callParamsList) == 0 {
		return utils.MakeInvalidParamError(req.Id, "Cannot unmarshal params for eth_call")
	}
	return processEthCallParams(appCtx, req.Id, callParamsList[0])
}

func HandleEthCallRaw(appCtx *app.Context, callParam json.RawMessage, id interface{}) rpc_client.JSONRPCResponse {
	return processEthCallParams(appCtx, id, callParam)
}

func processEthCallParams(appCtx *app.Context, id interface{}, callObjectRaw json.RawMessage) rpc_client.JSONRPCResponse {
	fromAddress, toAddress, hasTo, payload, err := utils.DecodeCallObject(callObjectRaw)
	if err != nil {
		return utils.MakeInvalidParamError(id, "Invalid eth_call parameter")
	}

	var bTx []byte
	var buildErr error

	// ✅ Sử dụng appCtx.ClientRpc
	if !hasTo {
		bTx, buildErr = appCtx.ClientRpc.BuildDeployTransaction(payload, fromAddress)
	} else {
		bTx, buildErr = appCtx.ClientRpc.BuildCallTransaction(payload, toAddress, fromAddress)
	}

	if buildErr != nil {
		return utils.MakeInternalError(id, "Failed to build transaction for eth_call")
	}

	rs := appCtx.ClientRpc.SendCallTransaction(bTx)
	rs.Id = id
	return rs
}

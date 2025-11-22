package handlers

import (
	"context"
	"encoding/json"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/account_handler"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	utilsPkg "github.com/meta-node-blockchain/meta-node/pkg/utils"
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
	// logger.Info("Processing eth_call with params: %s", string(callObjectRaw))
	fromAddress, toAddress, hasTo, payload, err := utils.DecodeCallObject(callObjectRaw)
	if err != nil {
		return utils.MakeInvalidParamError(id, "Invalid eth_call parameter")
	}
	if hasTo && toAddress == utilsPkg.GetAddressSelector(common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		logger.Info("Handling eth_call for Account contract at address %s", toAddress.Hex())
		accountHandler, err := account_handler.GetAccountHandler(appCtx)
		if err != nil {
			logger.Error("Failed to get account handler: %v", err)
			return utils.MakeInternalError(id, "Failed to get account handler: "+err.Error())
		}

		// Handle eth_call cho account operations
		result, err := accountHandler.HandleEthCall(context.Background(), payload)
		if err != nil {
			// logger.Error("Account handler eth_call error: %v", err)
			return utils.MakeInternalError(id, "Account handler error: "+err.Error())
		}

		// Encode result thành JSON hex string
		jsonBytes, err := json.Marshal(result)
		if err != nil {
			return utils.MakeInternalError(id, "Failed to encode result: "+err.Error())
		}

		// Convert JSON to hex string (0x...)
		hexResult := "0x" + ethCommon.Bytes2Hex(jsonBytes)
		return rpc_client.JSONRPCResponse{
			Jsonrpc: "2.0",
			Result:  hexResult,
			Id:      id,
		}
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

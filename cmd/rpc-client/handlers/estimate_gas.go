package handlers

import (
	"encoding/json"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
)

func HandleEstimateGas(appCtx *app.Context, req models.JSONRPCRequestRaw) rpc_client.JSONRPCResponse {
	logger.Info("HandleEstimateGas: %v", req)
	var callParamsList []json.RawMessage
	if err := json.Unmarshal(req.Params, &callParamsList); err != nil || len(callParamsList) == 0 {
		return utils.MakeInvalidParamError(req.Id, "Cannot unmarshal params for eth_estimateGas")
	}
	return HandleEstimateGasRaw(appCtx, callParamsList[0], req.Id)
}

func HandleEstimateGasRaw(appCtx *app.Context, callParam json.RawMessage, id interface{}) rpc_client.JSONRPCResponse {
	decoded, err := utils.DecodeCallObject(callParam)
	if err != nil {
		logger.Error("Failed to decode call object for eth_estimateGas: %v, raw: %s", err, string(callParam))
		return utils.MakeInvalidParamError(id, "Invalid eth_estimateGas parameter: "+err.Error())
	}

	// Intercept contracts in ContractsInterceptor (e.g. AccountManager, RobotHandler)
	// Return a default gas estimate (0x30d40 = 200,000 gas) to avoid client estimation errors
	if decoded.HasTo && appCtx.Cfg != nil {
		for _, interceptorAddr := range appCtx.Cfg.ContractsInterceptor {
			if decoded.ToAddress == ethCommon.HexToAddress(interceptorAddr) {
				return rpc_client.JSONRPCResponse{
					Jsonrpc: "2.0",
					Result:  "0x30d40", // 200,000 gas in hex
					Id:      id,
				}
			}
		}
	}

	var bTx []byte
	var buildErr error

	if !decoded.HasTo {
		bTx, buildErr = appCtx.ClientRpc.BuildDeployTransaction(decoded)
	} else {
		bTx, buildErr = appCtx.ClientRpc.BuildCallTransaction(decoded)
	}
	if buildErr != nil {
		logger.Error("Failed to build transaction for estimateGas: %v", buildErr)
		return utils.MakeInternalError(id, "Failed to build transaction for estimateGas: "+buildErr.Error())
	}
	rs := appCtx.ClientRpc.SendEstimateGas(bTx)
	rs.Id = id
	return rs
}

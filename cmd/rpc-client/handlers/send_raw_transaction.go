package handlers

import (
	"context"
	"encoding/json"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/account_handler"
	"github.com/meta-node-blockchain/meta-node/pkg/bls"
	"github.com/meta-node-blockchain/meta-node/pkg/file_handler"
	robothandler "github.com/meta-node-blockchain/meta-node/pkg/robot_handler"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	mt_types "github.com/meta-node-blockchain/meta-node/types"
	"github.com/tidwall/gjson"
)

func HandleSendRawTransaction(appCtx *app.Context, req models.JSONRPCRequestRaw) rpc_client.JSONRPCResponse {
	rawTxResult := gjson.GetBytes(req.Params, "0")
	if !rawTxResult.Exists() {
		return utils.MakeInvalidParamError(req.Id, "Invalid params for sendRawTransaction")
	}
	data := ProcessSendRawTransaction(appCtx, rawTxResult.String(), req.Id)
	return data
}

func ProcessSendRawTransaction(appCtx *app.Context, rawTransactionHex string, id interface{}) rpc_client.JSONRPCResponse {
	decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(rawTransactionHex)
	if err != nil {
		return utils.MakeInvalidParamError(id, "Invalid raw transaction hex data")
	}
	decodedReleased := false
	releaseDecodedOnce := func() {
		if decodedReleased {
			return
		}
		decodedReleased = true
		if releaseDecoded != nil {
			releaseDecoded()
		}
	}
	ethTx := new(types.Transaction)
	if err := ethTx.UnmarshalBinary(decodedTxBytes); err != nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Failed to unmarshal Ethereum transaction")
	}
	signer := types.LatestSignerForChainID(appCtx.ClientRpc.ChainId)
	fromAddress, err := types.Sender(signer, ethTx)
	if err != nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Failed to derive sender from transaction")
	}
	if appCtx.PKS == nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Private key store not available.")
	}
	exists, err := appCtx.PKS.HasPrivateKey(fromAddress)
	if err != nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Error checking private key store")
	}
	var (
		bTx       []byte
		releaseTx func()
		buildErr  error

		tx mt_types.Transaction
	)
	if !exists {
		bTx, tx, releaseTx, buildErr = appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTx(ethTx, appCtx.TcpCfg, appCtx.Cfg, appCtx.LdbContractFreeGas, 0) // nonce = 0: dùng nonce từ account state
	} else {
		senderPkString, _ := appCtx.PKS.GetPrivateKey(fromAddress)
		keyPair := bls.NewKeyPair(ethCommon.FromHex(senderPkString))
		bTx, tx, releaseTx, buildErr = appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTxAndBlsPrivateKey(ethTx, appCtx.TcpCfg, appCtx.Cfg, appCtx.LdbContractFreeGas, keyPair.PrivateKey(), 0) // nonce = 0: dùng nonce từ account state
	}
	if buildErr != nil {
		releaseDecodedOnce()
		if releaseTx != nil {
			releaseTx()
		}
		return utils.MakeInternalError(id, "Failed to build transaction: "+buildErr.Error())
	}

	if tx != nil {
		if tx.ToAddress() == ethCommon.HexToAddress(appCtx.Cfg.ContractsInterceptor[0]) {
			accountHandler, err := account_handler.GetAccountHandler(appCtx)
			if err != nil {
				return utils.MakeInternalError(id, "Failed to get account: "+err.Error())
			}
			handled, result, err := accountHandler.HandleAccountTransaction(
				context.Background(),
				tx,
				rawTransactionHex,
			)
			if handled {
				releaseDecodedOnce()
				if releaseTx != nil {
					releaseTx()
				}
				if err != nil {
					logger.Error("Account handler transaction error: %v", err)
					return rpc_client.JSONRPCResponse{
						Jsonrpc: "2.0",
						Error: &rpc_client.JSONRPCError{
							Code:    -1,
							Message: err.Error(),
						},
					}
				}
				if result != nil {
					return rpc_client.JSONRPCResponse{
						Jsonrpc: "2.0",
						Result:  result,
						Id:      id,
					}
				}
				return rpc_client.JSONRPCResponse{
					Jsonrpc: "2.0",
					Result:  tx.Hash().Hex(),
					Id:      id,
				}
			} else if !handled && err == nil {
				rs := appCtx.ClientRpc.SendRawTransactionBinary(bTx, releaseTx, decodedTxBytes, releaseDecodedOnce, nil)
				releaseDecodedOnce()
				rs.Id = id
				return rs
			}
			return utils.MakeInternalError(id, "method notfound in abi account")

		}
		if tx.ToAddress() == ethCommon.HexToAddress(appCtx.Cfg.ContractsInterceptor[1]) {
			txHash := tx.Hash().Hex()
			logger.Info("🔵 [sendRawTransaction] Robot contract detected - txHash=%s, from=%s, to=%s, nonce=%d, id=%v",
				txHash, tx.FromAddress().Hex(), tx.ToAddress().Hex(), tx.GetNonce(), id)
			robotHandler, err := robothandler.GetRobotHandler(appCtx)
			if err != nil {
				logger.Error("❌ [sendRawTransaction] Failed to get robot handler: %v, id=%v", err, id)
				return utils.MakeInternalError(id, "Failed to get robot handler: "+err.Error())
			}
			logger.Info("🔵 [sendRawTransaction] Calling HandleRobotTransaction: txHash=%s, id=%v", txHash, id)
			handled, result, err := robotHandler.HandleRobotTransaction(
				context.Background(),
				tx,
				rawTransactionHex,
			)
			logger.Info("🔵 [sendRawTransaction] HandleRobotTransaction response: txHash=%s, id=%v, handled=%v, result=%v (type=%T), err=%v",
				txHash, id, handled, result, result, err)
			if handled {
				releaseDecodedOnce()
				if releaseTx != nil {
					releaseTx()
				}
				if err != nil {
					logger.Error("❌ [sendRawTransaction] Robot handler transaction error: %v", err)
					return rpc_client.JSONRPCResponse{
						Jsonrpc: "2.0",
						Error: &rpc_client.JSONRPCError{
							Code:    -1,
							Message: err.Error(),
						},
						Id: id,
					}
				}
				// Luôn trả về transaction hash (string) để viem có thể parse được
				// ĐẢM BẢO KHÔNG BAO GIỜ NULL
				txHash := tx.Hash().Hex()
				var finalResult string
				if result != nil {
					// Nếu result đã là string (txHash), dùng luôn
					if txHashStr, ok := result.(string); ok && txHashStr != "" {
						finalResult = txHashStr
						logger.Info("✅ [sendRawTransaction] Using txHash from result: %s", finalResult)
					} else {
						// Nếu là object hoặc empty string, lấy txHash từ transaction
						finalResult = txHash
						logger.Warn("⚠️ [sendRawTransaction] Result is not valid string (type=%T, value=%v), using tx.Hash(): %s",
							result, result, finalResult)
					}
				} else {
					finalResult = txHash
					logger.Warn("⚠️ [sendRawTransaction] Result is nil, using tx.Hash(): %s", finalResult)
				}

				// Đảm bảo finalResult không bao giờ empty
				if finalResult == "" {
					finalResult = txHash
					logger.Error("❌ [sendRawTransaction] finalResult is empty, using tx.Hash(): %s", finalResult)
				}

				logger.Info("✅ [sendRawTransaction] Sending response: txHash=%s, id=%v (type=%T), jsonrpc=%s",
					finalResult, id, finalResult, "2.0")
				response := rpc_client.JSONRPCResponse{
					Jsonrpc: "2.0",
					Result:  finalResult,
					Id:      id,
				}
				// Log JSON response để debug
				responseJSON, _ := json.Marshal(response)
				logger.Info("✅ [sendRawTransaction] Response JSON: %s", string(responseJSON))
				logger.Info("✅ [sendRawTransaction] Response created: Jsonrpc=%s, Result=%s, Id=%v",
					response.Jsonrpc, response.Result, response.Id)
				return response
			} else if !handled && err == nil {
				rs := appCtx.ClientRpc.SendRawTransactionBinary(bTx, releaseTx, decodedTxBytes, releaseDecodedOnce, nil)
				releaseDecodedOnce()
				rs.Id = id
				return rs
			}
			return utils.MakeInternalError(id, "method notfound in abi robot")
		}
		fileAbi, _ := file_handler.GetFileAbi()
		name, _ := fileAbi.ParseMethodName(tx)
		if !(tx.ToAddress() == file_handler.PredictContractAddress(ethCommon.HexToAddress(appCtx.ClientTcp.GetClientContext().Config.OwnerFileStorageAddress)) && name == "uploadChunk") {
			rs := appCtx.ClientRpc.SendRawTransactionBinary(bTx, releaseTx, decodedTxBytes, releaseDecodedOnce, nil)
			releaseDecodedOnce()
			rs.Id = id
			return rs
		} else {
			fileHandler, err := file_handler.GetFileHandlerTCP(appCtx.ClientTcp, appCtx.TcpCfg)
			if err != nil {
				return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
			}
			isPrevent, err := fileHandler.HandleFileTransactionNoReceipt(context.Background(), tx)
			if err != nil {
				return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
			}
			if isPrevent {
				releaseDecodedOnce()
				releaseTx()
				return rpc_client.JSONRPCResponse{
					Jsonrpc: "2.0",
					Result:  tx.Hash().Hex(),
					Id:      id,
				}
			}
			return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
		}

	} else {
		return utils.MakeInternalError(id, "null transaction: "+err.Error())
	}
}

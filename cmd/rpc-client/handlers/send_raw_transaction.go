package handlers

import (
	"context"
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/models"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/account_handler"
	"github.com/meta-node-blockchain/meta-node/pkg/file_handler"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/meta-node-blockchain/meta-node/pkg/transaction"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/proto"
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
		return utils.MakeInternalError(id, "Failed to derive sender from transaction "+err.Error())
	}
	as, err := appCtx.ClientRpc.GetAccountState(fromAddress, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber))
	if err != nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Failed to get account state: "+err.Error())
	}

	topUpFunc := func(toAddress ethCommon.Address) error {
		ah, err := account_handler.GetAccountHandler(appCtx)
		if err != nil {
			return fmt.Errorf("get account handler error: %w", err)
		}

		result := ah.SendHelpPayTransfer(toAddress, appCtx.Cfg.ExtraAmount)
		return result.Err
	}

	if !appCtx.Cfg.DisableFreeGas && ethTx.To() != nil && as.Balance().Cmp(appCtx.Cfg.GetFreeGasMinBalance()) < 0 && as.Nonce() != 0 {
		if topUpFunc != nil {
			// Đưa vào hàng chờ owner để tránh nonce conflict
			if err := topUpFunc(fromAddress); err != nil {
				releaseDecodedOnce()
				return utils.MakeInternalError(id, fmt.Sprintf("topUpFunc failed: %v", err))
			}
		}
	}

	tx, err := transaction.NewTransactionFromEth(ethTx)
	if err != nil {
		releaseDecodedOnce()
		return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
	}

	if tx != nil {
		if tx.ToAddress() == ethCommon.HexToAddress(appCtx.Cfg.ContractsInterceptor[0]) {
			accountHandler, err := account_handler.GetAccountHandler(appCtx)
			if err != nil {
				releaseDecodedOnce()
				return utils.MakeInternalError(id, "Failed to get account: "+err.Error())
			}
			handled, result, err := accountHandler.HandleAccountTransaction(
				context.Background(),
				tx,
				rawTransactionHex,
			)
			if handled {
				releaseDecodedOnce()
				if err != nil {
					logger.Error("Account handler transaction error: %v", err)
					return utils.MakeInternalError(id, "Account handler transaction error: "+err.Error())
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
				rs := *appCtx.ClientRpc.SendRawEthTransaction(rawTransactionHex, id)
				releaseDecodedOnce()
				return rs
			}
			releaseDecodedOnce()
			return utils.MakeInternalError(id, "method notfound in abi account")

		}

		fileAbi, _ := file_handler.GetFileAbi()
		name, _ := fileAbi.ParseMethodName(tx)

		if !(tx.ToAddress() == file_handler.PredictContractAddress(ethCommon.HexToAddress(appCtx.ClientTcp.GetClientContext().Config.OwnerFileStorageAddress)) && name == "uploadChunk") {
			rs := *appCtx.ClientRpc.SendRawEthTransaction(rawTransactionHex, id)
			releaseDecodedOnce()
			if rs.Error != nil && tx.ToAddress() != (ethCommon.Address{}) && appCtx.ErrorDecoder != nil {
				appCtx.ErrorDecoder.DecodeError(
					context.Background(),
					&rs,
					tx.ToAddress().Hex(),
					0,
				)
			}
			if rs.Error != nil {
				logger.Info("✅✅✅ rs.Error.Message : %s Data %s Code %d", rs.Error.Message, rs.Error.Data, rs.Error.Code)
			}
			return rs
		} else {
			fileHandler, err := file_handler.GetFileHandlerTCP(appCtx.ClientTcp, appCtx.TcpCfg)
			if err != nil {
				releaseDecodedOnce()
				return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
			}
			isPrevent, err := fileHandler.HandleFileTransactionNoReceipt(context.Background(), tx)
			if err != nil {
				releaseDecodedOnce()
				return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
			}
			if isPrevent {
				releaseDecodedOnce()
				return rpc_client.JSONRPCResponse{
					Jsonrpc: "2.0",
					Result:  tx.Hash().Hex(),
					Id:      id,
				}
			}
			releaseDecodedOnce()
			return utils.MakeInternalError(id, "Failed to build transaction: "+err.Error())
		}

	} else {
		errMsg := "null transaction"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		return utils.MakeInternalError(id, errMsg)
	}
}

func ProcessSendRawTransactionWithDeviceKey(appCtx *app.Context, rawTransactionHex string, id interface{}) (bool, string, error) {
	decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(rawTransactionHex)
	if err != nil {
		return false, "", fmt.Errorf("Invalid raw transaction hex data")
	}
	defer func() {
		if releaseDecoded != nil {
			releaseDecoded()
		}
	}()

	txD := &pb.TransactionWithDeviceKey{}
	err = proto.Unmarshal(decodedTxBytes, txD)
	if err != nil {
		return false, "", fmt.Errorf("Error Unmarshal input: %v", err)
	}

	txM := &transaction.Transaction{}
	txM.FromProto(txD.Transaction)

	fileAbi, _ := file_handler.GetFileAbi()
	name, _ := fileAbi.ParseMethodName(txM)

	if txM.ToAddress() == file_handler.PredictContractAddress(ethCommon.HexToAddress(appCtx.ClientTcp.GetClientContext().Config.OwnerFileStorageAddress)) && name == "uploadChunk" {
		fileHandler, err := file_handler.GetFileHandlerTCP(appCtx.ClientTcp, appCtx.TcpCfg)
		if err != nil {
			return true, "", fmt.Errorf("Failed to build transaction: " + err.Error())
		}
		isPrevent, err := fileHandler.HandleFileTransactionNoReceipt(context.Background(), txM)
		if err != nil {
			return true, "", fmt.Errorf("Failed to build transaction: " + err.Error())
		}
		if isPrevent {
			return true, txM.Hash().Hex(), nil
		}
		return true, "", fmt.Errorf("Failed to process transaction in fast-path")
	}

	return false, "", nil
}

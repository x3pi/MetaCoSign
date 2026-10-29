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
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/meta-node-blockchain/meta-node/pkg/transaction"
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

	if !appCtx.Cfg.DisableFreeGas && as.Balance().Cmp(appCtx.Cfg.GetFreeGasMinBalance()) < 0 {
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
		errMsg := "null transaction"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		return utils.MakeInternalError(id, errMsg)
	}
}

func ProcessSendRawTransactionWithDeviceKey(appCtx *app.Context, rawTransactionHex string, id interface{}) (bool, string, error) {
	return false, "", nil
}

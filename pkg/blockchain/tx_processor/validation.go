package tx_processor

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/meta-node-blockchain/meta-node/pkg/blockchain"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/transaction"
	"github.com/meta-node-blockchain/meta-node/pkg/utils"
	"github.com/meta-node-blockchain/meta-node/types"
)

func callDataToAccountType(callData []byte) (pb.ACCOUNT_TYPE, *transaction.TransactionError) {

	if len(callData) != 36 {
		return pb.ACCOUNT_TYPE_REGULAR_ACCOUNT, transaction.InvalidData
	}
	bytesFSelect := utils.GetFunctionSelector("setAccountType(int256)")
	// Kiểm tra 4 byte đầu tiên phải bằng "0x61e1270b"
	if !bytes.Equal(callData[:4], bytesFSelect) {
		return pb.ACCOUNT_TYPE_REGULAR_ACCOUNT, transaction.InvalidData
	}
	// Lấy 4 byte sau để xác định kiểu tài khoản
	num := int32(binary.BigEndian.Uint32(callData[len(callData)-4:]))

	switch num {
	case 0:
		return pb.ACCOUNT_TYPE_REGULAR_ACCOUNT, nil
	case 1:
		return pb.ACCOUNT_TYPE_READ_WRITE_STRICT, nil
	default:
		return pb.ACCOUNT_TYPE_REGULAR_ACCOUNT, transaction.InvalidData
	}
}

func VerifyTransaction(
	tx types.Transaction,
	chainState *blockchain.ChainState,

) *transaction.TransactionError {
	var as types.AccountState
	var err error

	as, err = chainState.GetAccountStateDB().AccountState(tx.FromAddress())
	if err != nil {
		panic(fmt.Sprintf("verifyTransaction AccountState %v", err))
	}
	if tx.GetNonce() != as.Nonce() {
		logger.Error("tx.GetNonce() ", tx.GetNonce(), as.Nonce())
		return transaction.InvalidNonce
	}

	if as.Nonce() != 0 || tx.ToAddress() != utils.GetAddressSelector(common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		request := transaction.NewVerifyTransactionRequest(
			tx.Hash(),
			common.PubkeyFromBytes(as.PublicKeyBls()),
			tx.Sign(),
		)
		if !request.Valid() {
			return transaction.InvalidSign
		}
	}

	if as.AccountType() == 1 && tx.ToAddress() != utils.GetAddressSelector(common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		if !tx.ValidEthSign() {
			return transaction.RequiresTwoSignatures
		}
	}

	if tx.ToAddress() == utils.GetAddressSelector(common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		dataInput := tx.CallData().Input()

		if len(dataInput) < 4 {
			return transaction.InvalidData
		}

		selector := dataInput[:4]
		isSetBls := bytes.Equal(selector, utils.GetFunctionSelector("setBlsPublicKey(bytes)"))
		isSetType := bytes.Equal(selector, utils.GetFunctionSelector("setAccountType(uint8)"))

		switch {
		case as.Nonce() == 0 && isSetBls:
			if !tx.ValidEthSign() {
				return transaction.InvalidSignSecp
			}
			_, err := UnpackSetBlsPublicKeyInput(dataInput)
			if err != nil {
				return transaction.InvalidData
			}
			if len(as.PublicKeyBls()) != 0 {
				return transaction.PublicKeyExists
			}
		case as.Nonce() != 0 && isSetType:
			_, err := UnpackSetAccountTypeInput(dataInput)
			if err != nil {
				return transaction.InvalidData
			}
		default:
			return transaction.InvalidData
		}
	} else {
		if as.Nonce() == 0 {
			return transaction.InvalidAddressMatchForTx0
		}
		if !tx.ValidDeployData() {
			return transaction.InvalidDeployData
		}
		if !tx.ValidCallData() {
			return transaction.InvalidCallData
		}
	}

	// Thêm kiểm tra kích thước Call Data
	const maxDataSize = 6 * 1024 * 1024
	if len(tx.Data()) > maxDataSize  {
		logger.Error("Transaction data size exceeds limit", "hash", tx.Hash().Hex(), "size", len(tx.Data()), "limit", maxDataSize)
		return transaction.InvalidData // Sử dụng lỗi InvalidData hoặc tạo lỗi mới nếu cần
	}

	if tx.ToAddress() == tx.FromAddress() && tx.GetNonce() != 0 {
		_, erR := callDataToAccountType(tx.Data())
		if erR != nil {
			return erR
		}
	}

	if !tx.ValidChainID(chainState.GetConfig().ChainId.Uint64()) || err != nil {
		return transaction.InvalidChainId
	}

	// validTx0, errCode := tx.ValidTx0(as, chainState.GetConfig().ChainId.String())

	// if !validTx0 {
	// 	return transaction.CodeToError[errCode]
	// }

	// verify pending use
	if !tx.ValidPendingUse(as) {
		return transaction.InvalidPendingUse
	}

	// nếu mà là giao dịch chuyển native token
	// thì maxGas sẽ là định phí
	// chỉ cho phép lơn hơn 10 lần phí cố định
	// ngược lại nếu là smart contract thì phí lơn hơn tối đa là 10.000 lần
	_, isFree := chainState.GetFreeFeeAddress()[tx.ToAddress()]
	if tx.GetNonce() == 0 {
		isFree = true
	}
	// isFree = true

	// refundGas := tx.MaxGas() - common.TRANSFER_GAS_COST
	// if !isFree && refundGas < 0 {
	// 	return transaction.InvalidMaxGas
	// }
	// if len(tx.Data()) == 0 {
	// 	maxRefundLimit := uint64(common.TRANSFER_GAS_COST * 10) // Chuyển đổi sang uint64
	// 	if tx.MaxGas() > maxRefundLimit {
	// 		fileLogger.Info("tx MaxGas:  1", tx.MaxGas(), maxRefundLimit)
	// 		return transaction.InvalidMaxGas
	// 	}
	// } else {
	// 	maxRefundLimit := uint64(common.TRANSFER_GAS_COST * 10000) // Chuyển đổi sang uint64
	// 	if tx.MaxGas() > maxRefundLimit {
	// 		fileLogger.Info("tx MaxGas: 2", tx.MaxGas(), maxRefundLimit)
	// 		return transaction.InvalidMaxGas
	// 	}
	// }
	// verify amount
	// if tx.IsCallContract() {
	// 	if !isFree && !tx.ValidAmount(as, common.MINIMUM_BASE_FEE) {
	// 		return transaction.InvalidAmount
	// 	}
	// }

	if !tx.ValidAmount(as) {
		return transaction.InvalidAmount
	}

	if !isFree && !tx.ValidMaxFee(as) {
		return transaction.InvalidMaxFee
	}

	// kiểm tra số dư có đủ cho max price
	// maxFee := tx.MaxFee()
	// if !isFree && maxFee.Cmp(big.NewInt(common.MINIMUM_BASE_FEE)) < 0 {
	// 	logger.Error("maxFee", maxFee)
	// 	return transaction.InvalidAmount
	// }

	// if !isFree && !tx.ValidAmountSpend(as, maxFee) {
	// 	logger.Error("Error when execute transaction code 120003: maxFee")
	// 	logger.Error("Error when execute transaction code 120003: detail as", as.Balance(), as.PendingBalance())
	// 	logger.Error("Error when execute transaction code 120003: detail mf", maxFee, tx.Amount())
	// 	return transaction.InvalidMaxGasPrice
	// }

	// if (!isFree && tx.ValidMaxGas() ) {
	// 	return transaction.InvalidMaxGas
	// }

	// verify last hash

	// Debug
	// neu newDeviceKey ma bang voi as.DeviceKey() thi bao loi
	if tx.NewDeviceKey() == as.DeviceKey() && as.Nonce() != 0 {
		return transaction.InvalidNewDeviceKey
	}

	// // verify device key
	if !tx.ValidDeviceKey(as) {
		return transaction.InvalidLastDeviceKey
	}
	return nil
}

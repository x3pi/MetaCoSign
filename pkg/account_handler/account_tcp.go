package account_handler

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	utilsPkg "github.com/meta-node-blockchain/meta-node/pkg/utils"
)

// SendHelpPayTransfer đẩy transfer request vào hàng đợi chung cho các ví trả hộ.
func (h *AccountHandlerNoReceipt) SendHelpPayTransfer(toAddress ethCommon.Address, amount *big.Int) *TransferTxResult {
	resultCh := make(chan *TransferTxResult, 1)
	h.helpPayTxQueue <- &TransferTxRequest{
		ToAddress: toAddress,
		Amount:    amount,
		ResultCh:  resultCh,
	}
	return <-resultCh
}

func (h *AccountHandlerNoReceipt) SendTransfer(
	fromAddress, toAddress ethCommon.Address,
	amount *big.Int,
) *TransferTxResult {
	resultCh := make(chan *TransferTxResult, 1)

	// Lấy hoặc tạo queue riêng cho từ wallet (fromAddress)
	var queue chan *TransferTxRequest
	if val, ok := h.userTxQueues.Load(fromAddress); ok {
		queue = val.(chan *TransferTxRequest)
	} else {
		// Tạo queue mới size 100 cho ví này
		queue = make(chan *TransferTxRequest, 100)
		if actual, loaded := h.userTxQueues.LoadOrStore(fromAddress, queue); loaded {
			// Đã có thread khác tạo trước, dùng cái đã tạo
			queue = actual.(chan *TransferTxRequest)
		} else {
			// Đây là queue mới, cần khởi tạo worker cho queue này
			go h.processUserTxQueue(fromAddress, queue)
		}
	}

	queue <- &TransferTxRequest{
		FromAddress: fromAddress,
		ToAddress:   toAddress,
		Amount:      amount,
		ResultCh:    resultCh,
	}
	return <-resultCh
}

// processUserTxQueue xử lý tuần tự các transaction từ một ví cụ thể.
func (h *AccountHandlerNoReceipt) processUserTxQueue(fromAddress ethCommon.Address, queue chan *TransferTxRequest) {
	logger.Info("🚀 User TX Queue worker started for wallet %s", fromAddress.Hex())
	for req := range queue {
		result := h.executeTransfer(req)
		req.ResultCh <- result
	}
}

// executeTransfer thực hiện 1 transfer transaction qua TCP
func (h *AccountHandlerNoReceipt) executeTransfer(req *TransferTxRequest) *TransferTxResult {
	chainConn, err := h.appCtx.ChainPool.Get()
	if err != nil {
		return &TransferTxResult{Err: fmt.Errorf("get chain connection error: %w", err)}
	}

	metaTxData, _, releaseFunc, err := h.appCtx.ClientRpc.BuildTransferTransactionTCP(
		req.FromAddress, req.ToAddress, req.Amount, chainConn,
	)
	if err != nil {
		return &TransferTxResult{Err: fmt.Errorf("failed to build transfer transaction (From: %s): %w", req.FromAddress.Hex(), err)}
	}

	txBLS, err := chainConn.SendTransactionWithDeviceKey(metaTxData, 120*time.Second)
	if releaseFunc != nil {
		releaseFunc()
	}
	if err != nil {
		return &TransferTxResult{Err: fmt.Errorf("TCP send transfer error (From: %s): %w", req.FromAddress.Hex(), err)}
	}
	txHash := "0x" + hex.EncodeToString(txBLS)
	logger.Info("✅ Transfer sent via TCP, tx hash: %s", txHash)

	_, err = utilsPkg.WaitForReceiptTCP(chainConn, txHash, 30*time.Second)
	if err != nil {
		logger.Error("Wait for transfer receipt error: %v", err)
	}

	return &TransferTxResult{TxHash: txHash}
}

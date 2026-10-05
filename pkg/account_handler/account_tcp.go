package account_handler

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	mt_transaction "github.com/meta-node-blockchain/meta-node/pkg/transaction"
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
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lấy kết nối chain thất bại cho ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}

	// 1. Lấy private key secp256k1 của FromAddress
	var privKey *ecdsa.PrivateKey
	if exists, _ := h.appCtx.PKS.HasPrivateKey(req.FromAddress); exists {
		pkHex, _ := h.appCtx.PKS.GetPrivateKey(req.FromAddress)
		privKey, err = crypto.HexToECDSA(strings.TrimPrefix(pkHex, "0x"))
	} else if h.appCtx.Cfg.PrivateKey != "" {
		privKey, err = crypto.HexToECDSA(strings.TrimPrefix(h.appCtx.Cfg.PrivateKey, "0x"))
	}
	if err != nil || privKey == nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("không tìm thấy private key cho ví chuyển tiền %s: %v", req.FromAddress.Hex(), err),
		}
	}

	// 2. Lấy account state qua TCP để kiểm tra số dư và lấy nonce
	as, err := h.appCtx.ClientRpc.GetAccountStateTCP(req.FromAddress, chainConn)
	if err != nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lỗi lấy account state của ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}
	if as.Balance().Cmp(req.Amount) < 0 {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("ví chuyển tiền %s không đủ số dư: hiện có %s, cần chuyển %s", req.FromAddress.Hex(), as.Balance().String(), req.Amount.String()),
		}
	}
	nonce := as.Nonce()

	// 3. Tạo và ký Ethereum transaction (ECDSA secp256k1)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(100000)
	signer := types.LatestSignerForChainID(h.appCtx.ClientRpc.ChainId)
	ethTx := types.NewTransaction(nonce, req.ToAddress, req.Amount, gasLimit, gasPrice, nil)
	signedTx, err := types.SignTx(ethTx, signer, privKey)
	if err != nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lỗi ký giao dịch từ ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}

	// 4. Chuyển đổi thành MetaNode Transaction và gửi qua TCP
	mtTx, err := mt_transaction.NewTransactionFromEth(signedTx)
	if err != nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lỗi convert eth tx sang metanode tx cho ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}
	bTransaction, err := mtTx.Marshal()
	if err != nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lỗi marshal metanode tx cho ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}

	err = chainConn.SendTransaction(bTransaction)
	if err != nil {
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			Err:         fmt.Errorf("lỗi gửi TCP giao dịch từ ví chuyển tiền %s: %w", req.FromAddress.Hex(), err),
		}
	}

	txHash := signedTx.Hash().Hex()
	logger.Info("✅ Giao dịch chuyển tiền từ ví %s đến %s (số lượng %s) đã gửi qua TCP, tx hash: %s", req.FromAddress.Hex(), req.ToAddress.Hex(), req.Amount.String(), txHash)

	_, err = utilsPkg.WaitForReceiptTCP(chainConn, txHash, 30*time.Second)
	if err != nil {
		logger.Error("❌ Lỗi chờ receipt cho giao dịch từ ví chuyển tiền %s (tx: %s): %v", req.FromAddress.Hex(), txHash, err)
		return &TransferTxResult{
			FromAddress: req.FromAddress,
			TxHash:      txHash,
			Err:         fmt.Errorf("lỗi chờ receipt giao dịch từ ví chuyển tiền %s (tx %s): %w", req.FromAddress.Hex(), txHash, err),
		}
	}

	return &TransferTxResult{
		FromAddress: req.FromAddress,
		TxHash:      txHash,
	}
}

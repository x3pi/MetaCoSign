package account_handler

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/account_handler/abi_account"
	"github.com/meta-node-blockchain/meta-node/pkg/bls"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	utilsPkg "github.com/meta-node-blockchain/meta-node/pkg/utils"
	mt_types "github.com/meta-node-blockchain/meta-node/types"
)

// Nó không còn chứa chainState nữa.
type AccountHandlerNoReceipt struct {
	abi     abi.ABI
	storage *storage.BlsAccountStorage
	appCtx  *app.Context
}

var (
	accountHandlerInstance *AccountHandlerNoReceipt
	accountOnce            sync.Once
)

func GetAccountHandler(appCtx *app.Context) (*AccountHandlerNoReceipt, error) {
	var err error
	accountOnce.Do(func() {
		var parsedABI abi.ABI
		parsedABI, err = abi.JSON(strings.NewReader(abi_account.AccountABI))
		if err != nil {
			return
		}

		accountHandlerInstance = &AccountHandlerNoReceipt{
			abi:     parsedABI,
			storage: storage.NewBlsAccountStorage(appCtx.LdbBlsWallet),
			appCtx:  appCtx,
		}
	})

	return accountHandlerInstance, err
}

// HandleAccountTransaction xử lý các giao dịch liên quan đến account
func (h *AccountHandlerNoReceipt) HandleAccountTransaction(
	ctx context.Context,
	tx mt_types.Transaction,
	rawTransactionHex string,
) (handled bool, result interface{}, err error) {
	// Kiểm tra địa chỉ đích
	if tx.ToAddress() != utilsPkg.GetAddressSelector(common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		return false, nil, nil
	}

	inputData := tx.CallData().Input()
	if len(inputData) < 4 {
		return false, nil, fmt.Errorf("dữ liệu input không hợp lệ")
	}

	method, err := h.abi.MethodById(inputData[:4])
	if err != nil {
		return false, nil, fmt.Errorf("lỗi khi lấy method từ input data: %v", err)
	}

	switch method.Name {
	case "setBlsPublicKey":
		err = h.handleSetBlsPublicKey(tx, method, inputData[4:], rawTransactionHex)
		return true, nil, err

	case "confirmAccount":
		result, err = h.handleConfirmAccount(tx, method, inputData[4:])
		return true, result, err

	case "getAllAccount":
		result, err = h.handleGetAllAccount(tx, method, inputData[4:])
		return true, result, err

	default:
		return false, nil, nil
	}
}
func (h *AccountHandlerNoReceipt) handleSetBlsPublicKey(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) error {
	// Unpack input data
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return fmt.Errorf("lỗi khi unpack input data: %v", err)
	}

	blsPublicKeyBytes, ok := args[0].([]byte)
	if !ok {
		return fmt.Errorf("invalid BLS public key format")
	}
	fromAddress := tx.FromAddress()

	accountData := &pb.BlsAccountData{
		Address:        fromAddress.Bytes(),
		BlsPublicKey:   blsPublicKeyBytes,
		RegisteredAt:   time.Now().Unix(),
		RegisterTxHash: tx.Hash().Bytes(),
		IsConfirmed:    false,
	}

	if err := h.storage.SaveBlsAccount(accountData); err != nil {
		return fmt.Errorf("failed to save account data: %w", err)
	}
	pendingTx := &pb.PendingTransaction{
		Address:           fromAddress.Bytes(),
		BlsPublicKey:      blsPublicKeyBytes,
		RawTransactionHex: rawTransactionHex, // Chỉ lưu hex string
		CreatedAt:         time.Now().Unix(),
		Nonce:             tx.GetNonce(),
		OriginalGasPrice:  0,
	}

	if err := h.storage.SavePendingTransaction(pendingTx); err != nil {
		return fmt.Errorf("failed to save pending transaction: %w", err)
	}
	if err := h.storage.AddAccountToBlsPublicKey(blsPublicKeyBytes, fromAddress, false); err != nil {
		return fmt.Errorf("failed to add account to BLS list: %w", err)
	}
	return nil
}

func (h *AccountHandlerNoReceipt) handleConfirmAccount(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
) (string, error) {

	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return "", fmt.Errorf("lỗi khi unpack input data: %v", err)
	}

	accountAddress, ok := args[0].(ethCommon.Address)
	if !ok {
		return "", fmt.Errorf("invalid account address format")
	}
	pendingTx, err := h.storage.GetPendingTransaction(accountAddress)
	if err != nil {
		return "", fmt.Errorf("pending transaction not found: %w", err)
	}
	// ========== REBUILD TRANSACTION TỪ rawTransactionHex ==========
	rawTransactionHex := pendingTx.RawTransactionHex

	// Decode hex
	decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(rawTransactionHex)
	if err != nil {
		return "", fmt.Errorf("invalid raw transaction hex: %w", err)
	}
	defer func() {
		if releaseDecoded != nil {
			releaseDecoded()
		}
	}()

	// Unmarshal Ethereum transaction
	ethTx := new(types.Transaction)
	if err := ethTx.UnmarshalBinary(decodedTxBytes); err != nil {
		return "", fmt.Errorf("failed to unmarshal ethereum transaction: %w", err)
	}

	// Verify sender
	signer := types.LatestSignerForChainID(h.appCtx.ClientRpc.ChainId)
	fromAddress, err := types.Sender(signer, ethTx)
	if err != nil {
		return "", fmt.Errorf("failed to derive sender: %w", err)
	}

	if fromAddress != accountAddress {
		return "", fmt.Errorf("sender mismatch: expected %s, got %s", accountAddress.Hex(), fromAddress.Hex())
	}

	// Tạo transaction mới với gas price = 0
	newTx := types.NewTx(&types.LegacyTx{
		Nonce:    ethTx.Nonce(),
		To:       ethTx.To(),
		Value:    ethTx.Value(),
		Gas:      ethTx.Gas(),
		GasPrice: big.NewInt(0), // SET GAS PRICE = 0
		Data:     ethTx.Data(),
	})

	// Check private key từ PKS hoặc dùng node's BLS key
	var (
		bTx       []byte
		mtTx      mt_types.Transaction
		releaseTx func()
		buildErr  error
	)

	exists, err := h.appCtx.PKS.HasPrivateKey(fromAddress)
	if err != nil {
		return "", fmt.Errorf("error checking private key store: %w", err)
	}

	if !exists {
		// Dùng device key
		bTx, mtTx, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTx(newTx)
	} else {
		// Dùng BLS private key từ PKS
		senderPkString, _ := h.appCtx.PKS.GetPrivateKey(fromAddress)
		keyPair := bls.NewKeyPair(ethCommon.FromHex(senderPkString))
		bTx, mtTx, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTxAndBlsPrivateKey(
			newTx,
			keyPair.PrivateKey(),
		)
	}

	if buildErr != nil {
		return "", fmt.Errorf("failed to build transaction: %w", buildErr)
	}

	// Marshal new Ethereum transaction
	newEthTxBytes, err := newTx.MarshalBinary()
	if err != nil {
		if releaseTx != nil {
			releaseTx()
		}
		return "", fmt.Errorf("failed to marshal new transaction: %w", err)
	}

	// Gửi transaction với SendRawTransactionBinary
	rs := h.appCtx.ClientRpc.SendRawTransactionBinary(
		bTx,
		releaseTx,
		newEthTxBytes,
		nil,
		nil,
	)

	if rs.Error != nil {
		return "", fmt.Errorf("failed to send transaction: %v", rs.Error)
	}

	// Cập nhật trạng thái confirmed
	if err := h.storage.MarkAccountConfirmed(
		accountAddress,
		mtTx.Hash().Bytes(),
		pendingTx.BlsPublicKey,
	); err != nil {
		logger.Error("Failed to mark account as confirmed: %v", err)
	}

	// Xóa pending transaction
	if err := h.storage.DeletePendingTransaction(accountAddress); err != nil {
		logger.Error("Failed to delete pending transaction: %v", err)
	}

	logger.Info("✅ Đã confirm account %s, tx hash: %v", accountAddress.Hex(), rs.Result)
	return rs.Result.(string), nil
}

func (h *AccountHandlerNoReceipt) handleGetAllAccount(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
) (interface{}, error) {
	// Signature: getAllAccount(bytes memory _sign, bytes memory _publicKeyBls, uint _time, uint _page, uint _pageSize, bool _isConfirm)
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi unpack input data: %v", err)
	}

	signBytes, _ := args[0].([]byte)
	blsPublicKeyBytes, _ := args[1].([]byte)
	timestamp, _ := args[2].(*big.Int)
	page, _ := args[3].(*big.Int)
	pageSize, _ := args[4].(*big.Int)
	isConfirmed, _ := args[5].(bool)

	if !h.verifySignature(signBytes, blsPublicKeyBytes, timestamp.Uint64()) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Parse page và pageSize từ big.Int
	pageNum := int(page.Int64())
	pageSizeNum := int(pageSize.Int64())

	// Validate pagination parameters
	if pageNum < 0 {
		pageNum = 0
	}
	if pageSizeNum <= 0 || pageSizeNum > 100 {
		pageSizeNum = 20 // Default size, max 100
	}

	logger.Info("📋 Lấy danh sách accounts: page=%d, pageSize=%d, confirmed=%v",
		pageNum, pageSizeNum, isConfirmed)

	// Lấy accounts từ LevelDB (filter by confirmation status)
	accounts, total, err := h.storage.GetAccountsByBlsPublicKey(
		blsPublicKeyBytes,
		pageNum,
		pageSizeNum,
		isConfirmed,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	// Trả về kết quả
	result := map[string]interface{}{
		"accounts":  accounts,
		"total":     total,
		"page":      pageNum,
		"pageSize":  pageSizeNum,
		"totalPage": (total + pageSizeNum - 1) / pageSizeNum,
		"confirmed": isConfirmed,
	}

	logger.Info("✅ Trả về %d accounts (tổng: %d)", len(accounts), total)
	return result, nil

}
func (h *AccountHandlerNoReceipt) verifySignature(signBytes, blsPublicKeyBytes []byte, timestamp uint64) bool {
	now := uint64(time.Now().Unix())
	if now > timestamp+300 || now < timestamp-300 {
		logger.Warn("Timestamp out of range: %d, now: %d", timestamp, now)
		return false
	}

	// TODO: Implement proper BLS verification
	return len(signBytes) > 0 && len(blsPublicKeyBytes) > 0
}

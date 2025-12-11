package account_handler

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/account_handler/abi_account"
	"github.com/meta-node-blockchain/meta-node/pkg/bls"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	utilsPkg "github.com/meta-node-blockchain/meta-node/pkg/utils"
	mt_types "github.com/meta-node-blockchain/meta-node/types"
	"github.com/syndtr/goleveldb/leveldb"
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
	case "setAccountType":
		return false, nil, nil
	default:
		return false, nil, nil
	}
}
func (h *AccountHandlerNoReceipt) HandleEthCall(ctx context.Context, data []byte) (interface{}, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid call data: too short")
	}
	// Lấy method signature từ 4 bytes đầu
	method, err := h.abi.MethodById(data[:4])
	if err != nil {
		return nil, fmt.Errorf("method not found: %w", err)
	}
	// Chỉ handle getAllAccount cho eth_call
	switch method.Name {
	case "getAllAccount":
		return h.handleGetAllAccount(method, data[4:])
	case "getNotifications": // ✅ THÊM
		return h.handleGetNotifications(method, data[4:])

	default:
		return nil, fmt.Errorf("unsupported eth_call method: %s", method.Name)
	}
}

func (h *AccountHandlerNoReceipt) handleSetBlsPublicKey(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) error {
	logger.Info("Handling setBlsPublicKey for tx %s", tx.Hash().Hex())
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
	currentTime := time.Now().Unix()
	adminAddress := ethCommon.HexToAddress(h.appCtx.Cfg.OwnerRpcAddress)
	accountData := &pb.BlsAccountData{
		Address:        fromAddress.Bytes(),
		BlsPublicKey:   blsPublicKeyBytes,
		RegisteredAt:   time.Now().Unix(),
		RegisterTxHash: tx.Hash().Bytes(),
		IsConfirmed:    false,
	}

	// Lưu account data vào unconfirmed storage
	if err := h.storage.AddAccountToBlsPublicKey(accountData, false); err != nil {
		return fmt.Errorf("failed to save account data: %w", err)
	}

	// Lưu pending transaction
	pendingTx := &pb.PendingTransaction{
		Address:           fromAddress.Bytes(),
		BlsPublicKey:      blsPublicKeyBytes,
		RawTransactionHex: rawTransactionHex,
		CreatedAt:         time.Now().Unix(),
		Nonce:             tx.GetNonce(),
		OriginalGasPrice:  0,
	}

	if err := h.storage.SavePendingTransaction(pendingTx); err != nil {
		return fmt.Errorf("failed to save pending transaction: %w", err)
	}
	msgNoti := fmt.Sprintf("BLS registered for account %s", fromAddress.Hex())
	notification := &pb.Notification{
		AccountAddress: adminAddress.Bytes(),
		Message:        msgNoti,
		CreatedAt:      currentTime,
	}
	if err := h.appCtx.LdbNotification.SaveNotification(notification); err != nil {
		logger.Error("Failed to save notification: %v", err)
		return fmt.Errorf("Failed to save notification: %v", err)
	}
	h.broadcastEvent(
		"RegisterBls",
		adminAddress,
		big.NewInt(currentTime),
		blsPublicKeyBytes,
		msgNoti,
	)
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
	accountAddress, _ := args[0].(ethCommon.Address)
	timestamp, _ := args[1].(*big.Int)
	signatureBytes, _ := args[2].([]byte)

	currentTime := time.Now().Unix()
	if utilsPkg.Abs(currentTime-timestamp.Int64()) > 300 {
		return "", fmt.Errorf("timestamp expired (current: %d, provided: %d)", currentTime, timestamp.Int64())
	}
	message := make([]byte, 0, 52) // 20 bytes address + 32 bytes uint256
	message = append(message, accountAddress.Bytes()...)
	timestampBytes := make([]byte, 32)
	timestamp.FillBytes(timestampBytes)
	message = append(message, timestampBytes...)

	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	messageHash := crypto.Keccak256Hash([]byte(prefixedMessage))
	if len(signatureBytes) < 65 {
		return "", fmt.Errorf(
			"invalid signature length: expected at least 65, got %d",
			len(signatureBytes),
		)
	}
	// Adjust V value (Ethereum uses 27/28, crypto.Ecrecover expects 0/1)
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}
	// Recover public key
	pubKey, err := crypto.SigToPub(messageHash.Bytes(), signatureBytes)
	if err != nil {
		return "", fmt.Errorf("failed to recover public key: %w", err)
	}

	// Get signer address
	signerAddress := crypto.PubkeyToAddress(*pubKey)
	if signerAddress != ethCommon.HexToAddress(h.appCtx.Cfg.OwnerRpcAddress) {
		return "", fmt.Errorf("invalid signature: signer %s is not authorized", signerAddress.Hex())
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
	logger.Info("Rebuilding transaction for account %s, exists in PKS: %v", fromAddress.Hex(), exists)
	if !exists {
		bTx, mtTx, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTx(ethTx)
	} else {
		senderPkString, _ := h.appCtx.PKS.GetPrivateKey(fromAddress)
		keyPair := bls.NewKeyPair(ethCommon.FromHex(senderPkString))
		bTx, mtTx, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTxAndBlsPrivateKey(
			ethTx,
			keyPair.PrivateKey(),
		)
	}
	if buildErr != nil {
		return "", fmt.Errorf("failed to build transaction: %w", buildErr)
	}
	rs := h.appCtx.ClientRpc.SendRawTransactionBinary(
		bTx,
		releaseTx,
		decodedTxBytes,
		releaseDecodedOnce,
		nil,
	)
	if rs.Error != nil {
		return "", fmt.Errorf("failed to send transaction: %v", rs.Error)
	}
	metaTxData, _, releaseFunc, err := h.appCtx.ClientRpc.BuildTransferTransaction(ethCommon.HexToAddress(h.appCtx.Cfg.OwnerRpcAddress), ethCommon.Address(pendingTx.Address), h.appCtx.Cfg.RewardAmount)
	rst := h.appCtx.ClientRpc.SendRawTransactionBinary(
		metaTxData,
		releaseFunc,
		nil,
		nil,
		nil,
	)
	if rs.Error != nil {
		return "", fmt.Errorf("failed to send transaction: %v", rst.Error)
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
	// ✅ TẠO NOTIFICATION VÀ BROADCAST EVENT
	msgNoti := fmt.Sprintf("Your account %s has been successfully confirmed!", accountAddress.Hex())

	notification := &pb.Notification{
		AccountAddress: accountAddress.Bytes(),
		Message:        msgNoti,
		CreatedAt:      currentTime,
	}
	if err := h.appCtx.LdbNotification.SaveNotification(notification); err != nil {
		logger.Error("Failed to save notification: %v", err)
		return "", fmt.Errorf("Failed to save notification: %v", err)
	}
	h.broadcastEvent("AccountConfirmed", accountAddress, big.NewInt(currentTime), msgNoti)

	logger.Info("✅ Đã confirm account %s, tx hash: %v", accountAddress.Hex(), rs.Result)
	return rs.Result.(string), nil
}

func (h *AccountHandlerNoReceipt) broadcastEvent(
	eventName string,
	eventArgs ...interface{},
) error {
	addressContract := ethCommon.HexToAddress(h.appCtx.Cfg.ContractsInterceptor[0])
	event, ok := h.abi.Events[eventName]
	if !ok {
		return fmt.Errorf("event %s not found in ABI", eventName)
	}
	eventHash := event.ID
	argIndex := 0
	eventTopics := []string{eventHash.Hex()}
	nonIndexedArgs := make([]interface{}, 0)
	for _, input := range event.Inputs {
		if argIndex >= len(eventArgs) {
			break
		}
		logger.Info("Processing event arg %d: input %v \n indexed=%v, type=%v", argIndex, input, input.Indexed, input.Type.String())
		if input.Indexed {
			topicValue, err := utilsPkg.EncodeIndexedTopic(eventArgs[argIndex], input.Type)
			if err != nil {
				logger.Error("Failed to encode indexed topic: %v", err)
				return err
			}
			eventTopics = append(eventTopics, topicValue)
		} else {
			nonIndexedArgs = append(nonIndexedArgs, eventArgs[argIndex])
		}
		argIndex++
	}
	// Pack event data
	eventData, err := event.Inputs.NonIndexed().Pack(eventArgs...)
	if err != nil {
		logger.Error("Failed to pack %s event data: %v", eventName, err)
		return fmt.Errorf("failed to pack %s event data: %w", eventName, err)
	}
	eventLogData := map[string]interface{}{
		"address": addressContract,
		"topics": []string{
			eventHash.Hex(), // topics[0]: Event signature
		},
		"data":             fmt.Sprintf("0x%x", eventData),
		"blockNumber":      fmt.Sprintf("0x%x", 1),
		"transactionHash":  fmt.Sprintf("0x%064x", time.Now().UnixNano()),
		"blockHash":        "0xa08082c7663f884e3c4d325ad1de149f6e167a84556be205103c16b1595d22cc",
		"logIndex":         "0x0",
		"transactionIndex": "0x0",
		"removed":          false,
	}

	h.appCtx.SubInterceptor.BroadcastEventToContract(
		addressContract.Hex(),
		[]string{eventHash.Hex()},
		eventLogData,
	)
	logger.Info("✅ Broadcasted %s event", eventName)
	return nil
}
func (h *AccountHandlerNoReceipt) handleGetAllAccount(
	method *abi.Method,
	inputData []byte,
) (interface{}, error) {
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

	ok, err := h.verifySignature(signBytes, blsPublicKeyBytes, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}
	if !ok {
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

	// Lấy accounts từ LevelDB (filter by confirmation status)
	accounts, total, err := h.storage.GetAccountsByBlsPublicKey(
		blsPublicKeyBytes,
		pageNum,
		pageSizeNum,
		isConfirmed,
	)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return map[string]interface{}{
				"accounts":  []map[string]interface{}{},
				"total":     total,
				"page":      pageNum,
				"pageSize":  pageSizeNum,
				"totalPage": (total + pageSizeNum - 1) / pageSizeNum,
				"confirmed": isConfirmed,
			}, nil
		}
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}
	accountsJSON := make([]map[string]interface{}, 0, len(accounts))
	for _, acc := range accounts {
		accountsJSON = append(accountsJSON, map[string]interface{}{
			"address":        ethCommon.BytesToAddress(acc.Address).Hex(),
			"blsPublicKey":   "0x" + ethCommon.Bytes2Hex(acc.BlsPublicKey),
			"registeredAt":   acc.RegisteredAt,
			"registerTxHash": "0x" + ethCommon.Bytes2Hex(acc.RegisterTxHash),
			"isConfirmed":    acc.IsConfirmed,
			"confirmedAt":    acc.ConfirmedAt,
			"confirmTxHash":  "0x" + ethCommon.Bytes2Hex(acc.ConfirmTxHash),
		})
	}
	// Trả về kết quả
	result := map[string]interface{}{
		"accounts":  accountsJSON,
		"total":     total,
		"page":      pageNum,
		"pageSize":  pageSizeNum,
		"totalPage": (total + pageSizeNum - 1) / pageSizeNum,
		"confirmed": isConfirmed,
	}

	logger.Info("✅ Trả về %d accounts (tổng: %d)", len(accounts), total)
	return result, nil

}
func (h *AccountHandlerNoReceipt) handleGetNotifications(
	method *abi.Method,
	inputData []byte,
) (interface{}, error) {
	logger.Info("🔍 Handling getNotifications...")
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack input: %w", err)
	}

	accountAddress := args[0].(ethCommon.Address)
	page := int(args[1].(*big.Int).Int64())
	pageSize := int(args[2].(*big.Int).Int64())

	notifications, total, err := h.appCtx.LdbNotification.GetNotifications(
		accountAddress,
		page,
		pageSize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}
	// ✅ Convert notifications to JSON-serializable format
	notifList := make([]map[string]interface{}, len(notifications))
	for i, notif := range notifications {
		notifList[i] = map[string]interface{}{
			"id":        notif.Id,
			"message":   notif.Message,
			"createdAt": notif.CreatedAt,
		}
	}
	totalPage := (total + pageSize - 1) / pageSize
	if totalPage == 0 {
		totalPage = 1
	}
	result := map[string]interface{}{
		"notifications": notifList,
		"total":         total,
		"page":          page,
		"pageSize":      pageSize,
		"totalPages":    totalPage, // ✅ Đổi từ totalPage thành totalPages để match với TypeScript
	}
	return result, nil
}
func (h *AccountHandlerNoReceipt) verifySignature(
	signBytes []byte,
	blsPublicKeyBytes []byte,
	timestamp *big.Int,
) (bool, error) {
	// ✅ 1. Verify timestamp (trong vòng 5 phút)
	currentTime := time.Now().Unix()
	if utilsPkg.Abs(currentTime-timestamp.Int64()) > 300 {
		return false, fmt.Errorf(
			"timestamp expired (current: %d, provided: %d)",
			currentTime,
			timestamp.Int64(),
		)
	}
	// ✅ 2. Check signature length
	if len(signBytes) < 65 {
		return false, fmt.Errorf(
			"invalid signature length: expected at least 65, got %d",
			len(signBytes),
		)
	}
	// Message structure: [blsPublicKey (48 bytes)] + [timestamp (32 bytes)]
	message := make([]byte, 0, len(blsPublicKeyBytes)+32)
	message = append(message, blsPublicKeyBytes...)

	timestampBytes := make([]byte, 32)
	timestamp.FillBytes(timestampBytes)
	message = append(message, timestampBytes...)

	prefixedMessage := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	messageHash := crypto.Keccak256Hash([]byte(prefixedMessage))
	// ✅ 5. Adjust V value (Ethereum uses 27/28, crypto.Ecrecover expects 0/1)
	signature := make([]byte, 65)
	copy(signature, signBytes)
	if signature[64] >= 27 {
		signature[64] -= 27
	}
	pubKey, err := crypto.SigToPub(messageHash.Bytes(), signature)
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %w", err)
	}

	// ✅ 7. Get signer address
	signerAddress := crypto.PubkeyToAddress(*pubKey)

	if signerAddress != ethCommon.HexToAddress(h.appCtx.Cfg.OwnerRpcAddress) {
		return false, fmt.Errorf(
			"invalid signature: signer %s is not authorized",
			signerAddress.Hex(),
		)
	}
	return true, nil
}

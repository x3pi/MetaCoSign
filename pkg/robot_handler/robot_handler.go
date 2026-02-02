package robothandler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/app"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/utils"
	"github.com/meta-node-blockchain/meta-node/pkg/bls"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/robot_handler/abi_robot"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	utilsPkg "github.com/meta-node-blockchain/meta-node/pkg/utils"
	mt_types "github.com/meta-node-blockchain/meta-node/types"
)

type RobotHandler struct {
	abi    abi.ABI
	appCtx *app.Context
	// Queue để đưa giao dịch lên chain với nonce tuần tự
	txQueue chan *QueuedTransaction
	// Map lưu session data tạm thời
	processedCount uint64
	// Map để track các transaction đã xử lý (tránh duplicate)
}

type QueuedTransaction struct {
	RobotAddress      ethCommon.Address
	RawTransactionHex string
	txHash            string
	SessionID         uint64   // Deprecated: không dùng nữa, giữ lại để tương thích
	Method            string   // Deprecated: không dùng nữa, giữ lại để tương thích
	SessionId         [32]byte // Session ID từ dispatch
	ActionId          [32]byte // Action ID từ dispatch
	Data              []byte   // Data từ dispatch (input data gửi lên)
}

var (
	robotHandlerInstance *RobotHandler
	robotOnce            sync.Once
)

func GetRobotHandler(appCtx *app.Context) (*RobotHandler, error) {
	var err error
	robotOnce.Do(func() {
		// Parse ABI từ contract
		parsedABI, parseErr := abi.JSON(strings.NewReader(abi_robot.RobotABI))
		if parseErr != nil {
			err = parseErr
			return
		}

		handler := &RobotHandler{
			abi:     parsedABI,
			appCtx:  appCtx,
			txQueue: make(chan *QueuedTransaction, 5000),
		}

		// Khởi động worker để xử lý queue
		go handler.processTransactionQueue()
		robotHandlerInstance = handler
	})

	return robotHandlerInstance, err
}

// serializeInputData serialize sessionId, actionId, data thành JSON string
func serializeInputData(sessionId, actionId [32]byte, data []byte) string {
	inputMap := map[string]interface{}{
		"sessionId": fmt.Sprintf("0x%x", sessionId),
		"actionId":  fmt.Sprintf("0x%x", actionId),
		"data":      fmt.Sprintf("0x%x", data),
	}
	jsonBytes, err := json.Marshal(inputMap)
	if err != nil {
		// Fallback nếu marshal lỗi
		return fmt.Sprintf(`{"sessionId":"0x%x","actionId":"0x%x","data":"0x%x"}`, sessionId, actionId, data)
	}
	return string(jsonBytes)
}

// HandleRobotTransaction xử lý giao dịch robot NGAY LẬP TỨC (không check nonce)
func (h *RobotHandler) HandleRobotTransaction(
	ctx context.Context,
	tx mt_types.Transaction,
	rawTransactionHex string,
) (handled bool, result interface{}, err error) {
	inputData := tx.CallData().Input()
	if len(inputData) < 4 {
		return false, nil, nil
	}

	method, err := h.abi.MethodById(inputData[:4])
	if err != nil {
		return false, nil, nil
	}

	switch method.Name {
	case "emitQuestion":
		return h.handleEmitQuestion(tx, method, inputData[4:], rawTransactionHex)
	case "emitAnswer":
		return h.handleEmitAnswer(tx, method, inputData[4:], rawTransactionHex)

	default:
		return false, nil, fmt.Errorf("unknown method: %s", method.Name)
	}
}

func (h *RobotHandler) HandleEthCall(ctx context.Context, data []byte) (interface{}, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid call data: too short")
	}
	// Lấy method signature từ 4 bytes đầu
	method, err := h.abi.MethodById(data[:4])
	if err != nil {
		return nil, fmt.Errorf("method not found: %w", err)
	}
	logger.Info("🔵 [HandleEthCall] method: %s", method.Name)
	// Chỉ handle getAllAccount cho eth_call
	switch method.Name {
	case "getDataByTxhash":
		return h.handleGetDataByTxhash(method, data[4:])
	default:
		return nil, nil
	}
}

// handleEmitQuestion xử lý emitQuestion, đưa vào queue và broadcast event QuestionAsked
func (h *RobotHandler) handleEmitQuestion(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) (bool, interface{}, error) {
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		logger.Error("❌ [emitQuestion] Failed to unpack input data: %v", err)
		return false, nil, fmt.Errorf("failed to unpack: %w", err)
	}

	if len(args) < 3 {
		logger.Error("❌ [emitQuestion] Invalid args length: %d", len(args))
		return false, nil, fmt.Errorf("invalid args length")
	}

	user, ok := args[0].(ethCommon.Address)
	if !ok {
		return false, nil, fmt.Errorf("user wrong type")
	}

	conversationId, ok := args[1].(*big.Int)
	if !ok {
		return false, nil, fmt.Errorf("conversationId wrong type")
	}

	question, ok := args[2].([]byte)
	if !ok {
		return false, nil, fmt.Errorf("question wrong type")
	}

	txHash := tx.Hash().Hex()
	var sessionId [32]byte
	copy(sessionId[:], ethCommon.BigToHash(conversationId).Bytes())

	queuedTx := &QueuedTransaction{
		RobotAddress:      tx.FromAddress(),
		RawTransactionHex: rawTransactionHex,
		txHash:            txHash,
		SessionId:         sessionId,
		Data:              question,
	}

	select {
	case h.txQueue <- queuedTx:
		logger.Info("✅ Queued transaction for emitQuestion - conversationId=%s, txHash=%s", conversationId.String(), txHash)
	default:
		logger.Error("❌ Queue is full, cannot queue transaction for emitQuestion - conversationId=%s", conversationId.String())
	}

	// Broadcast QuestionAsked event
	h.broadcastEvent("QuestionAsked", user, conversationId, question)

	return true, txHash, nil
}

// handleEmitAnswer xử lý emitAnswer, đưa vào queue và broadcast event AnswerStored
func (h *RobotHandler) handleEmitAnswer(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) (bool, interface{}, error) {
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		logger.Error("❌ [emitAnswer] Failed to unpack input data: %v", err)
		return false, nil, fmt.Errorf("failed to unpack: %w", err)
	}

	if len(args) < 4 {
		logger.Error("❌ [emitAnswer] Invalid args length: %d", len(args))
		return false, nil, fmt.Errorf("invalid args length")
	}

	user, ok := args[0].(ethCommon.Address)
	if !ok {
		return false, nil, fmt.Errorf("user wrong type")
	}

	conversationId, ok := args[1].(*big.Int)
	if !ok {
		return false, nil, fmt.Errorf("conversationId wrong type")
	}

	answer, ok := args[3].([]byte)
	if !ok {
		return false, nil, fmt.Errorf("answer wrong type")
	}

	txHash := tx.Hash().Hex()
	var sessionId [32]byte
	copy(sessionId[:], ethCommon.BigToHash(conversationId).Bytes())

	queuedTx := &QueuedTransaction{
		RobotAddress:      tx.FromAddress(),
		RawTransactionHex: rawTransactionHex,
		txHash:            txHash,
		SessionId:         sessionId,
		Data:              answer,
	}

	select {
	case h.txQueue <- queuedTx:
		logger.Info("✅ Queued transaction for emitAnswer - conversationId=%s, txHash=%s", conversationId.String(), txHash)
	default:
		logger.Error("❌ Queue is full, cannot queue transaction for emitAnswer - conversationId=%s", conversationId.String())
	}

	// Broadcast AnswerStored event
	h.broadcastEvent("AnswerStored", user, conversationId, answer)

	return true, txHash, nil
}

// handleGetDataByTxhash: Tra cứu transaction và event data từ LevelDB
func (h *RobotHandler) handleGetDataByTxhash(
	method *abi.Method,
	inputData []byte,
) (interface{}, error) {
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		logger.Error("❌ [getDataByTxhash] Failed to unpack input data: %v", err)
		return nil, fmt.Errorf("failed to unpack: %w", err)
	}
	if len(args) < 1 {
		logger.Error("❌ [getDataByTxhash] Invalid args length: %d", len(args))
		return nil, fmt.Errorf("invalid args length")
	}
	// Lấy txHash từ args (bytes32 trong Go ABI là [32]byte, không phải []byte)
	var txHashHex string
	switch v := args[0].(type) {
	case [32]byte:
		txHashHex = ethCommon.BytesToHash(v[:]).Hex()
		logger.Info("🔵 [getDataByTxhash] Received bytes32: %s", txHashHex)
	default:
		logger.Error("❌ [getDataByTxhash] txHash wrong type: %T, value: %v", args[0], args[0])
		return nil, fmt.Errorf("txHash must be bytes32, got %T", args[0])
	}
	// Kiểm tra storage có tồn tại không
	if h.appCtx.LdbRobotTransaction == nil {
		logger.Error("❌ [getDataByTxhash] LdbRobotTransaction is not initialized")
		return nil, fmt.Errorf("transaction storage not available")
	}
	// Tra cứu từ storage
	storedError, err := h.appCtx.LdbRobotTransaction.GetErrorByHash(txHashHex)
	if err != nil {
		logger.Error("❌ [getDataByTxhash] Failed to get transaction: %v", err)
		return nil, fmt.Errorf("error not found: %w", err)
	}
	// Tạo response object (không phải JSON string, để eth_call.go tự marshal)
	response := map[string]interface{}{
		"txHash":       storedError.TxHash,
		"inputData":    storedError.InputData,
		"errorMessage": storedError.ErrorMessage,
		"createdAt":    storedError.CreatedAt,
	}
	logger.Info("✅ [getDataByTxhash] Returning error data for txHash=%s", txHashHex)
	return response, nil
}

// processTransactionQueue: Worker chính xử lý hàng đợi giao dịch
func (h *RobotHandler) processTransactionQueue() {
	logger.Info("🚀 RobotHandler Worker started processing queue...")

	for queuedTx := range h.txQueue {
		success := false
		maxRetries := 3
		var lastError error
		// Vòng lặp Retry 3 lần
		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Gọi hàm xử lý thực thi giao dịch
			err := h.executeSingleTransaction(queuedTx)
			if err == nil {
				success = true
				break
			}

			lastError = err

			// Nếu thất bại, tính toán thời gian chờ tăng dần (Lần 1: 2s, Lần 2: 4s)
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*2) * 400 * time.Millisecond
				logger.Error("⚠️ [Attempt %d/%d] Tx %s failed: %v. Retrying in %v...",
					attempt, maxRetries, queuedTx.txHash, err, waitTime)
				time.Sleep(waitTime)
			}
		}

		// Nếu sau 3 lần vẫn lỗi -> Dừng toàn bộ chương trình
		if !success {
			criticalMsg := fmt.Sprintf("❌ [CRITICAL] Transaction %s failed after %d retries. System must stop to prevent nonce desync. Error: %v",
				queuedTx.txHash, maxRetries, lastError)
			logger.Error(criticalMsg)
			// Lưu lỗi cuối cùng vào LevelDB
			inputDataStr := serializeInputData(queuedTx.SessionId, queuedTx.ActionId, queuedTx.Data)
			h.appCtx.LdbRobotTransaction.SaveError(queuedTx.txHash, inputDataStr, criticalMsg)
			// Phát tán event lỗi cuối cùng
			txHashBytes := ethCommon.HexToHash(queuedTx.txHash)
			h.broadcastEvent("EmitError", txHashBytes, criticalMsg)
			// Dừng chương trình ngay lập tức
			// time.Sleep(1 * time.Second)
			// logger.Error("🛑 Shutting down process due to critical transaction failure.")
			// os.Exit(1)
		}
	}
}

// executeSingleTransaction thực hiện một chu kỳ: Giải mã -> Build -> Gửi -> Chờ Receipt
func (h *RobotHandler) executeSingleTransaction(
	queuedTx *QueuedTransaction,
) error {
	fromAddress := queuedTx.RobotAddress
	count := atomic.AddUint64(&h.processedCount, 1)
	// 1. Giải mã RawTransactionHex
	decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(queuedTx.RawTransactionHex)
	if err != nil {
		return fmt.Errorf("decode hex error: %w", err)
	}
	defer releaseDecoded()

	// 2. Unmarshal Ethereum transaction
	ethTx := new(types.Transaction)
	if err := ethTx.UnmarshalBinary(decodedTxBytes); err != nil {
		return fmt.Errorf("unmarshal binary error: %w", err)
	}

	logger.Info("🔄 Processing Tx: %s | Attempting send...", queuedTx.txHash)

	// 4. Rebuild transaction (nonce tự động từ account state)
	var (
		bTx       []byte
		releaseTx func()
		buildErr  error
	)

	hasKey, _ := h.appCtx.PKS.HasPrivateKey(fromAddress)
	if !hasKey {
		bTx, _, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTx(
			ethTx, h.appCtx.TcpCfg, h.appCtx.Cfg, h.appCtx.LdbContractFreeGas,
		)
	} else {
		senderPkString, _ := h.appCtx.PKS.GetPrivateKey(fromAddress)
		keyPair := bls.NewKeyPair(ethCommon.FromHex(senderPkString))
		bTx, _, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTxAndBlsPrivateKey(
			ethTx, h.appCtx.TcpCfg, h.appCtx.Cfg, h.appCtx.LdbContractFreeGas, keyPair.PrivateKey(),
		)
	}

	if buildErr != nil {
		if releaseTx != nil {
			releaseTx()
		}
		return fmt.Errorf("build transaction error: %w", buildErr)
	}

	// 5. Gửi lên Chain
	rs := h.appCtx.ClientRpc.SendRawTransactionBinary(bTx, releaseTx, nil, nil, nil)
	if rs.Error != nil {
		return fmt.Errorf("RPC send error: %v", rs.Error)
	}

	newTxHash, ok := rs.Result.(string)
	if !ok {
		return fmt.Errorf("invalid result type from RPC")
	}

	// 6. Chờ Receipt (đảm bảo transaction đã lên block trước khi xử lý tx tiếp theo)
	_, err = h.waitForReceipt(newTxHash, 30*time.Second)
	if err != nil {
		return fmt.Errorf("wait for receipt timeout/error: %w", err)
	}

	logger.Info("✅ Tx Success: %s | Total Processed: %d", newTxHash, count)
	return nil
}

func (h *RobotHandler) broadcastEvent(
	eventName string,
	eventArgs ...interface{},
) error {
	addressContract := ethCommon.HexToAddress(h.appCtx.Cfg.ContractsInterceptor[1])
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
		"address":          addressContract,
		"topics":           eventTopics,
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

// waitForReceipt chờ receipt từ RPC với timeout 30s
func (h *RobotHandler) waitForReceipt(txHash string, timeout time.Duration) (map[string]interface{}, error) {
	startTime := time.Now()
	checkInterval := 100 * time.Millisecond // Check mỗi 500ms
	maxAttempts := int(timeout / checkInterval)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Gọi RPC eth_getTransactionReceipt trực tiếp
		request := &rpc_client.JSONRPCRequest{
			Jsonrpc: "2.0",
			Method:  "eth_getTransactionReceipt",
			Params:  []interface{}{txHash},
			Id:      1,
		}

		response := h.appCtx.ClientRpc.SendHTTPRequest(request)

		// Nếu có lỗi, kiểm tra xem có phải là "not found" không
		if response.Error != nil {
			// Nếu lỗi là "not found" hoặc transaction chưa được mined, tiếp tục chờ
			if strings.Contains(response.Error.Message, "not found") ||
				strings.Contains(response.Error.Message, "Not Found") ||
				strings.Contains(strings.ToLower(response.Error.Message), "pending") {
				time.Sleep(checkInterval)
				continue
			}
			return nil, fmt.Errorf("RPC error: %s", response.Error.Message)
		}

		// Nếu có result và không null, transaction đã được mined
		if response.Result != nil {
			// Kiểm tra xem result có phải là null không (JSON null)
			resultStr := fmt.Sprintf("%v", response.Result)
			if resultStr == "<nil>" || resultStr == "null" || resultStr == "" {
				// Receipt chưa có, tiếp tục chờ
				time.Sleep(checkInterval)
				continue
			}

			// Parse receipt
			receiptBytes, err := json.Marshal(response.Result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal receipt: %w", err)
			}

			var receipt map[string]interface{}
			if err := json.Unmarshal(receiptBytes, &receipt); err != nil {
				return nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
			}

			// Kiểm tra xem receipt có hợp lệ không (có blockNumber)
			if blockNumber, ok := receipt["blockNumber"]; ok && blockNumber != nil {
				return receipt, nil
			}
			// Receipt không hợp lệ, tiếp tục chờ
			time.Sleep(checkInterval)
			continue
		}

		// Không có result, tiếp tục chờ
		time.Sleep(checkInterval)
	}

	elapsed := time.Since(startTime)
	return nil, fmt.Errorf("timeout (%v) waiting for receipt with txHash %s", elapsed, txHash)
}

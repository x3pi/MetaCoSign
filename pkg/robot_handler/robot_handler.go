package robothandler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	sessions       sync.Map // sessionId => *SessionData
	processedCount uint64
}

type QueuedTransaction struct {
	RobotAddress      ethCommon.Address
	RawTransactionHex string
	InitialNonce      uint64
	SessionID         uint64 // Deprecated: không dùng nữa, giữ lại để tương thích
	Method            string // Deprecated: không dùng nữa, giữ lại để tương thích
}

type SessionData struct {
	SessionID         uint64
	RobotAddress      ethCommon.Address
	Sentences         []string
	CreatedAt         time.Time
	MerkleRoot        []byte
	RawTransactionHex string // Lưu rawTransactionHex tạm vào RAM (không lưu DB)
	InitialNonce      uint64 // Lưu nonce ban đầu để rebuild transaction
	Method            string // Method name: "createSession" hoặc "emitSentence"
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
	case "dispatch":
		return h.handleDispatchImmediate(tx, method, inputData[4:], rawTransactionHex)

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

// handleDispatchImmediate: Xử lý dispatch ngay, đưa vào queue và broadcast event
func (h *RobotHandler) handleDispatchImmediate(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) (bool, interface{}, error) {
	logger.Info("🔵 [dispatch] Request received - txHash=%s, from=%s, to=%s",
		tx.Hash().Hex(), tx.FromAddress().Hex(), tx.ToAddress().Hex())

	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		logger.Error("❌ [dispatch] Failed to unpack input data: %v", err)
		return false, nil, fmt.Errorf("failed to unpack: %w", err)
	}

	if len(args) < 3 {
		logger.Error("❌ [dispatch] Invalid args length: %d", len(args))
		return false, nil, fmt.Errorf("invalid args length")
	}

	sessionId, ok := args[0].([32]byte)
	if !ok {
		logger.Error("❌ [dispatch] sessionId wrong type: %T", args[0])
		return false, nil, fmt.Errorf("sessionId type invalid")
	}

	actionId, ok := args[1].([32]byte)
	if !ok {
		logger.Error("❌ [dispatch] actionId wrong type: %T", args[1])
		return false, nil, fmt.Errorf("actionId type invalid")
	}

	data, ok := args[2].([]byte)
	if !ok {
		logger.Error("❌ [dispatch] data wrong type: %T", args[2])
		return false, nil, fmt.Errorf("data type invalid")
	}

	sessionIdHex := ethCommon.BytesToHash(sessionId[:]).Hex()
	actionIdHex := ethCommon.BytesToHash(actionId[:]).Hex()
	logger.Info("🔵 [dispatch] sessionId=%s, actionId=%s, dataLen=%d",
		sessionIdHex, actionIdHex, len(data))

	// Đưa transaction vào queue để xử lý tuần tự
	queuedTx := &QueuedTransaction{
		RobotAddress:      tx.FromAddress(),
		RawTransactionHex: rawTransactionHex,
		InitialNonce:      tx.GetNonce(),
	}
	select {
	case h.txQueue <- queuedTx:
		logger.Info("✅ Queued transaction for dispatch - sessionId=%s, actionId=%s", sessionIdHex, actionIdHex)
	default:
		logger.Error("❌ Queue is full, cannot queue transaction for dispatch - sessionId=%s", sessionIdHex)
	}
	operator := tx.FromAddress()
	h.broadcastEvent("EmitSentence", sessionId, actionId, operator, data)
	if h.appCtx.LdbRobotTransaction != nil {
		// Pack event data: sessionId, actionId, operator, data
		// Chỉ lưu data từ dispatch (args[2])
		if err := h.appCtx.LdbRobotTransaction.SaveTransaction(tx, rawTransactionHex, data); err != nil {
			logger.Error("❌ [dispatch] Failed to save transaction to storage: %v", err)
			// Không return error, chỉ log vì đây là non-critical operation
			return false, nil, fmt.Errorf("failed to save transaction to storage: %w", err)
		} else {
			logger.Debug("✅ [dispatch] Saved transaction to storage: txHash=%s", tx.Hash().Hex())
		}
	}
	txHash := tx.Hash().Hex()
	logger.Info("✅ [dispatch] Returning txHash=%s for sessionId=%s, actionId=%s",
		txHash, sessionIdHex, actionIdHex)

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
	storedData, err := h.appCtx.LdbRobotTransaction.GetTransactionByHash(txHashHex)
	if err != nil {
		logger.Error("❌ [getDataByTxhash] Failed to get transaction: %v", err)
		return nil, fmt.Errorf("transaction not found: %w", err)
	}
	// Tạo response object (không phải JSON string, để eth_call.go tự marshal)
	response := map[string]interface{}{
		"txHash":      storedData.TxHash,
		"rawTxHex":    storedData.RawTxHex,
		"fromAddress": ethCommon.BytesToAddress(storedData.FromAddress).Hex(),
		"toAddress":   ethCommon.BytesToAddress(storedData.ToAddress).Hex(),
		"createdAt":   storedData.CreatedAt,
		"events":      []map[string]interface{}{},
	}

	// Thêm events nếu có
	for _, event := range storedData.Events {
		eventData := map[string]interface{}{
			"data":      fmt.Sprintf("0x%x", event.Data),
			"createdAt": event.CreatedAt,
		}
		response["events"] = append(response["events"].([]map[string]interface{}), eventData)
	}

	logger.Info("✅ [getDataByTxhash] Returning data for txHash=%s", txHashHex)
	// Trả về map/object trực tiếp (eth_call.go sẽ tự marshal thành JSON hex string)
	return response, nil
}

// processTransactionQueue: Worker xử lý queue - lấy từ queue và gửi lên chain với nonce tuần tự
func (h *RobotHandler) processTransactionQueue() {
	// Map để quản lý nonce theo từng address
	nonceMap := make(map[ethCommon.Address]uint64)
	nonceMutex := sync.Mutex{}
	// Lấy từ queue và xử lý tuần tự
	for queuedTx := range h.txQueue {
		count := atomic.AddUint64(&h.processedCount, 1)
		fromAddress := queuedTx.RobotAddress

		// Decode rawTransactionHex
		decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(queuedTx.RawTransactionHex)
		if err != nil {
			logger.Error("Failed to decode rawTransactionHex: %v", err)
			continue
		}

		// Unmarshal Ethereum transaction
		ethTx := new(types.Transaction)
		if err := ethTx.UnmarshalBinary(decodedTxBytes); err != nil {
			releaseDecoded()
			logger.Error("Failed to unmarshal ethTx: %v", err)
			continue
		}

		// Lấy fromAddress từ transaction (sử dụng queuedTx.RobotAddress đã có sẵn)
		// fromAddress đã được set từ queuedTx.RobotAddress ở đầu vòng lặp

		// Lấy nonce tuần tự cho address này
		nonceMutex.Lock()
		currentNonce, exists := nonceMap[fromAddress]
		logger.Info("current nonce for %s: %v", fromAddress.Hex(), currentNonce)
		if !exists {
			logger.Info("init nonce for %s: %v", fromAddress.Hex(), queuedTx.InitialNonce)
			// Lần đầu tiên, sử dụng InitialNonce làm base
			currentNonce = queuedTx.InitialNonce
		}

		// Tăng nonce cho lần tiếp theo
		nonceMap[fromAddress] = currentNonce + 1
		nonceMutex.Unlock()
		// Rebuild transaction với nonce mới
		var (
			bTx       []byte
			releaseTx func()
			buildErr  error
			hasKey    bool
		)
		hasKey, err = h.appCtx.PKS.HasPrivateKey(fromAddress)
		if err != nil {
			releaseDecoded()
			logger.Error("Error checking private key for %s: %v", fromAddress.Hex(), err)
			continue
		}
		// Sử dụng transaction cũ (ethTx) và truyền nonce vào để cập nhật
		// Không tạo transaction mới vì transaction mới chưa được ký sẽ bị lỗi "invalid transaction v, r, s values"
		if !hasKey {
			bTx, _, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTx(
				ethTx,
				h.appCtx.TcpCfg,
				h.appCtx.Cfg,
				h.appCtx.LdbContractFreeGas,
				currentNonce, // Truyền nonce để cập nhật (nếu > 0 thì cập nhật, nếu = 0 thì dùng nonce từ account state)
			)
		} else {
			senderPkString, _ := h.appCtx.PKS.GetPrivateKey(fromAddress)
			keyPair := bls.NewKeyPair(ethCommon.FromHex(senderPkString))
			bTx, _, releaseTx, buildErr = h.appCtx.ClientRpc.BuildTransactionWithDeviceKeyFromEthTxAndBlsPrivateKey(
				ethTx,
				h.appCtx.TcpCfg,
				h.appCtx.Cfg,
				h.appCtx.LdbContractFreeGas,
				keyPair.PrivateKey(),
				currentNonce, // Truyền nonce để cập nhật (nếu > 0 thì cập nhật, nếu = 0 thì dùng nonce từ account state)
			)
		}
		releaseDecoded()
		if buildErr != nil {
			if releaseTx != nil {
				releaseTx()
			}
			logger.Error("❌ Failed to build transaction: %v", buildErr)
			// Rollback nonce nếu build failed
			nonceMutex.Lock()
			if nonceMap[fromAddress] > 0 {
				nonceMap[fromAddress]--
			}
			nonceMutex.Unlock()
			continue
		}
		// Gửi lên chain
		rs := h.appCtx.ClientRpc.SendRawTransactionBinary(bTx, releaseTx, nil, nil, nil)
		if rs.Error != nil {
			logger.Error("❌ Failed to send transaction: %v", rs.Error)
			// Rollback nonce nếu send failed
			nonceMutex.Lock()
			if nonceMap[fromAddress] > 0 {
				nonceMap[fromAddress]--
			}
			nonceMutex.Unlock()
		} else {
			logger.Info("error send transaction: %v", rs)
			txHash, ok := rs.Result.(string)
			if !ok {
				logger.Error("Failed to get transaction hash from result")
				// Rollback nonce
				nonceMutex.Lock()
				if nonceMap[fromAddress] > 0 {
					nonceMap[fromAddress]--
				}
				nonceMutex.Unlock()
				continue
			}
			logger.Info("✅ Sent robot tx to chain: nonce=%d, txHash=%s, count=%d",
				currentNonce, txHash, count)

			// Chờ receipt để đảm bảo nonce được cập nhật tuần tự
			receipt, err := h.waitForReceipt(txHash, 30*time.Second)
			if err != nil {
				logger.Error("❌ Failed to get receipt for txHash=%s: %v", txHash, err)
				// Rollback nonce nếu không có receipt
				nonceMutex.Lock()
				if nonceMap[fromAddress] > 0 {
					nonceMap[fromAddress]--
				}
				nonceMutex.Unlock()
				continue
			}
			logger.Info("✅ Received receipt for txHash=%s, blockNumber=%v, count=%d",
				txHash, receipt["blockNumber"], count)
		}

	}
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

// waitForReceipt chờ receipt từ RPC với timeout 30s
func (h *RobotHandler) waitForReceipt(txHash string, timeout time.Duration) (map[string]interface{}, error) {
	startTime := time.Now()
	checkInterval := 200 * time.Millisecond // Check mỗi 500ms
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
				elapsed := time.Since(startTime)
				logger.Info("✅ Receipt received for txHash=%s after %v, blockNumber=%v",
					txHash, elapsed, blockNumber)
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

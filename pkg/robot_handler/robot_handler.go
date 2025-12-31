package robothandler

import (
	"context"
	"encoding/json"
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
	sessions sync.Map // sessionId => *SessionData
}

type QueuedTransaction struct {
	SessionID         uint64
	RobotAddress      ethCommon.Address
	RawTransactionHex string
	InitialNonce      uint64
	Method            string
	CreatedAt         time.Time
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
	case "createSession":
		return h.handleCreateSessionImmediate(tx, method, inputData[4:], rawTransactionHex)
	case "emitSentence":
		return h.handleEmitSentenceImmediate(tx, method, inputData[4:], rawTransactionHex)
	default:
		return false, nil, fmt.Errorf("unknown method: %s", method.Name)
	}
}

// handleCreateSessionImmediate: Tạo session và trả về ngay, lưu rawTransactionHex vào RAM
func (h *RobotHandler) handleCreateSessionImmediate(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) (bool, interface{}, error) {
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return false, nil, fmt.Errorf("failed to unpack: %w", err)
	}
	logger.Info("handleCreateSessionImmediate: args=%v", args)

	// createSession có 3 tham số: sessionId (uint256), robotAddress (address), requestData (bytes)
	sessionIdBigInt := args[0].(*big.Int)
	sessionId := sessionIdBigInt.Uint64()
	robotAddress := args[1].(ethCommon.Address)
	requestData := args[2].([]byte)

	// Lưu session data tạm thời vào RAM (không lưu DB)
	sessionData := &SessionData{
		SessionID:         sessionId,
		RobotAddress:      robotAddress,
		Sentences:         make([]string, 0),
		CreatedAt:         time.Now(),
		RawTransactionHex: "",            // Sẽ được xóa sau khi đưa vào queue
		InitialNonce:      tx.GetNonce(), // Lưu nonce ban đầu
		Method:            "createSession",
	}
	h.sessions.Store(sessionId, sessionData)

	// Đưa transaction vào queue để xử lý tuần tự
	queuedTx := &QueuedTransaction{
		SessionID:         sessionId,
		RobotAddress:      robotAddress,
		RawTransactionHex: rawTransactionHex,
		InitialNonce:      tx.GetNonce(),
		Method:            "createSession",
		CreatedAt:         time.Now(),
	}
	select {
	case h.txQueue <- queuedTx:
		logger.Info("✅ Queued transaction for session %d (createSession)", sessionId)
	default:
		logger.Error("❌ Queue is full, cannot queue transaction for session %d", sessionId)
	}
	// Broadcast event ngay lập tức (không cần đợi chain) - dùng broadcastEvent chung
	h.broadcastEvent("SessionCreated", big.NewInt(int64(sessionId)), robotAddress, big.NewInt(time.Now().Unix()))
	h.broadcastEvent("AIRequest", big.NewInt(int64(sessionId)), requestData, big.NewInt(time.Now().Unix()))
	// Trả về kết quả NGAY LẬP TỨC
	return true, map[string]interface{}{
		"sessionId": fmt.Sprintf("0x%x", sessionId),
		"status":    "created",
	}, nil
}

// handleEmitSentenceImmediate: Emit sentence ngay, lưu rawTransactionHex và nonce vào RAM
func (h *RobotHandler) handleEmitSentenceImmediate(
	tx mt_types.Transaction,
	method *abi.Method,
	inputData []byte,
	rawTransactionHex string,
) (bool, interface{}, error) {
	logger.Info("🔵 [emitSentence] Request received - txHash=%s, from=%s, to=%s",
		tx.Hash().Hex(), tx.FromAddress().Hex(), tx.ToAddress().Hex())

	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		logger.Error("❌ [emitSentence] Failed to unpack input data: %v", err)
		return false, nil, fmt.Errorf("failed to unpack: %w", err)
	}

	if len(args) < 3 {
		logger.Error("❌ [emitSentence] Invalid args length: %d", len(args))
		return false, nil, fmt.Errorf("invalid args length")
	}

	sessionIDBig, ok := args[0].(*big.Int)
	if !ok {
		logger.Error("❌ [emitSentence] sessionID wrong type: %T", args[0])
		return false, nil, fmt.Errorf("sessionID type invalid")
	}

	sentenceIndexBig, ok := args[1].(*big.Int)
	if !ok {
		logger.Error("❌ [emitSentence] sentenceIndex wrong type: %T", args[1])
		return false, nil, fmt.Errorf("sentenceIndex type invalid")
	}

	sentence, ok := args[2].(string)
	if !ok {
		logger.Error("❌ [emitSentence] sentence wrong type: %T", args[2])
		return false, nil, fmt.Errorf("sentence type invalid")
	}

	sessionID := sessionIDBig.Uint64()
	sentenceIndex := sentenceIndexBig.Uint64()
	logger.Info("🔵🔵🔵🔵 [emitSentence] sentence=%d index=%d", sessionID, sentenceIndex)
	// Lấy session data
	val, ok := h.sessions.Load(sessionID)
	if !ok {
		// Nếu không tìm thấy session, tự tạo session default tạm thời
		logger.Warn("Session not found: %d, creating default temporary session", sessionID)
		fromAddress := tx.FromAddress()
		sessionData := &SessionData{
			SessionID:         sessionID,
			RobotAddress:      fromAddress,
			Sentences:         make([]string, 0),
			CreatedAt:         time.Now(),
			RawTransactionHex: "",
			InitialNonce:      tx.GetNonce(),
			Method:            "emitSentence",
		}
		h.sessions.Store(sessionID, sessionData)
		val = sessionData
		logger.Info("✅ Created default temporary session: sessionId=%d, robotAddress=%s", sessionID, fromAddress.Hex())
	}
	sessionData := val.(*SessionData)

	// Thêm sentence vào session
	sessionData.Sentences = append(sessionData.Sentences, sentence)

	// Đưa transaction vào queue để xử lý tuần tự
	queuedTx := &QueuedTransaction{
		SessionID:         sessionID,
		RobotAddress:      sessionData.RobotAddress,
		RawTransactionHex: rawTransactionHex,
		InitialNonce:      tx.GetNonce(),
		Method:            "emitSentence",
		CreatedAt:         time.Now(),
	}
	select {
	case h.txQueue <- queuedTx:
		logger.Info("✅ Queued transaction for session %d (emitSentence)", sessionID)
	default:
		logger.Error("❌ Queue is full, cannot queue transaction for session %d", sessionID)
	}

	// Broadcast event NGAY LẬP TỨC - dùng broadcastEvent chung
	h.broadcastEvent("SentenceEmitted", big.NewInt(int64(sessionID)), big.NewInt(int64(sentenceIndex)), sentence, big.NewInt(time.Now().Unix()))

	// Trả về transaction hash để viem có thể parse được (thay vì object)
	txHash := tx.Hash().Hex()
	logger.Info("✅ [emitSentence] Returning txHash=%s for sessionID=%d, sentenceIndex=%d",
		txHash, sessionID, sentenceIndex)

	return true, txHash, nil
}

// processTransactionQueue: Worker xử lý queue - lấy từ queue và gửi lên chain với nonce tuần tự
func (h *RobotHandler) processTransactionQueue() {
	// Map để quản lý nonce theo từng address
	nonceMap := make(map[ethCommon.Address]uint64)
	nonceMutex := sync.Mutex{}

	logger.Info("🚀 Robot transaction queue worker started")

	// Lấy từ queue và xử lý tuần tự
	for queuedTx := range h.txQueue {
		sessionID := queuedTx.SessionID
		fromAddress := queuedTx.RobotAddress

		// Lấy nonce tuần tự cho address này
		nonceMutex.Lock()
		currentNonce, exists := nonceMap[fromAddress]
		if !exists {
			// Lần đầu tiên, sử dụng InitialNonce làm base
			currentNonce = queuedTx.InitialNonce
		}
		// Tăng nonce cho lần tiếp theo
		nonceMap[fromAddress] = currentNonce + 1
		nonceMutex.Unlock()
		// Decode rawTransactionHex
		decodedTxBytes, releaseDecoded, err := utils.DecodeHexPooled(queuedTx.RawTransactionHex)
		if err != nil {
			logger.Error("Failed to decode rawTransactionHex for session %d: %v", sessionID, err)
			continue
		}

		// Unmarshal Ethereum transaction
		ethTx := new(types.Transaction)
		if err := ethTx.UnmarshalBinary(decodedTxBytes); err != nil {
			releaseDecoded()
			logger.Error("Failed to unmarshal ethTx for session %d: %v", sessionID, err)
			continue
		}
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
			logger.Error("Error checking private key for session %d: %v", sessionID, err)
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
			logger.Error("❌ ____Failed to build transaction for session %d: %v", sessionID, buildErr)
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
			logger.Error("❌ ______________Failed to send transaction for session %d: %v", sessionID, rs.Error)
			// Rollback nonce nếu send failed
			nonceMutex.Lock()
			if nonceMap[fromAddress] > 0 {
				nonceMap[fromAddress]--
			}
			nonceMutex.Unlock()
		} else {
			txHash, ok := rs.Result.(string)
			if !ok {
				logger.Error("Failed to get transaction hash from result for session %d", sessionID)
				// Rollback nonce
				nonceMutex.Lock()
				if nonceMap[fromAddress] > 0 {
					nonceMap[fromAddress]--
				}
				nonceMutex.Unlock()
				continue
			}
			logger.Info("✅ Sent robot tx to chain: session=%d, method=%s, nonce=%d, txHash=%s",
				sessionID, queuedTx.Method, currentNonce, txHash)

			// Chờ receipt để đảm bảo nonce được cập nhật tuần tự
			receipt, err := h.waitForReceipt(txHash, 30*time.Second)
			if err != nil {
				logger.Error("❌ Failed to get receipt for session %d, txHash=%s: %v", sessionID, txHash, err)
				// Rollback nonce nếu không có receipt
				nonceMutex.Lock()
				if nonceMap[fromAddress] > 0 {
					nonceMap[fromAddress]--
				}
				nonceMutex.Unlock()
				continue
			}
			logger.Info("✅ Received receipt for session %d, txHash=%s, blockNumber=%v",
				sessionID, txHash, receipt["blockNumber"])
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

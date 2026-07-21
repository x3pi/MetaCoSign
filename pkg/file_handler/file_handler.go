package file_handler

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	client_tcp "github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp"
	tcp_config "github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp/config"
	"github.com/meta-node-blockchain/meta-node/pkg/file_handler/abi_file"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"

	file_model "github.com/meta-node-blockchain/meta-node/pkg/models/file_model"
	"github.com/meta-node-blockchain/meta-node/pkg/quic_network"
	"github.com/meta-node-blockchain/meta-node/types"
	"github.com/quic-go/quic-go"
	"github.com/shirou/gopsutil/mem"
)

const MAX_SIZE_CHUNK = 1024 * 1024
const DEFAULT_MAX_MEMORY_USAGE_PERCENT = 99

const MAX_CHUNK = 8192 // 2GB
// Nó không còn chứa chainState nữa.
type FileHandlerNoReceipt struct {
	abi            abi.ABI
	comm           BlockchainCommunicator
	uploadProgress sync.Map
	fileInfoCache  sync.Map
	streamPool     sync.Map // fileKeyStr + "_isServer1" -> *quic_network.StreamContext
	connPool1      []quic.Connection
	connPool2      []quic.Connection
	//
	cachedRustServers []string   // Cache cho địa chỉ 2 server
	initMutex         sync.Mutex // (Để bảo vệ việc khởi tạo)
	isInitialized     bool       // (Cờ báo đã khởi tạo thành công)
	//
	pool1Mutex sync.RWMutex // Mutex cho pool 1 (Giữ nguyên)
	pool2Mutex sync.RWMutex // Mutex cho pool 2 (Giữ nguyên)
	//
	maxMemoryUsagePercent float64
}

var (
	tcpHandlerInstance *FileHandlerNoReceipt
	tcpOnce            sync.Once

	inProcessHandlerInstance *FileHandlerNoReceipt
	inProcessOnce            sync.Once
)

const (
	CONNECTION_POOL_SIZE = 30
	MAX_SEND_RETRIES     = 3
)

func createAndStartFileHandler(comm BlockchainCommunicator) (*FileHandlerNoReceipt, error) {
	var err error

	var parsedABI abi.ABI
	parsedABI, err = abi.JSON(strings.NewReader(abi_file.FileABI))
	if err != nil {
		return nil, err // Trả về lỗi ngay
	}
	// Tạo instance
	instance := &FileHandlerNoReceipt{
		abi:                   parsedABI,
		comm:                  comm,
		uploadProgress:        sync.Map{},
		fileInfoCache:         sync.Map{},
		streamPool:            sync.Map{},
		cachedRustServers:     make([]string, 2),
		pool1Mutex:            sync.RWMutex{},
		pool2Mutex:            sync.RWMutex{},
		maxMemoryUsagePercent: DEFAULT_MAX_MEMORY_USAGE_PERCENT,
	}

	go instance.waitForMemoryAvailability()
	go instance.monitorCacheHealth()

	if err != nil {
		return nil, err
	}
	return instance, nil
}

// sử lý trên tcp client
func GetFileHandlerTCP(c *client_tcp.Client, config *tcp_config.ClientConfig) (*FileHandlerNoReceipt, error) {
	var err error
	tcpOnce.Do(func() {
		comm := NewTCPCommunicator(c, config)
		// Gọi hàm tạo mới và gán vào instance TCP
		tcpHandlerInstance, err = createAndStartFileHandler(comm)
	})

	if err != nil {
		return nil, err // Trả về lỗi nếu khởi tạo thất bại
	}
	return tcpHandlerInstance, nil
}

func (h *FileHandlerNoReceipt) HandleFileTransactionNoReceipt(
	ctx context.Context,
	tx types.Transaction,
) (bool, error) {
	blockTime := uint64(time.Now().Unix())
	inputData := tx.CallData().Input()
	if len(inputData) < 4 {
		err := fmt.Errorf("FileHandler: Dữ liệu input không hợp lệ")
		return false, err
	}
	method, err := h.abi.MethodById(inputData[:4])
	if err != nil {
		// Bỏ qua các method không có trong ABI của file handler để EVM tự xử lý
		return false, nil
	}
	var logicErr error
	var isCall bool = false
	switch method.Name {
	case "uploadChunk":
		isCall = true
		if !h.isInitialized {
			h.initMutex.Lock()
			if !h.isInitialized {
				err := h.initializeServerCacheAndPools(tx)
				if err != nil {
					logger.Error("Lỗi khi khởi tạo cache server: %v", err)
				}
				h.isInitialized = true
			}
			h.initMutex.Unlock()
		}
		_, logicErr = h.HandleUploadChunk(tx, method, inputData[4:], blockTime)
	default:
		return false, nil
	}
	if logicErr != nil {
		return true, logicErr
	}
	if isCall {
		return true, nil
	}
	return false, nil
}
func (h *FileHandlerNoReceipt) monitorCacheHealth() {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		h.uploadProgress.Range(func(key, value interface{}) bool {
			fileKeyStr, ok := key.(string)
			if !ok {
				return true
			}
			progress, ok := value.(*file_model.FileUploadProgress)
			if !ok {
				return true
			}
			// --- GARBAGE COLLECTION ---
			// Tránh memory leak nếu người dùng bỏ dở upload hoặc mạng đứt hoàn toàn
			if time.Since(progress.StartTime) > 24*time.Hour {
				logger.Warn("🗑️ Xóa cache upload bị treo quá 24h: %s", fileKeyStr)
				h.uploadProgress.Delete(fileKeyStr)
				h.fileInfoCache.Delete(fileKeyStr)
			}
			return true
		})
	}
}

// handleUploadChunk: Chặn giao dịch, giải mã và gửi chunk đến Rust server.
func (h *FileHandlerNoReceipt) HandleUploadChunk(
	tx types.Transaction,
	method *abi.Method,
	inputData []byte,
	blockTime uint64,
) ([]types.EventLog, error) {
	// check
	// logger.Info("Bắt đầu xử lý uploadChunk cho tx %s", tx.Hash().Hex())
	// check
	// start := time.Now()
	args, err := method.Inputs.Unpack(inputData)
	if err != nil {
		return nil, fmt.Errorf("Lỗi khi unpack input data: %v", err)
	}
	fileKey, _ := args[0].([32]byte)
	chunkData, _ := args[1].([]byte)
	chunkIndex, _ := args[2].(*big.Int)
	merkleProofHashes, _ := args[3].([][32]byte)

	fileKeyStr := hex.EncodeToString(fileKey[:])
	chunkIndexInt := int(chunkIndex.Int64())

	// Log tiến độ upload để debug trên production (Chỉ log mỗi 50 chunk hoặc chunk đầu/cuối để tránh rác log)
	if chunkIndexInt == 0 || chunkIndexInt%50 == 0 {
		logger.Info("📥 [HandleUploadChunk] Đang xử lý file %s... (Chunk: %d)", fileKeyStr[:8], chunkIndexInt)
	}

	return h.doUploadChunkLogic(tx, fileKeyStr, fileKey, chunkIndexInt, chunkData, merkleProofHashes)
}

func (h *FileHandlerNoReceipt) doUploadChunkLogic(
	tx types.Transaction,
	fileKeyStr string,
	fileKey [32]byte,
	chunkIndexInt int,
	chunkData []byte,
	merkleProofHashes [][32]byte,
) ([]types.EventLog, error) {
	var err error
	var fileInfo *file_model.FileInfo
	val, found := h.fileInfoCache.Load(fileKeyStr)
	if !found {
		fileInfo, err = h.comm.GetFileInfo(fileKey, tx)
		if err != nil {
			return nil, fmt.Errorf("Lỗi khi tạo transaction getFileInfo: %v", err)
		}

		actualVal, loaded := h.fileInfoCache.LoadOrStore(fileKeyStr, fileInfo)
		if loaded {
			fileInfo = actualVal.(*file_model.FileInfo)
		}
	} else {
		fileInfo = val.(*file_model.FileInfo)
	}
	if fileInfo.TotalChunks.Cmp(big.NewInt(int64(MAX_CHUNK))) > 0 {
		return nil, fmt.Errorf("số chunk vượt quá giới hạn %d , yêu cầu < 2G", MAX_CHUNK)
	}
	if fileInfo.Status == 1 {
		h.uploadProgress.Delete(fileKeyStr)
		h.fileInfoCache.Delete(fileKeyStr)
		return nil, fmt.Errorf("file đã ở trạng thái Active, không thể upload thêm chunk %d filekey %s", chunkIndexInt, fileKeyStr)
	}
	if tx.FromAddress() != fileInfo.OwnerAddress {
		return nil, fmt.Errorf("chỉ chủ sở hữu file mới có thể upload chunk")
	}
	if len(chunkData) > MAX_SIZE_CHUNK {
		return nil, fmt.Errorf("kích thước chunk vượt quá giới hạn %d KB", MAX_SIZE_CHUNK/1024)
	}
	// startVerifyMerkle := time.Now()
	merkleRoot := fileInfo.MerkleRoot
// BỎ QUA MERKLE VERIFY Ở GO ĐỂ TĂNG TỐC ĐỘ. RUST SERVER SẼ ĐẢM NHẬN VIỆC NÀY.
	// fileTimeLogger.Info("%s Xác thực Merkle Proof (OK): %v", logPrefix, time.Since(startVerifyMerkle))
	// startSendChunk := time.Now()
	// --- THAY ĐỔI: Lấy Progress từ sync.Map ---
	var progress *file_model.FileUploadProgress
	val, found = h.uploadProgress.Load(fileKeyStr)
	if !found {
		newProgress := &file_model.FileUploadProgress{
			TotalChunks: fileInfo.TotalChunks,
			StartTime:   time.Now(),
		}
		actualVal, loaded := h.uploadProgress.LoadOrStore(fileKeyStr, newProgress)
		if loaded {
			progress = actualVal.(*file_model.FileUploadProgress) // <<< THAY ĐỔI
		} else {
			progress = newProgress
		}
	} else {
		progress = val.(*file_model.FileUploadProgress)
	}

	isServer1 := chunkIndexInt%2 == 0
	poolIndex := (chunkIndexInt / 2) % CONNECTION_POOL_SIZE

	// THÊM: Băm chunkIndex vào 1 trong 50 stream để cho phép gửi 50 chunk song song qua QUIC
	streamIndex := chunkIndexInt % 50
	streamKey := fmt.Sprintf("%s_%v_%d", fileKeyStr, isServer1, streamIndex)

	var streamCtx *quic_network.StreamContext
	valStream, foundStream := h.streamPool.Load(streamKey)
	if !foundStream {
		conn, errConn := h.getAndRenewConn(isServer1, poolIndex, fileKeyStr, chunkIndexInt)
		if errConn != nil {
			return nil, fmt.Errorf("không thể lấy/tạo kết nối cho chunk %d: %v", chunkIndexInt, errConn)
		}
		stream, errStream := conn.OpenStreamSync(context.Background())
		if errStream != nil {
			return nil, fmt.Errorf("không thể mở stream: %v", errStream)
		}
		newStreamCtx := &quic_network.StreamContext{Stream: stream}
		actualVal, loaded := h.streamPool.LoadOrStore(streamKey, newStreamCtx)
		if loaded {
			stream.Close()
			streamCtx = actualVal.(*quic_network.StreamContext)
		} else {
			streamCtx = newStreamCtx
		}
	} else {
		streamCtx = valStream.(*quic_network.StreamContext)
	}

	status, err := h.sendChunk(streamCtx, isServer1, poolIndex, fileKeyStr, chunkIndexInt, chunkData, fileInfo.Signature, merkleProofHashes, merkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to send chunk %d: %v", chunkIndexInt, err)
	}

	if status == "COMPLETED" {
		completedCount := progress.CompletedServers.Add(1)

		threshold := uint32(2)
		if progress.TotalChunks.Cmp(big.NewInt(1)) == 0 {
			threshold = 1
		}

		if completedCount >= threshold {
			// Dọn dẹp cache RAM ngay lập tức ở Go Client,
			// phần Confirm giao dịch giờ đã do Server Rust đảm nhiệm!
			h.uploadProgress.Delete(fileKeyStr)
			h.fileInfoCache.Delete(fileKeyStr)
			logger.Info("✅ Nhận đủ %d COMPLETED confirm từ Rust Server. Đã xóa cache cho file %s!", threshold, fileKeyStr)
			// Cleanup streams
			for i := 0; i < 50; i++ {
				key1 := fmt.Sprintf("%s_true_%d", fileKeyStr, i)
				if sCtx, ok := h.streamPool.Load(key1); ok {
					sCtx.(*quic_network.StreamContext).Stream.Close()
					h.streamPool.Delete(key1)
				}
				key2 := fmt.Sprintf("%s_false_%d", fileKeyStr, i)
				if sCtx, ok := h.streamPool.Load(key2); ok {
					sCtx.(*quic_network.StreamContext).Stream.Close()
					h.streamPool.Delete(key2)
				}
			}
		}
	}
	return nil, nil
}

func (h *FileHandlerNoReceipt) sendChunk(
	streamCtx *quic_network.StreamContext,
	isServer1 bool,
	poolIndex int,
	fileKey string,
	chunkIndex int,
	chunkData []byte,
	signature string,
	merkleProofHashes [][32]byte,
	merkleRoot [32]byte,
) (string, error) {
	var lastErr error
	var status string
	for i := 0; i < MAX_SEND_RETRIES; i++ {
		status, lastErr = quic_network.SendChunkToRustServerQuic(streamCtx, fileKey, chunkIndex, chunkData, signature, merkleProofHashes, merkleRoot)
		if lastErr == nil {
			return status, nil
		}
		if errors.Is(lastErr, context.DeadlineExceeded) || strings.Contains(lastErr.Error(), "deadline exceeded") {
			logger.Warn("[file: %s, chunk: %d] Stream timeout, sẽ thử stream mới...", fileKey, chunkIndex)
			// Không continue ngay, tiếp tục xuống dưới để renew kết nối và stream
		} else {
			logger.Error("[file: %s, chunk: %d] Lỗi gửi chunk (lần thử %d/%d): %v. Đang lấy kết nối mới...", fileKey, chunkIndex, i+1, MAX_SEND_RETRIES, lastErr)
		}

		newConn, reconErr := h.getAndRenewConn(isServer1, poolIndex, fileKey, chunkIndex)
		if reconErr == nil {
			newStream, errStream := newConn.OpenStreamSync(context.Background())
			if errStream == nil {
				streamCtx.Mu.Lock()
				if streamCtx.Stream != nil {
					streamCtx.Stream.Close()
				}
				streamCtx.Stream = newStream
				streamCtx.Mu.Unlock()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.isInitialized = false
	return "", fmt.Errorf("❌❌ [file: %s, chunk: %d] không thể gửi chunk sau %d lần thử: %v",
		fileKey, chunkIndex, MAX_SEND_RETRIES, lastErr)
}

func (h *FileHandlerNoReceipt) getAndRenewConn(isServer1 bool, poolIndex int, fileKeyStr string, chunkIndexInt int) (quic.Connection, error) {
	var pool []quic.Connection
	var addr string
	var serverName string
	var mtx *sync.RWMutex // Trỏ tới mutex đúng

	if isServer1 {
		pool = h.connPool1
		addr = h.cachedRustServers[0]
		serverName = "Server 1"
		mtx = &h.pool1Mutex // Dùng mutex 1
	} else {
		pool = h.connPool2
		addr = h.cachedRustServers[1]
		serverName = "Server 2"
		mtx = &h.pool2Mutex // Dùng mutex 2
	}

	// --- 1. FAST-PATH ---
	conn := pool[poolIndex]
	if conn != nil && conn.Context().Err() == nil {
		return conn, nil
	}

	// --- 2. SLOW-PATH ---
	mtx.Lock()
	defer mtx.Unlock()
	conn = pool[poolIndex]
	if conn != nil && conn.Context().Err() == nil {
		return conn, nil
	}

	// Đóng connection cũ (nếu có) để giải phóng UDP socket, tránh lỗi "address already in use"
	if conn != nil {
		conn.CloseWithError(0, "renew connection")
	}

	newConn, err := quic_network.CreateQuicConnection(addr)
	if err != nil {
		logger.Error("[file: %s, chunk: %d] [ConnPool] Lỗi khi tái kết nối (%s, Index %d): %v", fileKeyStr, chunkIndexInt, serverName, poolIndex, err) // <<< LOG
		return nil, err
	}

	pool[poolIndex] = newConn
	return newConn, nil
}

func (h *FileHandlerNoReceipt) waitForMemoryAvailability() error {
	if h.maxMemoryUsagePercent <= 0 {
		return nil
	}
	const (
		gcAttemptThreshold   = 20
		maxWaitAttempts      = 50
		waitIntervalDuration = 200 * time.Millisecond
	)
	logged := false
	attempts := 0
	for {
		if h.isMemoryUsageWithinLimit() {
			return nil
		}
		if !logged {
			logger.Warn("Đang tạm dừng upload chunk do RAM hệ thống đã vượt %.2f%%", h.maxMemoryUsagePercent)
			logged = true
		}
		attempts++
		if attempts == gcAttemptThreshold {
			logger.Warn("RAM vẫn cao sau %d lần kiểm tra, thực hiện runtime.GC() và debug.FreeOSMemory()", attempts)
			runtime.GC()
			debug.FreeOSMemory()
		}
		if attempts >= maxWaitAttempts {
			return fmt.Errorf("RAM vẫn vượt ngưỡng %.2f%% sau khi chờ %d lần", h.maxMemoryUsagePercent, attempts)
		}
		time.Sleep(waitIntervalDuration)
	}
}

func (h *FileHandlerNoReceipt) isMemoryUsageWithinLimit() bool {
	if h.maxMemoryUsagePercent <= 0 {
		return true
	}
	vmem, err := mem.VirtualMemory()
	if err != nil {
		logger.Error("Không thể đọc thông tin RAM hệ thống: %v", err)
		return true
	}
	return vmem.UsedPercent < h.maxMemoryUsagePercent
}

func (h *FileHandlerNoReceipt) initializeServerCacheAndPools(tx types.Transaction) error {
	servers, err := h.comm.GetRustServerAddresses(tx)
	if err != nil {
		return fmt.Errorf("lỗi tạo tx getList: %v", err)
	}
	if len(servers) < 2 {
		return fmt.Errorf("lỗi: Contract trả về ít hơn 2 server (có %d)", len(servers))
	}

	h.cachedRustServers[0] = servers[0]
	h.cachedRustServers[1] = servers[1]

	// Đóng các connection cũ nếu đã tồn tại để tránh rò rỉ UDP port
	if h.connPool1 != nil {
		for _, c := range h.connPool1 {
			if c != nil {
				c.CloseWithError(0, "re-init pool")
			}
		}
	}
	if h.connPool2 != nil {
		for _, c := range h.connPool2 {
			if c != nil {
				c.CloseWithError(0, "re-init pool")
			}
		}
	}

	h.connPool1 = make([]quic.Connection, CONNECTION_POOL_SIZE)
	h.connPool2 = make([]quic.Connection, CONNECTION_POOL_SIZE)
	var wg sync.WaitGroup
	wg.Add(CONNECTION_POOL_SIZE * 2)
	var connErr error
	for i := 0; i < CONNECTION_POOL_SIZE; i++ {
		go func(idx int) { // Server 1
			defer wg.Done()
			conn, err := quic_network.CreateQuicConnection(h.cachedRustServers[0])
			if err != nil {
				logger.Error("[Init] Failed to create initial QUIC connection to server 1 (index %d): %v", idx, err) // <<< LOG
				if connErr == nil {
					connErr = err
				}
			}
			h.connPool1[idx] = conn
		}(i)

		go func(idx int) { // Server 2
			defer wg.Done()
			conn, err := quic_network.CreateQuicConnection(h.cachedRustServers[1])
			if err != nil {
				logger.Error("[Init] Failed to create initial QUIC connection to server 2 (index %d): %v", idx, err) // <<< LOG
				if connErr == nil {
					connErr = err
				}
			}
			h.connPool2[idx] = conn
		}(i)
	}
	wg.Wait()
	if connErr != nil {
		return fmt.Errorf("lỗi khi tạo connection pools: %v", connErr)
	}
	logger.Info("[Init] FileHandler: Khởi tạo server cache và connection pools thành công. %v", h.cachedRustServers) // <<< LOG
	return nil
}

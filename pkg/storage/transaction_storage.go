package storage

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/syndtr/goleveldb/leveldb"
	"google.golang.org/protobuf/proto"
)

const (
	ERROR_STORAGE_PREFIX = "error:" // error:<txHash> → StoredErrorData
)

// CachedErrorData lưu error data trong cache
type CachedErrorData struct {
	Data        *pb.StoredErrorData
	CachedAt    time.Time
	AccessCount int64 // Số lần truy cập để LRU
}

// RobotTransaction quản lý lưu trữ error data
type RobotTransaction struct {
	db           *leveldb.DB
	cache        sync.Map // txHash (string) → *CachedErrorData
	cacheSize    int      // Số lượng items trong cache
	maxCacheSize int      // Giới hạn cache (default 500)
	mu           sync.RWMutex

	// Batch write channel để async write
	writeChan    chan *writeRequest
	batchSize    int           // Số lượng write trong một batch
	batchTimeout time.Duration // Timeout để flush batch
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

type writeRequest struct {
	txHash string
	data   *pb.StoredErrorData
}

// NewTransactionStorage tạo mới RobotTransaction storage
func NewTransactionStorage(db *leveldb.DB) *RobotTransaction {
	ts := &RobotTransaction{
		db:           db,
		maxCacheSize: 500,
		batchSize:    50,                             // Batch 50 errors
		batchTimeout: 500 * time.Millisecond,         // Flush sau 500ms
		writeChan:    make(chan *writeRequest, 1000), // Buffer 1000 requests
		stopChan:     make(chan struct{}),
	}
	ts.wg.Add(1)
	go ts.batchWriter()

	return ts
}

// SaveError lưu error data (async via channel)
func (ts *RobotTransaction) SaveError(
	txHash string,
	inputData string, // Input data serialized as JSON string
	errorMessage string,
) error {
	// Tạo stored error data
	storedData := &pb.StoredErrorData{
		TxHash:       txHash,
		InputData:    inputData,
		ErrorMessage: errorMessage,
		CreatedAt:    time.Now().Unix(),
	}

	// Update cache ngay lập tức
	ts.updateCache(txHash, storedData)

	// Gửi vào channel để batch write
	select {
	case ts.writeChan <- &writeRequest{txHash: txHash, data: storedData}:
		// Success
	default:
		// Channel đầy, log warning nhưng không block
		logger.Warn("⚠️ Error write channel full, dropping txHash=%s", txHash)
	}

	return nil
}

// batchWriter xử lý batch write async
func (ts *RobotTransaction) batchWriter() {
	defer ts.wg.Done()
	batch := new(leveldb.Batch)
	pending := make(map[string]*pb.StoredErrorData)
	ticker := time.NewTicker(ts.batchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(pending) == 0 {
			return
		}

		batch.Reset()
		for txHash, data := range pending {
			key := []byte(fmt.Sprintf("%s%s", ERROR_STORAGE_PREFIX, txHash))
			dataBytes, err := proto.Marshal(data)
			if err != nil {
				logger.Error("❌ Failed to marshal error data for txHash=%s: %v", txHash, err)
				continue
			}
			batch.Put(key, dataBytes)
		}

		if err := ts.db.Write(batch, nil); err != nil {
			logger.Error("❌ Failed to write error batch: %v", err)
		} else {
			logger.Debug("✅ Flushed %d errors to DB", len(pending))
		}

		// Clear pending
		for k := range pending {
			delete(pending, k)
		}
	}

	for {
		select {
		case req := <-ts.writeChan:
			if req == nil {
				continue
			}
			pending[req.txHash] = req.data

			// Flush nếu đủ batch size
			if len(pending) >= ts.batchSize {
				flush()
			}

		case <-ticker.C:
			// Flush theo timeout
			flush()

		case <-ts.stopChan:
			// Flush tất cả trước khi stop
			flush()
			return
		}
	}
}

// GetErrorByHash lấy error data theo txHash (check cache trước)
func (ts *RobotTransaction) GetErrorByHash(txHashHex string) (*pb.StoredErrorData, error) {
	// 1. Check cache trước
	if cached, ok := ts.cache.Load(txHashHex); ok {
		cachedData := cached.(*CachedErrorData)
		// Update access count
		atomic.AddInt64(&cachedData.AccessCount, 1)
		return cachedData.Data, nil
	}

	// 2. Load từ DB
	key := []byte(fmt.Sprintf("%s%s", ERROR_STORAGE_PREFIX, txHashHex))
	dataBytes, err := ts.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("error not found: %s", txHashHex)
		}
		return nil, fmt.Errorf("failed to get error: %w", err)
	}

	var storedData pb.StoredErrorData
	if err := proto.Unmarshal(dataBytes, &storedData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal error data: %w", err)
	}

	// 3. Update cache
	ts.updateCache(txHashHex, &storedData)

	return &storedData, nil
}

// updateCache cập nhật cache
func (ts *RobotTransaction) updateCache(txHash string, data *pb.StoredErrorData) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Kiểm tra xem đã có trong cache chưa
	if _, exists := ts.cache.Load(txHash); !exists {
		// Nếu cache đầy, xóa item cũ nhất
		if ts.cacheSize >= ts.maxCacheSize {
			ts.evictLRU()
		}
		ts.cacheSize++
	}

	// Update cache
	ts.cache.Store(txHash, &CachedErrorData{
		Data:        data,
		CachedAt:    time.Now(),
		AccessCount: 0,
	})
}

// evictLRU xóa item ít được truy cập nhất
func (ts *RobotTransaction) evictLRU() {
	var bestKey string
	var minCount int64 = 1<<63 - 1

	samples := 0
	maxSamples := 10 // Chỉ kiểm tra 10 phần tử ngẫu nhiên

	ts.cache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedErrorData)
		count := atomic.LoadInt64(&cached.AccessCount)

		if count < minCount {
			minCount = count
			bestKey = key.(string)
		}

		samples++
		return samples < maxSamples // Dừng lại khi đủ mẫu
	})

	if bestKey != "" {
		ts.cache.Delete(bestKey)
		ts.cacheSize--
	}
}

// Close đóng storage và flush tất cả pending writes
func (ts *RobotTransaction) Close() error {
	close(ts.stopChan)
	ts.wg.Wait()
	close(ts.writeChan)
	return nil
}

// GetCacheStats trả về thống kê cache
func (ts *RobotTransaction) GetCacheStats() (size int, maxSize int) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.cacheSize, ts.maxCacheSize
}

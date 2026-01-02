package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	mt_types "github.com/meta-node-blockchain/meta-node/types"
	"github.com/syndtr/goleveldb/leveldb"
	"google.golang.org/protobuf/proto"
)

const (
	TX_STORAGE_PREFIX = "tx:" // tx:<txHash> → StoredTransactionData
)

// CachedTxData lưu transaction trong cache
type CachedTxData struct {
	Data        *pb.StoredTransactionData
	CachedAt    time.Time
	AccessCount int // Số lần truy cập để LRU
}

// TransactionStorage quản lý lưu trữ transaction và events
type RobotTransaction struct {
	db           *leveldb.DB
	cache        sync.Map // txHash (string) → *CachedTxData
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
	data   *pb.StoredTransactionData
}

// NewTransactionStorage tạo mới TransactionStorage
func NewTransactionStorage(db *leveldb.DB) *RobotTransaction {
	ts := &RobotTransaction{
		db:           db,
		maxCacheSize: 500,
		batchSize:    50,                             // Batch 50 transactions
		batchTimeout: 500 * time.Millisecond,         // Flush sau 100ms
		writeChan:    make(chan *writeRequest, 1000), // Buffer 1000 requests
		stopChan:     make(chan struct{}),
	}
	// Khởi động batch writer
	ts.wg.Add(1)
	go ts.batchWriter()

	return ts
}

// SaveTransaction lưu transaction và events (async via channel)
func (ts *RobotTransaction) SaveTransaction(
	tx mt_types.Transaction,
	rawTxHex string,
	eventData []byte, // Event data từ dispatch
) error {
	txHash := tx.Hash().Hex()

	// Marshal transaction
	txBytes, err := tx.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// Tạo stored data
	storedData := &pb.StoredTransactionData{
		TxHash:      txHash,
		TxBytes:     txBytes,
		RawTxHex:    rawTxHex,
		FromAddress: tx.FromAddress().Bytes(),
		ToAddress:   tx.ToAddress().Bytes(),
		CreatedAt:   time.Now().Unix(),
	}
	// Thêm event data nếu có
	if len(eventData) > 0 {
		storedData.Events = []*pb.StoredEventData{
			{
				Data:      eventData,
				CreatedAt: time.Now().Unix(),
			},
		}
	}
	// Update cache ngay lập tức
	ts.updateCache(txHash, storedData)
	// Gửi vào channel để batch write
	select {
	case ts.writeChan <- &writeRequest{txHash: txHash, data: storedData}:
		// Success
	default:
		// Channel đầy, log warning nhưng không block
		logger.Warn("⚠️ Transaction write channel full, dropping txHash=%s", txHash)
	}

	return nil
}

// batchWriter xử lý batch write async
func (ts *RobotTransaction) batchWriter() {
	defer ts.wg.Done()

	batch := new(leveldb.Batch)
	pending := make(map[string]*pb.StoredTransactionData)
	ticker := time.NewTicker(ts.batchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(pending) == 0 {
			return
		}

		batch.Reset()
		for txHash, data := range pending {
			key := []byte(fmt.Sprintf("%s%s", TX_STORAGE_PREFIX, txHash))
			dataBytes, err := proto.Marshal(data)
			if err != nil {
				logger.Error("❌ Failed to marshal transaction data for txHash=%s: %v", txHash, err)
				continue
			}
			batch.Put(key, dataBytes)
		}

		if err := ts.db.Write(batch, nil); err != nil {
			logger.Error("❌ Failed to write transaction batch: %v", err)
		} else {
			logger.Debug("✅ Flushed %d transactions to DB", len(pending))
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

// GetTransactionByHash lấy transaction theo hash (check cache trước)
func (ts *RobotTransaction) GetTransactionByHash(txHashHex string) (*pb.StoredTransactionData, error) {
	// 1. Check cache trước
	if cached, ok := ts.cache.Load(txHashHex); ok {
		cachedData := cached.(*CachedTxData)
		// Update access count
		cachedData.AccessCount++
		return cachedData.Data, nil
	}

	// 2. Load từ DB
	key := []byte(fmt.Sprintf("%s%s", TX_STORAGE_PREFIX, txHashHex))
	dataBytes, err := ts.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("transaction not found: %s", txHashHex)
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	var storedData pb.StoredTransactionData
	if err := proto.Unmarshal(dataBytes, &storedData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction data: %w", err)
	}

	// 3. Update cache
	ts.updateCache(txHashHex, &storedData)

	return &storedData, nil
}

// updateCache cập nhật cache
func (ts *RobotTransaction) updateCache(txHash string, data *pb.StoredTransactionData) {
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
	ts.cache.Store(txHash, &CachedTxData{
		Data:        data,
		CachedAt:    time.Now(),
		AccessCount: 0,
	})
}

// evictLRU xóa item ít được truy cập nhất
func (ts *RobotTransaction) evictLRU() {
	var oldestKey string
	var oldestAccessCount int = -1

	ts.cache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedTxData)
		if oldestAccessCount == -1 || cached.AccessCount < oldestAccessCount {
			oldestAccessCount = cached.AccessCount
			oldestKey = key.(string)
		}
		return true
	})

	if oldestKey != "" {
		ts.cache.Delete(oldestKey)
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

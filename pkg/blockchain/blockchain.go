package blockchain

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/account_state_db"
	"github.com/meta-node-blockchain/meta-node/pkg/block"
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	"github.com/meta-node-blockchain/meta-node/pkg/trie"
	mtn_types "github.com/meta-node-blockchain/meta-node/types"
)

const (
	blockNumberPrefix       = "blockNumber_"
	txHashPrefix            = "txHashPrefix"            // Tiền tố cho key
	ethHashMapBlsHashPrefix = "ethHashMapBlsHashPrefix" // Tiền tố cho key

)

var (
	blockChainInstance *BlockChain
	once               sync.Once
	storeLimiter       = make(chan struct{}, 100) // Giới hạn 100 concurrent operations

)

// BlockChain quản lý bộ nhớ đệm cho các khối, biên nhận và giao dịch.
// Tất cả các cache đã được chuyển sang sync.Map để đảm bảo an toàn khi truy cập đồng thời.
type BlockChain struct {
	blockCache             *sync.Map // Key: common.Hash, Value: cachedBlock
	receiptsCache          *sync.Map // Key: common.Hash, Value: []types.Receipt
	txsCache               *sync.Map // Key: common.Hash, Value: cachedTx
	blockNumberToHashCache *sync.Map // Key: uint64,      Value: cachedHash
	txHashToBlockNumber    *sync.Map // Key: common.Hash, Value: cachedUint64
	ethHashMapBlsHash      *sync.Map // Key: common.Hash, Value: cachedHash

	blockDatabase  *block.BlockDatabase
	storageManager *storage.StorageManager

	dirtyStorage sync.Map
	mappingBatch []byte
}

// cachedTx giữ raw transaction và thời điểm thêm vào cache để có thể dọn theo TTL.
type cachedTx struct {
	raw     []byte
	addedAt time.Time
}

type cachedBlock struct {
	block   mtn_types.Block
	addedAt time.Time
}

type cachedHash struct {
	hash    common.Hash
	addedAt time.Time
}

type cachedUint64 struct {
	value   uint64
	addedAt time.Time
}

func (bc *BlockChain) SetMappingBatch(batch []byte) {
	bc.mappingBatch = batch
}

func (bc *BlockChain) GetMappingBatch() []byte {
	batch := bc.mappingBatch
	bc.mappingBatch = nil
	return batch
}

// InitBlockChain khởi tạo instance duy nhất của BlockChain sử dụng sync.Map.
// Hàm này chỉ được gọi MỘT LẦN duy nhất trong suốt vòng đời của ứng dụng.
func InitBlockChain(size int, blockDatabase *block.BlockDatabase, storageManager *storage.StorageManager) {
	once.Do(func() {
		blockChainInstance = &BlockChain{
			blockCache:             new(sync.Map),
			receiptsCache:          new(sync.Map),
			txsCache:               new(sync.Map),
			blockNumberToHashCache: new(sync.Map),
			txHashToBlockNumber:    new(sync.Map),
			ethHashMapBlsHash:      new(sync.Map),
			blockDatabase:          blockDatabase,
			storageManager:         storageManager,
		}
		log.Println("BlockChain instance initialized successfully with sync.Map")
	})
}

const (
	txCacheTTL      = 2 * time.Minute
	blockCacheTTL   = 10 * time.Minute
	mappingCacheTTL = 30 * time.Minute
)

// AddTxToCache lưu raw transaction vào cache và dọn những mục đã quá hạn.
func (bc *BlockChain) AddTxToCache(txHash common.Hash, rawTx []byte) {
	if bc.txsCache == nil {
		logger.Error("txsCache is not initialized, cannot add transaction")
		return
	}
	snapshot := append([]byte(nil), rawTx...)
	bc.txsCache.Store(txHash, cachedTx{
		raw:     snapshot,
		addedAt: time.Now(),
	})
	logger.Debug("Stored transaction in txsCache:", txHash.Hex())
	bc.pruneTxCache(time.Now().Add(-txCacheTTL))
}

// GetTxFromCache trả về raw transaction bytes nếu còn hiệu lực trong cache.
func (bc *BlockChain) GetTxFromCache(txHash common.Hash) ([]byte, bool) {
	if bc.txsCache == nil {
		logger.Error("txsCache is not initialized, cannot get transaction")
		return nil, false
	}
	value, ok := bc.txsCache.Load(txHash)
	if !ok {
		logger.Debug("Transaction not found in txsCache:", txHash.Hex())
		return nil, false
	}

	cached, ok := value.(cachedTx)
	if !ok {
		logger.Error("Invalid type in txsCache for hash:", txHash.Hex())
		bc.txsCache.Delete(txHash)
		return nil, false
	}

	if time.Since(cached.addedAt) > txCacheTTL {
		bc.txsCache.Delete(txHash)
		return nil, false
	}

	logger.Debug("Retrieved transaction from txsCache:", txHash.Hex())
	return append([]byte(nil), cached.raw...), true
}

// pruneTxCache loại bỏ các giao dịch đã quá hạn khỏi cache.
func (bc *BlockChain) pruneTxCache(expireBefore time.Time) {
	if bc.txsCache == nil {
		return
	}

	bc.txsCache.Range(func(key, value any) bool {
		cached, ok := value.(cachedTx)
		if !ok || cached.addedAt.Before(expireBefore) {
			bc.txsCache.Delete(key)
		}
		return true
	})
}

func (bc *BlockChain) pruneBlockCache(expireBefore time.Time) {
	if bc.blockCache == nil {
		return
	}

	bc.blockCache.Range(func(key, value any) bool {
		cached, ok := value.(cachedBlock)
		if !ok || cached.addedAt.Before(expireBefore) {
			bc.blockCache.Delete(key)
		}
		return true
	})
}

func (bc *BlockChain) pruneBlockNumberCache(expireBefore time.Time) {
	if bc.blockNumberToHashCache == nil {
		return
	}

	bc.blockNumberToHashCache.Range(func(key, value any) bool {
		cached, ok := value.(cachedHash)
		if !ok || cached.addedAt.Before(expireBefore) {
			bc.blockNumberToHashCache.Delete(key)
		}
		return true
	})
}

func (bc *BlockChain) pruneTxHashCache(expireBefore time.Time) {
	if bc.txHashToBlockNumber == nil {
		return
	}

	bc.txHashToBlockNumber.Range(func(key, value any) bool {
		cached, ok := value.(cachedUint64)
		if !ok || cached.addedAt.Before(expireBefore) {
			bc.txHashToBlockNumber.Delete(key)
		}
		return true
	})
}

func (bc *BlockChain) pruneEthHashCache(expireBefore time.Time) {
	if bc.ethHashMapBlsHash == nil {
		return
	}

	bc.ethHashMapBlsHash.Range(func(key, value any) bool {
		cached, ok := value.(cachedHash)
		if !ok || cached.addedAt.Before(expireBefore) {
			bc.ethHashMapBlsHash.Delete(key)
		}
		return true
	})
}

// GetBlockChainInstance trả về instance đã được khởi tạo của BlockChain.
func GetBlockChainInstance() *BlockChain {
	if blockChainInstance == nil {
		log.Fatal("BlockChain instance has not been initialized. Call InitBlockChain() first.")
	}
	return blockChainInstance
}

// GetBlock lấy một block, ưu tiên từ cache, nếu không có sẽ đọc từ DB và lưu vào cache.
func (bc *BlockChain) GetBlock(hash common.Hash) mtn_types.Block {
	// 1. Kiểm tra cache trước
	if value, ok := bc.blockCache.Load(hash); ok {
		if cached, ok := value.(cachedBlock); ok {
			if time.Since(cached.addedAt) <= blockCacheTTL {
				return cached.block
			}
			bc.blockCache.Delete(hash)
		}
	}

	// 2. Nếu không có trong cache, đọc từ DB
	block, err := bc.blockDatabase.GetBlockByHash(hash)
	if err != nil {
		return nil // Trả về nil nếu không tìm thấy hoặc có lỗi
	}

	// 3. Lưu vào cache cho lần truy cập sau
	bc.blockCache.Store(hash, cachedBlock{
		block:   block,
		addedAt: time.Now(),
	})
	bc.pruneBlockCache(time.Now().Add(-blockCacheTTL))
	return block
}

func (bc *BlockChain) GetBlockByNumber(number uint64) mtn_types.Block {
	hash, ok := bc.GetBlockHashByNumber(number)
	if !ok {
		return nil
	}
	return bc.GetBlock(hash)
}

func (bc *BlockChain) GetLastBlock() mtn_types.Block {
	block, err := bc.blockDatabase.GetLastBlock()
	if err != nil {
		return nil
	}
	return block
}

func (bc *BlockChain) NewAccountStateDBFromBlock(blockHeader mtn_types.BlockHeader) (*account_state_db.AccountStateDB, error) {
	accountStateTrie, err := trie.New(
		blockHeader.AccountStatesRoot(),
		bc.storageManager.GetStorageAccount(),
		true,
	)
	if err != nil {
		return nil, err
	}
	accountStateDB := account_state_db.NewAccountStateDB(
		accountStateTrie,
		bc.storageManager.GetStorageAccount())
	return accountStateDB, nil
}

// SetBlockNumberToHash lưu ánh xạ từ block number sang block hash vào dirty storage và cache.
func (bc *BlockChain) SetBlockNumberToHash(blockNumber uint64, blockHash common.Hash) error {
	key := fmt.Sprintf("%s%d", blockNumberPrefix, blockNumber)
	bc.dirtyStorage.Store(key, blockHash.Bytes())
	bc.blockNumberToHashCache.Store(blockNumber, cachedHash{
		hash:    blockHash,
		addedAt: time.Now(),
	})
	bc.pruneBlockNumberCache(time.Now().Add(-mappingCacheTTL))
	return nil
}

// GetBlockHashByNumber lấy block hash từ block number, ưu tiên từ cache.
func (bc *BlockChain) GetBlockHashByNumber(blockNumber uint64) (common.Hash, bool) {
	// 1. Kiểm tra cache trước
	if value, ok := bc.blockNumberToHashCache.Load(blockNumber); ok {
		if cached, ok := value.(cachedHash); ok {
			if time.Since(cached.addedAt) <= mappingCacheTTL {
				return cached.hash, true
			}
			bc.blockNumberToHashCache.Delete(blockNumber)
		}
	}

	// 2. Nếu không có trong cache, truy vấn từ DB
	key := []byte(fmt.Sprintf("%s%d", blockNumberPrefix, blockNumber))
	data, err := bc.storageManager.GetStorageMapping().Get(key)
	if err != nil || data == nil || len(data) != common.HashLength {
		return common.Hash{}, false
	}
	blockHash := common.BytesToHash(data)

	// 3. Lưu vào cache cho lần truy cập sau
	bc.blockNumberToHashCache.Store(blockNumber, cachedHash{
		hash:    blockHash,
		addedAt: time.Now(),
	})
	bc.pruneBlockNumberCache(time.Now().Add(-mappingCacheTTL))
	return blockHash, true
}

// // SetTxHashMapBlockNumber lưu ánh xạ từ tx hash sang block number.
// func (bc *BlockChain) SetTxHashMapBlockNumber(txHash common.Hash, blockNumber uint64) error {
// 	key := fmt.Sprintf("%s%s", txHashPrefix, txHash.Hex())
// 	blockNumberBytes := make([]byte, 8)
// 	binary.BigEndian.PutUint64(blockNumberBytes, blockNumber)

// 	bc.dirtyStorage.Store(key, blockNumberBytes)
// 	bc.txHashToBlockNumber.Store(txHash, blockNumber)
// 	return nil
// }

func (bc *BlockChain) SetTxHashMapBlockNumber(txHash common.Hash, blockNumber uint64) error {
	// Rate limiting
	select {
	case storeLimiter <- struct{}{}:
		defer func() { <-storeLimiter }()
	default:
		// Nếu quá tải, đợi một chút
		time.Sleep(1 * time.Millisecond)
		storeLimiter <- struct{}{}
		defer func() { <-storeLimiter }()
	}

	key := fmt.Sprintf("%s%s", txHashPrefix, txHash.Hex())
	blockNumberBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockNumberBytes, blockNumber)

	bc.dirtyStorage.Store(key, blockNumberBytes)
	bc.txHashToBlockNumber.Store(txHash, cachedUint64{
		value:   blockNumber,
		addedAt: time.Now(),
	})
	bc.pruneTxHashCache(time.Now().Add(-mappingCacheTTL))
	return nil
}

// GetBlockNumberByTxHash lấy block number từ tx hash, ưu tiên từ cache.
func (bc *BlockChain) GetBlockNumberByTxHash(txHash common.Hash) (uint64, bool) {
	// 1. Kiểm tra cache trước
	if value, ok := bc.txHashToBlockNumber.Load(txHash); ok {
		if cached, ok := value.(cachedUint64); ok {
			if time.Since(cached.addedAt) <= mappingCacheTTL {
				return cached.value, true
			}
			bc.txHashToBlockNumber.Delete(txHash)
		}
	}

	// 2. Nếu không có trong cache, truy vấn từ DB
	key := []byte(fmt.Sprintf("%s%s", txHashPrefix, txHash.Hex()))
	data, err := bc.storageManager.GetStorageMapping().Get(key)
	if err != nil || data == nil || len(data) != 8 {
		return 0, false
	}
	blockNumber := binary.BigEndian.Uint64(data)

	// 3. Lưu vào cache cho lần truy cập sau
	bc.txHashToBlockNumber.Store(txHash, cachedUint64{
		value:   blockNumber,
		addedAt: time.Now(),
	})
	bc.pruneTxHashCache(time.Now().Add(-mappingCacheTTL))
	return blockNumber, true
}

// SetEthHashMapblsHash lưu ánh xạ từ ETH hash sang BLS hash.
func (bc *BlockChain) SetEthHashMapblsHash(ethHash common.Hash, blsHash common.Hash) error {
	key := fmt.Sprintf("%s%s", ethHashMapBlsHashPrefix, ethHash.Hex())
	err := bc.storageManager.GetStorageMapping().Put([]byte(key), blsHash.Bytes())
	if err != nil {
		return err
	}
	bc.ethHashMapBlsHash.Store(ethHash, cachedHash{
		hash:    blsHash,
		addedAt: time.Now(),
	})
	bc.pruneEthHashCache(time.Now().Add(-mappingCacheTTL))
	return nil
}

// GetEthHashMapblsHash lấy BLS hash từ ETH hash, ưu tiên từ cache.
func (bc *BlockChain) GetEthHashMapblsHash(ethHash common.Hash) (common.Hash, bool) {
	// 1. Kiểm tra cache trước
	if value, ok := bc.ethHashMapBlsHash.Load(ethHash); ok {
		if cached, ok := value.(cachedHash); ok {
			if time.Since(cached.addedAt) <= mappingCacheTTL {
				return cached.hash, true
			}
			bc.ethHashMapBlsHash.Delete(ethHash)
		}
	}

	// 2. Nếu không có trong cache, truy vấn từ DB
	key := []byte(fmt.Sprintf("%s%s", ethHashMapBlsHashPrefix, ethHash.Hex()))
	data, err := bc.storageManager.GetStorageMapping().Get(key)
	if err != nil || data == nil || len(data) != common.HashLength {
		return common.Hash{}, false
	}
	blsHash := common.BytesToHash(data)

	// 3. Lưu vào cache cho lần truy cập sau
	bc.ethHashMapBlsHash.Store(ethHash, cachedHash{
		hash:    blsHash,
		addedAt: time.Now(),
	})
	bc.pruneEthHashCache(time.Now().Add(-mappingCacheTTL))
	return blsHash, true
}

// Commit ghi tất cả các thay đổi trong dirtyStorage xuống DB.
func (bc *BlockChain) Commit() error {
	var batch [][2][]byte
	bc.dirtyStorage.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if v, ok := value.([]byte); ok {
				batch = append(batch, [2][]byte{[]byte(k), v})
			}
		}
		return true
	})

	if len(batch) > 0 {
		err := bc.storageManager.GetStorageMapping().BatchPut(batch)
		if err != nil {
			return err
		}
		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error("SerializeBatch: %v", err)
			}
			bc.SetMappingBatch(data)
		}
	}

	// Xóa dirty storage sau khi đã commit
	bc.dirtyStorage = sync.Map{}
	return nil
}

// Discard hủy bỏ tất cả các thay đổi chưa được commit.
func (bc *BlockChain) Discard() {
	bc.dirtyStorage = sync.Map{}
}

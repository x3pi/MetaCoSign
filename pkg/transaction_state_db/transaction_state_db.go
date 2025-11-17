package transaction_state_db

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	// Import types
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	"github.com/meta-node-blockchain/meta-node/pkg/transaction"
	"github.com/meta-node-blockchain/meta-node/pkg/trie"
	p_trie "github.com/meta-node-blockchain/meta-node/pkg/trie"
	"github.com/meta-node-blockchain/meta-node/types"
)

var (
	lastTransactionStateRootHashKey common.Hash = common.BytesToHash(crypto.Keccak256([]byte("lastTransactionStateHashKey")))
)

type TransactionStateDB struct {
	trie              *p_trie.MerklePatriciaTrie
	originRootHash    common.Hash
	db                storage.Storage
	dirtyTransactions map[common.Hash]types.Transaction
	txBatchPut        []byte
}

func NewTransactionStateDB(
	trie *p_trie.MerklePatriciaTrie,
	db storage.Storage,
) *TransactionStateDB {
	return &TransactionStateDB{
		trie:              trie,
		db:                db,
		originRootHash:    trie.Hash(),
		dirtyTransactions: make(map[common.Hash]types.Transaction),
	}
}

func (txdb *TransactionStateDB) SetTxBatchPut(batch []byte) {
	txdb.txBatchPut = batch
}

func (txdb *TransactionStateDB) GetTxBatchPut() []byte {
	batch := txdb.txBatchPut
	txdb.txBatchPut = nil
	return batch
}
func NewTransactionStateDBFromRoot(
	rootHash common.Hash,
	db storage.Storage,
) (*TransactionStateDB, error) {
	trie, err := p_trie.New(rootHash, db, true)
	if err != nil {
		return nil, err
	}

	return &TransactionStateDB{
		trie:              trie,
		db:                db,
		originRootHash:    rootHash,
		dirtyTransactions: make(map[common.Hash]types.Transaction),
	}, nil
}

// NewTransactionStateDBFromLastRoot retrieves the last transaction state root hash from the database
// and creates a new TransactionStateDB from that root hash.
func NewTransactionStateDBFromLastRoot(db storage.Storage) (*TransactionStateDB, error) {

	rootHash := common.Hash{}
	trie, err := p_trie.New(rootHash, db, true)
	if err != nil {
		return nil, err
	}

	return &TransactionStateDB{
		trie:              trie,
		db:                db,
		originRootHash:    rootHash,
		dirtyTransactions: make(map[common.Hash]types.Transaction),
	}, nil
}
func NewTransactionStateDBFromSpecificRoot(
	rootHash common.Hash,
	db storage.Storage,
) (*TransactionStateDB, error) {
	trie, err := p_trie.New(rootHash, db, true)
	if err != nil {
		return nil, err
	}

	return &TransactionStateDB{
		trie:              trie,
		db:                db,
		originRootHash:    rootHash,
		dirtyTransactions: make(map[common.Hash]types.Transaction),
	}, nil
}

func (db *TransactionStateDB) GetAll() (map[common.Hash]types.Transaction, error) {
	allTransactions := make(map[common.Hash]types.Transaction)
	allData, err := db.trie.GetAll()
	if err != nil {
		return nil, err
	}
	for hashStr, transactionBytes := range allData {
		hash := common.HexToHash(hashStr)
		transaction := &transaction.Transaction{} // Bạn cần implement hàm này để tạo transaction phù hợp
		err := transaction.Unmarshal(transactionBytes)
		if err != nil {
			return nil, err
		}
		allTransactions[hash] = transaction
	}
	return allTransactions, nil
}

// ReloadLastRoot reloads the last transaction state root hash from the database and updates the TransactionStateDB.
func (db *TransactionStateDB) ReloadLastRoot(rootHash common.Hash) error {

	newTrie, err := p_trie.New(rootHash, db.db, true)
	if err != nil {
		return err
	}

	db.trie = newTrie
	db.originRootHash = rootHash
	db.dirtyTransactions = make(map[common.Hash]types.Transaction) // Reset dirty transactions

	return nil
}

func (db *TransactionStateDB) ReturnDB() storage.Storage {
	return db.db
}

func (db *TransactionStateDB) GetTransaction(hash common.Hash) (types.Transaction, error) {
	tx, ok := db.dirtyTransactions[hash]
	if ok {
		return tx, nil // Trả về con trỏ thay vì giá trị trực tiếp
	}

	// if not exist in dirty then get from trie
	bData, _ := db.trie.Get(hash.Bytes())
	if len(bData) == 0 {
		logger.Error("TransactionStateDB GetTransaction transaction not found", hash)
		return nil, errors.New("TransactionStateDB GetTransaction transaction not found") // Trả về nil hợp lệ cho con trỏ
	}

	// exist in trie, unmarshal
	txData := &transaction.Transaction{} // Bạn cần implement hàm này để tạo transaction phù hợp

	err := txData.Unmarshal(bData)

	if err != nil {
		logger.Error("err: ", err)
		return nil, err
	}

	return txData, nil // Trả về con trỏ đến struct
}

func (db *TransactionStateDB) SetTransaction(tx types.Transaction) {
	db.setDirtyTransaction(tx)

}

func (db *TransactionStateDB) AddTransactions(txs []types.Transaction) {
	for _, tx := range txs {
		db.setDirtyTransaction(tx)
	}
}

// Commit là phiên bản đã được tối ưu hóa và sửa lỗi.
// Nó xử lý các giao dịch 'dirty' nếu có, sau đó commit trạng thái hiện tại của trie vào DB.
// Hàm này hoạt động đúng ngay cả khi IntermediateRoot() đã được gọi trước đó.
func (db *TransactionStateDB) Commit() (common.Hash, error) {
	totalTimeStart := time.Now()

	// Giai đoạn 1: Xử lý các giao dịch 'dirty' (nếu có)
	// Đây là phần tối ưu hóa chính, giúp song song hóa việc marshal.
	if len(db.dirtyTransactions) > 0 {
		marshalTimeStart := time.Now()
		logger.Info(fmt.Sprintf("Commit: Found %d dirty transactions. Processing...", len(db.dirtyTransactions)))

		type marshalResult struct {
			hash common.Hash
			data []byte
			err  error
		}

		numJobs := len(db.dirtyTransactions)
		jobs := make(chan types.Transaction, numJobs)
		results := make(chan marshalResult, numJobs)

		numWorkers := runtime.NumCPU()
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for tx := range jobs {
					b, err := tx.Marshal()
					results <- marshalResult{hash: tx.Hash(), data: b, err: err}
				}
			}()
		}

		for _, tx := range db.dirtyTransactions {
			jobs <- tx
		}
		close(jobs)

		wg.Wait()
		close(results)

		// Thu thập kết quả và cập nhật trie
		for res := range results {
			if res.err != nil {
				return common.Hash{}, fmt.Errorf("failed to marshal transaction %s: %w", res.hash.Hex(), res.err)
			}
			checkExits, err := db.trie.Get(res.hash.Bytes())

			if checkExits != nil && err == nil {
				panic("Hash đã tồn tại ")
			}

			if err := db.trie.Update(res.hash.Bytes(), res.data); err != nil {
				return common.Hash{}, fmt.Errorf("failed to update trie for tx %s: %w", res.hash.Hex(), err)
			}
		}
		// Sau khi đã cập nhật trie, xóa danh sách dirty
		db.dirtyTransactions = make(map[common.Hash]types.Transaction)
		logger.Info(fmt.Sprintf("✅ [Phase 1] Marshalling & Trie Update completed in %v", time.Since(marshalTimeStart)))
	} else {
		logger.Info("Commit: No dirty transactions to process. Proceeding to commit current trie state.")
	}

	// Giai đoạn 2: Commit trie và chuẩn bị batch ghi vào DB
	// Giai đoạn này luôn chạy để đảm bảo trạng thái trie trong bộ nhớ được ghi xuống DB.
	trieCommitTimeStart := time.Now()
	hash, nodeSet, _, err := db.trie.Commit(true)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to commit trie: %w", err)
	}

	batch := [][2][]byte{}
	if nodeSet != nil {
		for _, node := range nodeSet.Nodes {
			batch = append(batch, [2][]byte{node.Hash.Bytes(), node.Blob})
		}
	}
	batch = append(batch, [2][]byte{lastTransactionStateRootHashKey.Bytes(), hash.Bytes()})
	logger.Info(fmt.Sprintf("✅ [Phase 2] Trie Commit & Batch Preparation completed in %v. Root hash: %s", time.Since(trieCommitTimeStart), hash.Hex()))

	// Giai đoạn 3: Ghi DB và tuần tự hóa batch
	dbWriteTimeStart := time.Now()
	if len(batch) > 0 {
		if err := db.db.BatchPut(batch); err != nil {
			return common.Hash{}, fmt.Errorf("failed to batch put to db: %w", err)
		}
		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error(fmt.Sprintf("Error serializing transaction batch: %v", err))
			} else {
				db.SetTxBatchPut(data)
			}
		}
	}
	logger.Info(fmt.Sprintf("✅ [Phase 3] DB BatchPut & Serialize completed in %v", time.Since(dbWriteTimeStart)))

	// Giai đoạn 4: Dọn dẹp và Reset
	// Reset trie về trạng thái rỗng để chuẩn bị cho block tiếp theo, giống logic của hàm gốc.
	cleanupTimeStart := time.Now()
	db.trie, err = p_trie.New(trie.EmptyRootHash, db.db, true)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to reset trie to empty state: %w", err)
	}
	db.originRootHash = trie.EmptyRootHash
	logger.Info(fmt.Sprintf("✅ [Phase 4] Cleanup and Reset completed in %v", time.Since(cleanupTimeStart)))

	logger.Info(fmt.Sprintf("🚀 Total Commit execution time: %v", time.Since(totalTimeStart)))
	return hash, nil
}

func (db *TransactionStateDB) IntermediateRoot() (common.Hash, error) {
	for hash, tx := range db.dirtyTransactions {
		b, err := tx.Marshal()
		if err != nil {
			return common.Hash{}, err
		}
		err = db.trie.Update(hash.Bytes(), b)
		if err != nil {
			return common.Hash{}, err
		}
	}
	db.dirtyTransactions = make(map[common.Hash]types.Transaction)

	return db.trie.Hash(), nil
}

func (db *TransactionStateDB) setDirtyTransaction(tx types.Transaction) {
	db.dirtyTransactions[tx.Hash()] = tx

}

func (db *TransactionStateDB) Discard() (err error) {
	db.dirtyTransactions = make(map[common.Hash]types.Transaction)
	db.trie, err = p_trie.New(db.originRootHash, db.db, true)
	return err
}

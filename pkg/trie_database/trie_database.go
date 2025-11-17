package trie_database

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/meta-node-blockchain/meta-node/pkg/account_state_db"
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

// TrieDatabaseStatus đại diện cho trạng thái của TrieDatabase.
type TrieDatabaseStatus int

const (
	Committed TrieDatabaseStatus = iota // 0: Đã commit (mặc định)
	Deleted                             // 1: Đã xóa
	Reverted                            // 2: Đã hoàn nguyên
)

type TrieDatabase struct {
	trieR          *trie.Trie
	trieDB         *triedb.Database
	originRootHash common.Hash
	db             *storage.ShardelDB
	dirtyData      sync.Map
	mu             sync.Mutex
	address        common.Address
	mvmId          common.Address
	dbName         string
	accountStateDB *account_state_db.AccountStateDB
	status         TrieDatabaseStatus // Trạng thái của TrieDatabase
	backUpDb       []byte
	subPath        string
}

func NewTrieDatabase(
	hash common.Hash,
	db *storage.ShardelDB,
	mvmId common.Address,
	address common.Address,
	dbName string,
	accountStateDB *account_state_db.AccountStateDB,

) *TrieDatabase {

	trieDB := triedb.NewDatabase(rawdb.NewDatabase(db), &triedb.Config{})
	// Tạo một đối tượng Trie mới
	var trieR *trie.Trie
	var err error

	if (hash == common.Hash{}) {
		trieR, err = trie.New(trie.TrieID(common.Hash{}), trieDB)

	} else {
		// Thử tối đa 3 lần với độ trễ giữa các lần thử
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			trieR, err = trie.New(trie.TrieID(hash), trieDB)
			if err == nil {
				break
			}
			// Nếu không phải lần thử cuối cùng, đợi trước khi thử lại
			if i < maxRetries-1 {
				time.Sleep(100 * time.Millisecond)
			}
		}

	}
	if err != nil {
		logger.Error("Error creating trie: %v", err)
		return nil
	}
	subPath := filepath.Join(address.String(), dbName)

	return &TrieDatabase{
		trieR:          trieR,
		trieDB:         trieDB,
		db:             db,
		originRootHash: trieR.Hash(),
		dirtyData:      sync.Map{},
		address:        address,
		mvmId:          mvmId,
		dbName:         dbName,
		accountStateDB: accountStateDB,
		status:         Committed, // Mặc định là Committed
		subPath:        subPath,
	}
}

// GetStatus trả về trạng thái của TrieDatabase.
func (t *TrieDatabase) GetStatus() TrieDatabaseStatus {
	return t.status
}

// GetStatus trả về trạng thái của TrieDatabase.
func (t *TrieDatabase) GetSubPath() string {
	return t.subPath
}

// SetStatus đặt trạng thái của TrieDatabase.
func (t *TrieDatabase) SetStatus(status TrieDatabaseStatus) {
	t.status = status
}

func (trieDatabae *TrieDatabase) Commit() (common.Hash, error) {
	trieDatabae.IntermediateRoot()
	trieCopy := trieDatabae.trieR.Copy()
	root, nodes := trieCopy.Commit(false)

	if nodes == nil {
		return root, nil
	}

	nodeSet := trienode.NewWithNodeSet(nodes)

	if err := trieDatabae.trieDB.Update(root, trieDatabae.originRootHash, 0, nodeSet, nil); err != nil {
		log.Fatalf("lỗi khi cập nhật trie database: %v", err)
	}

	if err := trieDatabae.trieDB.Commit(root, false); err != nil {
		log.Fatalf("lỗi khi commit trie database: %v", err)
	}

	// Create a new trie based on the new root hash
	newTrie, err := trie.New(trie.TrieID(root), trieDatabae.trieDB)
	if err != nil {
		logger.Error("Error creating new trie after commit: %v", err)
		return common.Hash{}, err
	}

	trieDatabae.trieR = newTrie
	trieDatabae.originRootHash = root

	return root, nil
}

func (trieDatabae *TrieDatabase) RestoreTrieFromRootHash(rootHash common.Hash) (*trie.Trie, error) {
	// Thử tối đa 3 lần với độ trễ giữa các lần thử
	maxRetries := 3
	var err error
	var tr *trie.Trie
	for i := 0; i < maxRetries; i++ {
		tr, err = trie.New(trie.TrieID(rootHash), trieDatabae.trieDB)
		if err == nil {
			return tr, nil
		}

		// Nếu không phải lần thử cuối cùng, đợi trước khi thử lại
		if i < maxRetries-1 {
			time.Sleep(100 * time.Millisecond)
		}
		logger.Error("Error creating trie after restore, retrying: %v", err)
	}

	// Nếu đến đây, tất cả các lần thử đều thất bại
	logger.Error("Error creating trie after multiple retries")
	return nil, err
}

func (trieDatabae *TrieDatabase) IntermediateRoot() (common.Hash, error) {
	var sortedKeys []string // Thay đổi kiểu thành string
	trieDatabae.dirtyData.Range(func(key, value interface{}) bool {
		address := key.(string) // Thay đổi kiểu thành string
		sortedKeys = append(sortedKeys, address)
		return true
	})
	sort.Slice(sortedKeys, func(i, j int) bool {
		return sortedKeys[i] < sortedKeys[j] // So sánh chuỗi trực tiếp
	})
	var batch [][2][]byte

	for _, key := range sortedKeys {
		value, _ := trieDatabae.dirtyData.Load(key)
		valStr := value.(string)
		batch = append(batch, [2][]byte{[]byte(key), []byte(valStr)})
		if err := trieDatabae.trieR.Update([]byte(key), []byte(valStr)); err != nil { // Chuyển đổi cả key và value thành []byte
			return common.Hash{}, err
		}
	}

	if len(batch) > 0 { // Chỉ thực hiện BatchPut nếu có dữ liệu

		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error(fmt.Sprintf("Error marshaling receipt: %v", err))
			}
			trieDatabae.backUpDb = data
		}
	}
	rootHash := trieDatabae.trieR.Hash()
	trieDatabae.dirtyData = sync.Map{}

	return rootHash, nil
}

func (trieDatabae *TrieDatabase) Storage() storage.Storage {
	return trieDatabae.db
}

func (trieDatabae *TrieDatabase) setDirty(key string, value string) {
	trieDatabae.dirtyData.Store(key, value)
}

func (trieDatabae *TrieDatabase) Get(
	key string,
) (string, error) {

	value, ok := trieDatabae.dirtyData.Load(key)
	if ok {
		return value.(string), nil
	}
	bData, err := trieDatabae.trieR.Get([]byte(key))
	if err != nil {
		logger.Error("TrieDatabase Get", err)
		return "", err
	}
	return string(bData), nil
}

func (trieDatabae *TrieDatabase) Put(
	key string,
	value string,
) error {
	trieDatabae.setDirty(key, value)
	return nil
}

// GetAllKeyValues retrieves all key-value pairs from both dirtyData and the trie.
// It returns a map[string]string containing all the data.  If a key exists in both
// dirtyData and the trie, the value from dirtyData takes precedence.
func (trieDatabae *TrieDatabase) GetAllKeyValues() (map[string]string, error) {
	allKeyValues := make(map[string]string)

	// Iterate over dirtyData and add/update key-value pairs in the map.
	trieDatabae.dirtyData.Range(func(key, value interface{}) bool {
		allKeyValues[key.(string)] = value.(string)
		return true
	})

	// Get key-value pairs from the trie using NodeIterator, only for keys not in dirtyData
	iter, err := trieDatabae.trieR.NodeIterator(nil)
	if err != nil {
		return nil, err
	}
	it := trie.NewIterator(iter)
	for it.Next() {
		key := string(it.Key)
		if _, ok := allKeyValues[key]; !ok {
			allKeyValues[key] = string(it.Value)
		}
	}

	return allKeyValues, nil
}

// Discard abandons all changes made since the last Commit.
func (trieDatabae *TrieDatabase) Discard() error {
	trieDatabae.mu.Lock()
	defer trieDatabae.mu.Unlock()

	newTrie, err := trieDatabae.RestoreTrieFromRootHash(trieDatabae.originRootHash)
	if err != nil {
		return err
	}

	trieDatabae.trieR = newTrie
	trieDatabae.dirtyData = sync.Map{}
	return nil
}

// SearchKeyValuesByValue searches for key-value pairs with the given value.
// It returns a map[string]string containing all matching key-value pairs.
func (trieDatabae *TrieDatabase) SearchByValue(searchValue string) (map[string]string, error) {
	matchingKeyValues := make(map[string]string)
	// Search in dirtyData
	trieDatabae.dirtyData.Range(func(key, value interface{}) bool {
		if value.(string) == searchValue {
			matchingKeyValues[key.(string)] = value.(string)
		}
		return true
	})

	// Search in trieR
	iter, err := trieDatabae.trieR.NodeIterator(nil)
	if err != nil {
		return nil, err
	}
	it := trie.NewIterator(iter)
	for it.Next() {
		if string(it.Value) == searchValue {
			key := string(it.Key)
			if _, ok := matchingKeyValues[key]; !ok {
				matchingKeyValues[key] = string(it.Value)
			}
		}
	}

	return matchingKeyValues, nil
}

// GetNextKeys returns a sorted list of keys that lexicographically follow the given startKey.
// It considers keys from both the dirty map and the committed trie.
// The number of keys returned is limited by the 'limit' parameter, with a maximum hard cap of 10.
func (trieDatabae *TrieDatabase) GetNextKeys(startKey string, limit int) ([]string, error) {
	// Determine the effective limit, capping at 10
	effectiveLimit := limit
	const maxLimit = 10
	if effectiveLimit <= 0 || effectiveLimit > maxLimit {
		effectiveLimit = maxLimit // Apply default/max limit
	}

	// Use a map to collect unique keys efficiently
	nextKeysSet := make(map[string]struct{}) // Using struct{} uses zero memory

	// 1. Iterate through dirty accounts
	trieDatabae.dirtyData.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		if keyStr > startKey {
			nextKeysSet[keyStr] = struct{}{}
		}
		return true
	})

	// 2. Iterate through the committed trie starting from the key *after* startKey
	startKeyBytes := []byte(startKey)
	iter, err := trieDatabae.trieR.NodeIterator(startKeyBytes) // Start iterator at or after startKey
	if err != nil {
		logger.Error("Error creating node iterator for GetNextKeys starting at %s: %v", startKey, err)
		return nil, fmt.Errorf("failed to create trie node iterator: %w", err)
	}

	it := trie.NewIterator(iter)
	firstKey := true // Flag to handle the case where the iterator starts exactly at startKey
	for it.Next() {
		currentKeyBytes := it.Key
		currentKeyStr := string(currentKeyBytes)

		// Skip the first key if it's exactly the startKey
		if firstKey && currentKeyStr == startKey {
			firstKey = false // Update flag
			continue         // Skip to the next iteration
		}
		firstKey = false // No longer the first key

		// Add the key to the set
		nextKeysSet[currentKeyStr] = struct{}{}

		// Optimization: If we potentially have enough candidates already, maybe stop?
		// However, we need *all* candidates first before sorting to get the *correct* next keys.
		// So, we cannot easily stop early here without a more complex merge-sort approach.
		// Given the small limit (10), collecting all then sorting is acceptable.
	}
	// Check for iterator errors after the loop
	if it.Err != nil {
		logger.Error("Error during trie iteration in GetNextKeys: %v", it.Err)
		return nil, fmt.Errorf("error during trie iteration: %w", it.Err)
	}

	// Convert the set keys to a slice
	nextKeys := make([]string, 0, len(nextKeysSet))
	for k := range nextKeysSet {
		nextKeys = append(nextKeys, k)
	}

	// Sort the keys lexicographically
	sort.Strings(nextKeys)

	// Apply the limit *after* sorting
	if len(nextKeys) > effectiveLimit {
		nextKeys = nextKeys[:effectiveLimit] // Truncate the slice
	}

	return nextKeys, nil
}

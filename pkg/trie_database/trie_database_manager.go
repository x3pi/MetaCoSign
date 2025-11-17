package trie_database

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-node-blockchain/meta-node/pkg/account_state_db"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

// TrieDatabaseManager quản lý nhiều TrieDatabase
type TrieDatabaseManager struct {
	trieDatabases    map[common.Hash]*TrieDatabase
	accountStateDB   *account_state_db.AccountStateDB
	collectedBatches map[string][]byte
}

var (
	instance *TrieDatabaseManager
	once     sync.Once
)

func CreateTrieDatabaseManager(db storage.Storage, accountStateDB *account_state_db.AccountStateDB) *TrieDatabaseManager {
	once.Do(func() {
		instance = &TrieDatabaseManager{
			trieDatabases:    make(map[common.Hash]*TrieDatabase),
			accountStateDB:   accountStateDB,
			collectedBatches: make(map[string][]byte),
		}
	})
	return instance
}
func GetTrieDatabaseManager() *TrieDatabaseManager {
	return instance
}

// CommitAllTrieDatabases duyệt qua tất cả các TrieDatabase và commit chúng.
func (manager *TrieDatabaseManager) CommitAllTrieDatabases() error {

	for id, trieDB := range manager.trieDatabases {
		switch trieDB.GetStatus() {
		case Deleted:
			if err := manager.DeleteTrieDatabase(id); err != nil {
				logger.Error("Failed to delete TrieDatabase", "id", id, "error", err)
				return err
			}
		case Reverted:
			if err := trieDB.Discard(); err != nil {
				logger.Error("Failed to discard TrieDatabase", "id", id, "error", err)
				return err
			}
		case Committed:
			key := trieDB.GetSubPath()
			value := trieDB.backUpDb
			// Thêm key-value vào map mới này
			manager.collectedBatches[key] = value
			if _, err := trieDB.Commit(); err != nil {
				return err // Trả về lỗi nếu bất kỳ commit nào không thành công
			}
		}
	}
	return nil
}
func (manager *TrieDatabaseManager) GetCollectedBatches() map[string][]byte {
	result := make(map[string][]byte)
	for k, v := range manager.collectedBatches {
		// Tạo một bản sao của slice để tránh bị thay đổi bên ngoài
		copied := make([]byte, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result
}

// ResetCollectedBatches xóa toàn bộ dữ liệu đã thu thập trong collectedBatches.
func (manager *TrieDatabaseManager) ResetCollectedBatches() {
	manager.collectedBatches = make(map[string][]byte)
}

func (manager *TrieDatabaseManager) IntermediateRoot() error {
	for id, trieDB := range manager.trieDatabases {
		switch trieDB.GetStatus() {
		case Deleted:
			as, err := manager.accountStateDB.AccountState(trieDB.address)
			if err != nil {
				logger.Error("Failed to get AccountState", "id", id, "error", err)
				return err
			}
			as.SmartContractState().DeleteTrieDatabaseMapValue(trieDB.dbName)
			manager.accountStateDB.PublicSetDirtyAccountState(as)
			return nil
		case Reverted:
			trieDB.Discard()
		default: // Bao gồm cả trạng thái Committed và các trạng thái khác
			root, err := trieDB.IntermediateRoot()
			if err != nil {
				logger.Error("Failed to get IntermediateRoot TrieDatabase", "id", id, "error", err)
				return err
			}
			as, err := manager.accountStateDB.AccountState(trieDB.address)
			if err != nil {
				logger.Error("Failed to get AccountState", "id", id, "error", err)
				return err
			}
			as.SmartContractState().SetTrieDatabaseMapValue(trieDB.dbName, root.Bytes())
			manager.accountStateDB.PublicSetDirtyAccountState(as)
			logger.Info("Updated IntermediateRoot for TrieDatabase", "id", id, "root", root)
		}
	}
	return nil
}

func (manager *TrieDatabaseManager) FindTrieDatabasesByMvmID(mvmId common.Address) []*TrieDatabase {
	var result []*TrieDatabase
	for _, trieDB := range manager.trieDatabases {
		if trieDB.mvmId == mvmId {
			result = append(result, trieDB)
		}
	}
	return result
}
func (manager *TrieDatabaseManager) FindAndSetTrieDatabasesByMvmID(mvmId common.Address, status TrieDatabaseStatus) {
	for _, trieDB := range manager.trieDatabases {
		if trieDB.mvmId == mvmId {
			trieDB.SetStatus(status)
		}
	}
}

// DiscardAllTrieDatabases loại bỏ tất cả các thay đổi đang chờ xử lý trong tất cả các TrieDatabase.
func (manager *TrieDatabaseManager) DiscardAllTrieDatabases() {
	for id, trieDB := range manager.trieDatabases {
		trieDB.Discard()
		logger.Info("Discarded TrieDatabase", "id", id)
	}
}
func (manager *TrieDatabaseManager) CloseAllTrieDatabases() error {
	for id, trieDB := range manager.trieDatabases {
		err := trieDB.trieDB.Close()
		if err != nil {
			logger.Error("Failed to close TrieDatabase", "id", id, "error", err)
			return err
		}
		logger.Info("Closed TrieDatabase", "id", id)
	}
	return nil
}

func (manager *TrieDatabaseManager) DeleteTrieDatabase(id common.Hash) error {
	trieDB, exists := manager.trieDatabases[id]
	if !exists {
		return nil // Không có gì để xóa nếu không tồn tại
	}

	// Đóng database trước khi xóa
	err := trieDB.trieDB.Close()
	if err != nil {
		logger.Error("Failed to close TrieDatabase", "id", id, "error", err)
		return err
	}

	dbNameHash := crypto.Keccak256Hash([]byte(trieDB.dbName)).Hex()
	databasePath := filepath.Join(config.ConfigApp.Databases.Trie.Path, trieDB.address.String(), dbNameHash)

	err = os.RemoveAll(databasePath)
	if err != nil {
		logger.Error("Failed to delete TrieDatabase folder", "id", id, "path", databasePath, "error", err)
		return err
	}

	delete(manager.trieDatabases, id)
	logger.Info("Deleted TrieDatabase and folder", "id", id, "path", databasePath)
	return nil
}

// GetTrieDatabase lấy một TrieDatabase theo ID của nó.
func (manager *TrieDatabaseManager) GetOrCrateTrieDatabase(id common.Hash, hash common.Hash, mvmId common.Address, address common.Address, dbName string) (*TrieDatabase, bool) {
	trieDB, exists := manager.trieDatabases[id]
	if !exists {
		dbNameHash := crypto.Keccak256Hash([]byte(dbName)).Hex()

		subPath := filepath.Join(address.String(), dbNameHash)

		databasePath := filepath.Join(config.ConfigApp.Databases.RootPath+config.ConfigApp.Databases.Trie.Path, subPath)

		database, err := storage.NewShardelDB(
			databasePath,
			1, 2,
			storage.TypeLevelDB,
			databasePath,
		)
		database.Open()
		if err != nil {
			logger.Error(err)
			return nil, false
		}
		trieDB = NewTrieDatabase(hash, database, mvmId, address, dbName, manager.accountStateDB)
		if trieDB == nil {
			return nil, false
		}
		manager.trieDatabases[id] = trieDB
	}
	return trieDB, true // trả về true nếu nó đã tồn tại, false nếu nó vừa được tạo
}

// RemoveTrieDatabase xóa một TrieDatabase khỏi danh sách quản lý
func (manager *TrieDatabaseManager) RemoveTrieDatabase(id common.Hash) {
	delete(manager.trieDatabases, id)
}

// ListAllIDs lấy danh sách tất cả các ID của TrieDatabase
func (manager *TrieDatabaseManager) ListAllIDs() []common.Hash {
	ids := make([]common.Hash, 0, len(manager.trieDatabases))
	for id := range manager.trieDatabases {
		ids = append(ids, id)
	}
	return ids
}

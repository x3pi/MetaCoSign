package smart_contract_db

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/smart_contract"
	"google.golang.org/protobuf/proto"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	"github.com/meta-node-blockchain/meta-node/pkg/trie"
	"github.com/meta-node-blockchain/meta-node/types"
)

type SmartContractDB struct {
	codeStorage     storage.Storage
	dbSmartContract storage.Storage

	accountStateDB types.AccountStateDB

	smartContractStorageTries sync.Map // Replaces map[common.Address]*trie.MerklePatriciaTrie

	pendingCode      sync.Map // Replaces map[common.Hash][]byte
	pendingEventLogs sync.Map // Replaces map[common.Address][]types.EventLog
	lastAccessTime   sync.Map // map[common.Address]time.Time

	smartContractStorageBatch []byte
	codeBatchPut              []byte
	smartContractBatch        []byte
}

func (db *SmartContractDB) CodeStorage() storage.Storage {
	return db.codeStorage
}

func (db *SmartContractDB) DbSmartContract() storage.Storage {
	return db.dbSmartContract
}

func (db *SmartContractDB) SetSmartContractStorageBatch(batch []byte) {
	db.smartContractStorageBatch = batch
}

func (db *SmartContractDB) GetSmartContractStorageBatch() []byte {
	batch := db.smartContractStorageBatch
	db.smartContractStorageBatch = nil
	return batch
}

func (db *SmartContractDB) SetCodeBatchPut(batch []byte) {
	db.codeBatchPut = batch
}

func (db *SmartContractDB) GetCodeBatchPut() []byte {
	batch := db.codeBatchPut
	db.codeBatchPut = nil
	return batch
}

func (db *SmartContractDB) SetSmartContractBatch(batch []byte) {
	db.smartContractBatch = batch
}

func (db *SmartContractDB) GetSmartContractBatch() []byte {
	batch := db.smartContractBatch
	db.smartContractBatch = nil
	return batch
}

func NewSmartContractDB(
	codeStorage storage.Storage,
	dbSmartContract storage.Storage,
	accountStateDB types.AccountStateDB,
) *SmartContractDB {
	db := &SmartContractDB{
		codeStorage:     codeStorage,
		accountStateDB:  accountStateDB,
		dbSmartContract: dbSmartContract,
	} // go db.cleanupLoop() // Start the cleanup goroutine
	return db
}

func (db *SmartContractDB) Code(address common.Address) []byte {
	account, err := db.accountStateDB.AccountState(address)
	if err != nil {
		logger.Error("Error getting account state")
		return nil
	}
	codeHash := account.SmartContractState().CodeHash()
	code, err := db.codeStorage.Get(codeHash.Bytes())
	if err != nil {
		logger.Error("Error getting code from storage")
		return nil
	}
	return code
}

func (db *SmartContractDB) GetCodeByCodeHash(address common.Address, codeHash common.Hash) []byte {
	code, err := db.codeStorage.Get(codeHash.Bytes())
	if err != nil {
		logger.Error("Error getting code from storage")
		return nil
	}
	return code
}

func (db *SmartContractDB) StorageValue(address common.Address, key []byte, customRoot ...*common.Hash) ([]byte, bool) {
	t, err := db.loadStorageTrie(address, customRoot...)
	if err != nil {
		logger.Error("failed to get storage value: error: ", err)
		// Ghi log thời gian trước khi return
		return nil, false
	}
	if t == nil {
		logger.Error("failed to get storage value: trie is nil")
		// Ghi log thời gian trước khi return
		return nil, false
	}
	value, err := t.Get(key)
	if err != nil {
		logger.Error("failed to get value from trie: error: ", err)
		// Ghi log thời gian trước khi return
		return nil, false
	}
	if value == nil {
		// Ghi log thời gian trước khi return
		return common.Hash{}.Bytes(), true
	}
	// Kết thúc đo thời gian và ghi log
	return value, true
}

func (db *SmartContractDB) SetCode(address common.Address, codeHash common.Hash, code []byte) {
	db.pendingCode.LoadOrStore(codeHash, code)
}

func (db *SmartContractDB) SetStorageValue(address common.Address, key []byte, value []byte) error {
	t, err := db.loadStorageTrie(address)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("failed to get StorageValue: db is nil")
	}
	err = t.Update(key, value)
	// as, err := db.accountStateDB.AccountState(address)
	// if err != nil {

	// }
	// db.smartContractStorageTries.LoadOrStore(address, t) // Sử dụng LoadOrStore

	// as.SetStorageRoot(t.Hash())
	return err
}

func (db *SmartContractDB) EventLogs() map[common.Address][]types.EventLog {
	result := make(map[common.Address][]types.EventLog)
	db.pendingEventLogs.Range(func(key, value interface{}) bool {
		address := key.(common.Address)
		logs := value.([]types.EventLog)
		result[address] = logs
		return true
	})
	return result
}

func (db *SmartContractDB) AddEventLogs(eventLogs []types.EventLog) {
	for _, eventLog := range eventLogs {
		address := eventLog.Address()
		eve, _ := eventLog.Marshal()
		pbLog := &pb.EventLog{}

		// Unmarshal bytes thành protobuf object
		err := proto.Unmarshal(eve, pbLog)
		if err != nil {
			logger.Warn(err)
		}
		sEventLog := &smart_contract.EventLog{}
		sEventLog.FromProto(pbLog)

		if logs, ok := db.pendingEventLogs.Load(address); ok {
			db.pendingEventLogs.LoadOrStore(address, append(logs.([]types.EventLog), eventLog))
		} else {
			db.pendingEventLogs.LoadOrStore(address, []types.EventLog{eventLog})
		}
	}
}

func GroupEventLogsByAddress(eventLogs []types.EventLog) map[common.Address][]types.EventLog {
	groupedLogs := make(map[common.Address][]types.EventLog)

	for _, eventLog := range eventLogs {
		address := eventLog.Address()
		groupedLogs[address] = append(groupedLogs[address], eventLog)
	}

	return groupedLogs
}

func (db *SmartContractDB) CommitAllStorage() error {
	var allBatches [][2][]byte
	var finalErr error

	db.smartContractStorageTries.Range(func(key, _ interface{}) bool {
		address := key.(common.Address)

		// Load trie từ database
		t, err := db.loadStorageTrie(address)
		if err != nil || t == nil {
			logger.Error("Failed to load storage trie for address:", address, "error:", err)
			finalErr = err
			return true // Tiếp tục duyệt các trie khác
		}

		// Sao chép trie để commit
		trieCommit := t.Copy()
		root, nodeSet, _, err := trieCommit.Commit(true)
		if err != nil {
			logger.Error("Error committing storage trie for address:", address)
			finalErr = err
			return true
		}

		// Kiểm tra trạng thái account
		as, asErr := db.accountStateDB.AccountState(address)
		if asErr != nil || as.SmartContractState() == nil {
			logger.Error("Invalid account state for address:", address)
			finalErr = asErr
			return true
		}

		// Kiểm tra sự khác biệt root hash
		if as.SmartContractState().StorageRoot() != root {
			logger.Info("Root hash mismatch for address:", address)
			logger.Info("Expected:", as.SmartContractState().StorageRoot(), "Got:", root)
			finalErr = fmt.Errorf("root hash mismatch for address %s", address.Hex())
			return true
		}

		// Gom dữ liệu từ nodeSet vào batch tổng hợp
		if nodeSet != nil {
			for _, node := range nodeSet.Nodes {
				allBatches = append(allBatches, [2][]byte{node.Hash.Bytes(), node.Blob})
			}
		}

		db.smartContractStorageTries.Delete(key)
		return true
	})

	if len(allBatches) > 0 {
		if err := db.dbSmartContract.BatchPut(allBatches); err != nil {
			logger.Error("CommitAllStorage Error in BatchPut:", err)
			return err
		}
	}

	if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster && len(allBatches) > 0 {
		data, err := storage.SerializeBatch(allBatches)
		if err != nil {
			logger.Error("CommitAllStorage serialize error:", err)
			return err
		}
		db.SetSmartContractStorageBatch(data)
	}

	return finalErr
}

func (db *SmartContractDB) Commit() error {
	var batch [][2][]byte

	// Commit code
	db.pendingCode.Range(func(key, value interface{}) bool {
		codeHash := key.(common.Hash)
		code := value.([]byte)
		batch = append(batch, [2][]byte{codeHash.Bytes(), code})
		return true
	})

	if len(batch) > 0 {
		if err := db.codeStorage.BatchPut(batch); err != nil {
			logger.Error("Error batch putting code:", err)
			return err
		}

		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error("Error serializing code batch:", err)
				return err
			}
			db.SetCodeBatchPut(data)
		}

		db.pendingCode.Range(func(key, _ interface{}) bool {
			db.pendingCode.Delete(key)
			return true
		})
	}

	// Commit smart contract storage
	if err := db.CommitAllStorage(); err != nil {
		logger.Error("Error committing smart contract storage:", err)
		return err
	}

	// Commit event logs
	var eventLogErr error
	db.pendingEventLogs.Range(func(key, value interface{}) bool {
		logs := value.([]types.EventLog)
		var batch [][2][]byte

		for _, log := range logs {
			eventLogBytes, err := proto.Marshal(log.Proto())
			if err != nil {
				logger.Error("Error marshaling event log:", err)
				eventLogErr = err
				return false
			}
			batch = append(batch, [2][]byte{log.Hash().Bytes(), eventLogBytes})
		}

		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error("Error serializing event log batch:", err)
				eventLogErr = err
				return false
			}
			db.SetSmartContractBatch(data)
		}

		if err := db.dbSmartContract.BatchPut(batch); err != nil {
			logger.Error("Error batch putting event logs:", err)
			eventLogErr = err
			return false
		}

		db.pendingEventLogs.Delete(key)
		return true
	})

	if eventLogErr != nil {
		return eventLogErr
	}

	return nil
}

func (db *SmartContractDB) GetLogsByHash(hash common.Hash) (*smart_contract.EventLog, error) {
	eventLogBytes, err := db.dbSmartContract.Get(hash.Bytes())
	if err != nil {
		logger.Error("Error getting event log from storage:", err)
		return nil, err
	}
	if eventLogBytes == nil {
		return nil, fmt.Errorf("event log not found for hash: %s", hash.Hex())
	}

	pbLog := &pb.EventLog{}
	err = proto.Unmarshal(eventLogBytes, pbLog)
	if err != nil {
		logger.Error("Error unmarshaling event log:", err)
		return nil, err
	}

	sEventLog := &smart_contract.EventLog{}
	sEventLog.FromProto(pbLog)

	return sEventLog, nil
}

func (db *SmartContractDB) StorageRoot(address common.Address, customRoot ...*common.Hash) common.Hash {
	t, err := db.loadStorageTrie(address, customRoot...)
	if err != nil {
		logger.Error("failed to get storage root: error: ", err)
		return common.Hash{}
	}
	if t == nil {
		logger.Error("failed to get storage root: trie is nil")
		return common.Hash{}
	}
	return t.Hash()
}

func (db *SmartContractDB) loadStorageTrie(address common.Address, customRoot ...*common.Hash) (*trie.MerklePatriciaTrie, error) {

	db.lastAccessTime.LoadOrStore(address, time.Now())
	if t, ok := db.smartContractStorageTries.Load(address); ok {

		if t == nil {
			return nil, fmt.Errorf("trie is nil for address: %s", address.Hex())
		}

		return t.(*trie.MerklePatriciaTrie), nil
	}
	var root common.Hash
	if len(customRoot) > 0 && customRoot[0] != nil {
		root = *customRoot[0]
	} else {

		as, err := db.accountStateDB.AccountState(address)

		if err != nil || as.SmartContractState() == nil {
			root = common.Hash{}
		} else {

			root = as.SmartContractState().StorageRoot()

		}
	}

	t, err := trie.New(root, db.dbSmartContract, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create trie for address: %s, root %s, error: %w", address.Hex(), root, err)
	}
	db.smartContractStorageTries.LoadOrStore(address, t) // Sử dụng LoadOrStore

	// Kết thúc đo thời gian và ghi log
	return t, nil
}

func (db *SmartContractDB) Discard() {
	db.pendingCode.Range(func(key, _ interface{}) bool {
		db.pendingCode.Delete(key)
		return true
	})
	db.smartContractStorageTries.Range(func(key, _ interface{}) bool {
		db.smartContractStorageTries.Delete(key)
		return true
	})
}

package blockchain

import (
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/account_state_db"
	"github.com/meta-node-blockchain/meta-node/pkg/block"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/smart_contract_db"
	stake_state_db "github.com/meta-node-blockchain/meta-node/pkg/state_db"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	"github.com/meta-node-blockchain/meta-node/pkg/trie"
	"github.com/meta-node-blockchain/meta-node/types"
)

// ChainState quản lý trạng thái toàn cục của blockchain
type ChainState struct {
	config *config.SimpleChainConfig

	currentBlockHeader atomic.Pointer[types.BlockHeader]
	storageManager     *storage.StorageManager
	accountStateDB     *account_state_db.AccountStateDB
	smartContractDB    *smart_contract_db.SmartContractDB
	blockDatabase      *block.BlockDatabase
	stakeStateDB       *stake_state_db.StakeStateDB
	freeFeeAddress     map[common.Address]struct{}
}

// NewChainState tạo một đối tượng ChainState mới.
// Nó cần một StorageManager và header của block cuối cùng (lastHeader) đã biết.
func NewChainState(
	sm *storage.StorageManager,
	blockDatabase *block.BlockDatabase,
	currentBlockHeader types.BlockHeader,
	config *config.SimpleChainConfig,
	freeFeeAddress map[common.Address]struct{},
) (*ChainState, error) {
	// Create account state trie from existing root
	accountStorage := sm.GetStorageAccount()
	accountStateTrie, err := trie.New(currentBlockHeader.AccountStatesRoot(), accountStorage, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create account state trie: %v", err)
	}
	stakeStorage := sm.GetStorageStake()

	stakeStateTrie, err := trie.New(common.Hash(currentBlockHeader.StakeStatesRoot()), stakeStorage, true)

	if err != nil {
		return nil, fmt.Errorf("failed to create account state trie: %v", err)
	}

	asDB := account_state_db.NewAccountStateDB(accountStateTrie, accountStorage)

	stakeStateDB := stake_state_db.NewStakeStateDB(stakeStateTrie, stakeStorage)

	scDB := smart_contract_db.NewSmartContractDB(
		sm.GetStorageCode(),
		sm.GetStorageSmartContract(),
		asDB)

	cs := &ChainState{
		storageManager:  sm,
		accountStateDB:  asDB,
		stakeStateDB:    stakeStateDB,
		smartContractDB: scDB,
		config:          config,
		blockDatabase:   blockDatabase,
		freeFeeAddress:  freeFeeAddress,
	}

	headerCopy := currentBlockHeader
	cs.currentBlockHeader.Store(&headerCopy)

	return cs, nil // Trả về ChainState đã tạo và nil error
}

// UpdateStateForNewHeader cập nhật trạng thái dựa trên header mới.
// Hàm này sẽ cập nhật con trỏ header và khởi tạo lại các DB trạng thái liên quan.
func (cs *ChainState) UpdateStateForNewHeader(newHeader types.BlockHeader) error {
	if newHeader == nil {
		return fmt.Errorf("cannot update state with a nil header")
	}
	// 1. Khởi tạo lại AccountStateDB với root mới
	accountStorage := cs.storageManager.GetStorageAccount()
	newAccountRoot := newHeader.AccountStatesRoot() // Lấy root từ header mới
	newAccountStateTrie, err := trie.New(newAccountRoot, accountStorage, true)
	if err != nil {
		logger.Error("Failed to create new account state trie during update", "error", err, "newRoot", newAccountRoot)
		return fmt.Errorf("failed to create new account state trie for update: %w", err)
	}
	newAsDB := account_state_db.NewAccountStateDB(newAccountStateTrie, accountStorage)

	// 2. Khởi tạo lại StakeStateDB với root mới
	stakeStorage := cs.storageManager.GetStorageStake()
	newStakeRoot := common.Hash(newHeader.StakeStatesRoot()) // Lấy root từ header mới
	newStakeStateTrie, err := trie.New(newStakeRoot, stakeStorage, true)
	if err != nil {
		logger.Error("Failed to create new stake state trie during update", "error", err, "newRoot", newStakeRoot)
		return fmt.Errorf("failed to create new stake state trie for update: %w", err)
	}
	newStakeStateDB := stake_state_db.NewStakeStateDB(newStakeStateTrie, stakeStorage)

	// 3. Khởi tạo lại SmartContractDB với AccountStateDB mới
	// (Giả sử config không thay đổi)
	newScDB := smart_contract_db.NewSmartContractDB(
		cs.storageManager.GetStorageCode(),
		cs.storageManager.GetStorageSmartContract(),
		newAsDB, // Sử dụng asDB mới tạo
	)

	// 4. Cập nhật các trường trong ChainState
	cs.accountStateDB = newAsDB
	cs.stakeStateDB = newStakeStateDB // Thêm dòng này để cập nhật stakeStateDB
	cs.smartContractDB = newScDB

	// 5. Cập nhật con trỏ header nguyên tử
	headerCopy := newHeader
	cs.currentBlockHeader.Store(&headerCopy)

	logger.Info("ChainState updated for new header", "blockNumber", newHeader.BlockNumber(), "accountRoot", newAccountRoot, "stakeRoot", newStakeRoot)
	return nil
}

// NewChainState tạo một đối tượng ChainState mới.
// Nó cần một StorageManager và header của block cuối cùng (lastHeader) đã biết.
func NewChainStateRemote(
	currentBlockHeader types.BlockHeader,
	accountStorage,
	codeStorage, dbSmartContract storage.Storage,
	freeFeeAddress map[common.Address]struct{},
) (*ChainState, error) {

	// Create account state trie from existing root
	accountStateTrie, err := trie.New(currentBlockHeader.AccountStatesRoot(), accountStorage, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create account state trie: %v", err)
	}
	asDB := account_state_db.NewAccountStateDB(accountStateTrie, accountStorage)
	scDB := smart_contract_db.NewSmartContractDB(
		codeStorage,
		dbSmartContract,
		asDB)

	cs := &ChainState{
		accountStateDB:  asDB,
		smartContractDB: scDB,
		freeFeeAddress:  freeFeeAddress,
	}

	headerCopy := currentBlockHeader
	cs.currentBlockHeader.Store(&headerCopy)

	return cs, nil // Trả về ChainState đã tạo và nil error
}

// GetConfig trả về cấu hình của ChainState.
func (cs *ChainState) GetConfig() *config.SimpleChainConfig {
	return cs.config
}
func (cs *ChainState) TransferFrom(from, to types.AccountState, amount *big.Int) error {
	if from == nil || to == nil {
		return errors.New("invalid account: from or to is nil")
	}
	if amount == nil {
		return errors.New("invalid amount: nil")
	}
	if amount.Cmp(big.NewInt(0)) < 0 {
		return errors.New("amount must be greater than zero")
	}
	// Trừ bên gửi
	err := from.SubTotalBalance(amount)
	if err != nil {
		return err
	}
	// Cộng bên nhận
	to.AddPendingBalance(amount)
	return nil
}

// GetcurrentBlock trả về header của block cuối cùng một cách an toàn.
// Trả về nil nếu chưa có header nào được đặt.
func (cs *ChainState) GetcurrentBlockHeader() *types.BlockHeader {
	return cs.currentBlockHeader.Load()
}

// SetcurrentBlock cập nhật header của block cuối cùng một cách an toàn.
func (cs *ChainState) SetcurrentBlockHeader(header *types.BlockHeader) {
	cs.currentBlockHeader.Store(header)
}

func (cs *ChainState) GetAccountStateDB() *account_state_db.AccountStateDB {
	return cs.accountStateDB // Bên trong gói blockchain, ta có thể truy cập trực tiếp
}

// GetSmartContractDB trả về SmartContractDB.
func (cs *ChainState) GetSmartContractDB() *smart_contract_db.SmartContractDB {
	return cs.smartContractDB
}

func (cs *ChainState) GetStakeStateDB() *stake_state_db.StakeStateDB {
	return cs.stakeStateDB
}

// GetStorageManager trả về StorageManager.
func (cs *ChainState) GetStorageManager() *storage.StorageManager {
	return cs.storageManager
}

// SetAccountStateDB đặt AccountStateDB cho ChainState.
// Lưu ý: Việc thay đổi trực tiếp DB này có thể ảnh hưởng đến tính nhất quán
// của trạng thái nếu không được quản lý cẩn thận trong quy trình xử lý block.
func (cs *ChainState) SetAccountStateDB(asDB *account_state_db.AccountStateDB) {
	cs.accountStateDB = asDB
}

// GetBlockDatabase trả về BlockDatabase.
func (cs *ChainState) GetBlockDatabase() *block.BlockDatabase {
	return cs.blockDatabase
}

func (cs *ChainState) GetFreeFeeAddress() map[common.Address]struct{} {
	return cs.freeFeeAddress
}

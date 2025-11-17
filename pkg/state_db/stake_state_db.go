package stake_state_db

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/loggerfile"
	"github.com/meta-node-blockchain/meta-node/pkg/state"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	p_trie "github.com/meta-node-blockchain/meta-node/pkg/trie"
)

// StakeStateDB quản lý state staking của các validator sử dụng Merkle Patricia Trie.
type StakeStateDB struct {
	trie *p_trie.MerklePatriciaTrie

	originRootHash common.Hash
	db             storage.Storage
	//cache tránh trie truy vấn nhiều lần
	dirtyValidators sync.Map

	lockedFlag atomic.Bool

	stakeBatch []byte

	muCommit sync.Mutex
}

// NewStakeStateDB tạo một instance mới của StakeStateDB.
func NewStakeStateDB(
	trie *p_trie.MerklePatriciaTrie,
	db storage.Storage,
) *StakeStateDB {
	if trie == nil || db == nil {
		logger.Error("NewStakeStateDB received a nil trie or db storage")
		return nil
	}
	return &StakeStateDB{
		trie:            trie,
		db:              db,
		originRootHash:  trie.Hash(),
		dirtyValidators: sync.Map{},
	}
}

func (db *StakeStateDB) getOrCreateValidatorState(
	address common.Address,
) (state.ValidatorState, error) {
	value, ok := db.dirtyValidators.Load(address)
	if ok {
		if vs, valid := value.(state.ValidatorState); valid && vs != nil {
			return vs, nil
		}
	}

	trieToUse := db.trie
	if trieToUse == nil {
		return nil, errors.New("stake state DB has a nil trie")
	}
	bData, err := trieToUse.Get(address.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error getting %s from Trie: %w", address.Hex(), err)
	}

	var stateToStore state.ValidatorState
	if len(bData) == 0 {
		stateToStore = state.NewValidatorState(address)
	} else {
		loadedVs := state.NewValidatorState(address)
		if err = loadedVs.Unmarshal(bData); err != nil {
			return nil, fmt.Errorf("error unmarshalling %s from Trie: %w", address.Hex(), err)
		}
		stateToStore = loadedVs
	}

	if db.lockedFlag.Load() {
		return stateToStore, nil
	}

	actualValue, _ := db.dirtyValidators.LoadOrStore(address, stateToStore)
	finalVs, _ := actualValue.(state.ValidatorState)
	return finalVs, nil
}

func (db *StakeStateDB) setDirtyValidatorState(vs state.ValidatorState) {
	if vs == nil {
		return
	}
	if db.lockedFlag.Load() {
		panic("Error: setDirtyValidatorState called while db is locked!")
	}
	db.dirtyValidators.Store(vs.Address(), vs)
}

// --- Các hàm sửa đổi State ---
func (db *StakeStateDB) CreateRegister(
	address common.Address,
	name string,
	description string,
	website string,
	image string,
	commissionRate uint64,
	minSelfDelegation *big.Int,
	primaryAddress string,
	workerAddress string,
	p2pAddress string,
	pubKeyBls string,
	pubKeySecp string,
) error {
	if db.lockedFlag.Load() {
		return errors.New("CreateRegister: db is locked")
	}
	vs, err := db.getOrCreateValidatorState(address)
	if err != nil {
		return err
	}
	vs.SetName(name)
	vs.SetDescription(description)
	vs.SetWebsite(website)
	vs.SetImage(image)
	vs.SetCommissionRate(commissionRate)
	vs.SetMinSelfDelegation(minSelfDelegation)
	vs.SetPrimaryAddress(primaryAddress)
	vs.SetWorkerAddress(workerAddress)
	vs.SetP2PAddress(p2pAddress)
	vs.SetPubKeyBls(pubKeyBls)
	vs.SetPubKeySecp(pubKeySecp)

	db.setDirtyValidatorState(vs)

	return nil
}
func (db *StakeStateDB) DeleteValidator(address common.Address) error {
	if db.lockedFlag.Load() {
		return errors.New("DeleteValidator: db is locked")
	}
	data, err := db.GetValidator(address)
	if err != nil {
		return fmt.Errorf("could not get validator state for deletion: %w", err)
	}
	if data != nil {
		db.dirtyValidators.Store(address, nil)
	}
	return nil

}
func (db *StakeStateDB) GetDelegation(validatorAddress, delegatorAddress common.Address) (*big.Int, *big.Int, error) {
	vs, err := db.GetValidator(validatorAddress)
	if err != nil {
		return big.NewInt(0), big.NewInt(0), fmt.Errorf("could not get validator state for deletion: %w", err)
	}
	if vs == nil {
		return big.NewInt(0), big.NewInt(0), nil
	}
	amount, rewardDebt := vs.GetDelegation(delegatorAddress)
	return amount, rewardDebt, nil
}

func (db *StakeStateDB) Delegate(validatorAddress, delegatorAddress common.Address, amount *big.Int) error {
	if db.lockedFlag.Load() {
		return errors.New("Delegate: db is locked")
	}
	vs, err := db.getOrCreateValidatorState(validatorAddress)
	if err != nil {
		return err
	}
	vs.SetDelegate(delegatorAddress, amount)
	db.setDirtyValidatorState(vs)

	return nil
}

func (db *StakeStateDB) Undelegate(delegatorAddress, validatorAddress common.Address, amount *big.Int) error {
	if db.lockedFlag.Load() {
		return errors.New("Undelegate: db is locked")
	}
	vs, err := db.GetValidator(validatorAddress)
	if err != nil {
		return err
	}
	if vs == nil {
		return fmt.Errorf("validator %s not found", validatorAddress.Hex())
	}
	if err := vs.SetUndelegate(delegatorAddress, amount); err != nil {
		return err
	}
	db.setDirtyValidatorState(vs)
	return nil
}

// --- Quản lý Phần thưởng ---

func (db *StakeStateDB) DistributeRewardsToValidator(validatorAddress common.Address, totalBlockReward *big.Int) (*big.Int, error) {
	if db.lockedFlag.Load() {
		return nil, errors.New("DistributeRewardsToValidator: db is locked")
	}
	vs, err := db.GetValidator(validatorAddress)
	if err != nil {
		return nil, fmt.Errorf("could not get validator state for reward distribution: %w", err)
	}
	rewardAmount := vs.DistributeRewards(totalBlockReward)
	db.setDirtyValidatorState(vs)
	return rewardAmount, nil
}

func (db *StakeStateDB) WithdrawRewardFromValidator(validatorAddress, delegatorAddress common.Address) (*big.Int, error) {
	if db.lockedFlag.Load() {
		return nil, errors.New("WithdrawRewardFromValidator: db is locked")
	}
	vs, err := db.getOrCreateValidatorState(validatorAddress)
	if err != nil {
		return nil, err
	}
	rewardAmount := vs.WithdrawReward(delegatorAddress)
	return rewardAmount, nil
}
func (db *StakeStateDB) ResetRewardDebtForDelegator(validatorAddress, delegatorAddress common.Address) error {
	if db.lockedFlag.Load() {
		return errors.New("ResetRewardDebtForDelegator: db is locked")
	}
	vs, err := db.GetValidator(validatorAddress)
	if err != nil {
		return err
	}
	if vs == nil {
		return fmt.Errorf("validator %s not found", validatorAddress.Hex())
	}
	vs.ResetRewardDebt(delegatorAddress)
	db.setDirtyValidatorState(vs)
	return nil
}
func (db *StakeStateDB) GetPendingRewards(validatorAddress, delegatorAddress common.Address) (*big.Int, error) {
	if db.lockedFlag.Load() {
		return nil, errors.New("GetPendingRewards: db is locked")
	}
	vs, err := db.getOrCreateValidatorState(validatorAddress)
	if err != nil {
		return nil, err
	}
	rewardAmount := vs.WithdrawReward(delegatorAddress)
	return rewardAmount, nil
}

// --- Các hàm truy vấn ---

func (db *StakeStateDB) GetAllValidators() ([]state.ValidatorState, error) {
	trieToUse := db.trie
	if trieToUse == nil {
		return nil, errors.New("stake state DB has a nil trie")
	}
	allData, err := trieToUse.GetAll()
	if err != nil {
		return nil, fmt.Errorf("error getting all data from trie: %w", err)
	}
	allValidators := make([]state.ValidatorState, 0, len(allData))
	for addressStr, validatorStateBytes := range allData {
		address := common.HexToAddress(addressStr)
		validatorState := state.NewValidatorState(address)
		if err := validatorState.Unmarshal(validatorStateBytes); err != nil {
			logger.Warn("Failed to unmarshal validator state", "address", address.Hex(), "error", err)
			continue
		}
		allValidators = append(allValidators, validatorState)
	}
	return allValidators, nil
}

func (db *StakeStateDB) GetValidatorCount() (int, error) {
	data, err := db.GetAllValidators()
	if err != nil {
		return 0, err
	}

	return len(data), nil
}
func (db *StakeStateDB) GetValidator(address common.Address) (state.ValidatorState, error) {
	// 1. Kiểm tra cache trước
	value, ok := db.dirtyValidators.Load(address)
	if ok {
		if vs, valid := value.(state.ValidatorState); valid || vs == nil {
			return vs, nil
		}
	}
	// 2. Truy vấn Trie
	trieToUse := db.trie
	if trieToUse == nil {
		return nil, errors.New("stake state DB has a nil trie")
	}
	bData, err := trieToUse.Get(address.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error getting %s from Trie: %w", address.Hex(), err)
	}
	// 3. Nếu không có dữ liệu, validator không tồn tại
	if len(bData) == 0 {
		return nil, nil // Không tìm thấy
	}
	// 4. Giải mã dữ liệu và trả về
	// là chỉ đang lấy đối tượng vs để thực thi các hàm thôi
	vs := state.NewValidatorState(address)
	if err = vs.Unmarshal(bData); err != nil {
		return nil, fmt.Errorf("error unmarshalling %s from Trie: %w", address.Hex(), err)
	}

	return vs, nil
}

func (db *StakeStateDB) SetCommissionRate(address common.Address, newRate uint64) (bool, error) {
	if db.lockedFlag.Load() {
		return false, errors.New("GetPendingRewards: db is locked")
	}
	vs, err := db.GetValidator(address)
	if err != nil {
		return false, err
	}
	if vs == nil {
		return false, fmt.Errorf("validator %s not found", address.Hex())
	}
	vs.SetCommissionRate(newRate)
	db.setDirtyValidatorState(vs)
	return true, nil
}

func (db *StakeStateDB) UpdateValidatorInfo(address common.Address, name, description, website, image string) (bool, error) {
	if db.lockedFlag.Load() {
		return false, errors.New("GetPendingRewards: db is locked")
	}
	vs, err := db.GetValidator(address)
	if err != nil {
		return false, err
	}
	if vs == nil {
		return false, fmt.Errorf("validator %s not found", address.Hex())
	}
	vs.SetName(name)
	vs.SetDescription(description)
	vs.SetWebsite(website)
	vs.SetImage(image)

	db.setDirtyValidatorState(vs)
	return true, nil
}

func (db *StakeStateDB) GetAndSortValidators(descending bool) ([]state.ValidatorState, error) {
	validators, err := db.GetAllValidators()
	if err != nil {
		return nil, err
	}
	sort.Slice(validators, func(i, j int) bool {
		cmp := validators[i].TotalStakedAmount().Cmp(validators[j].TotalStakedAmount())
		if descending {
			return cmp > 0
		}
		return cmp < 0
	})
	return validators, nil
}
func (db *StakeStateDB) GetValidatorAddresses() ([]common.Address, error) {
	sortedValidators, err := db.GetAndSortValidators(true)
	if err != nil {
		return nil, err
	}
	addresses := make([]common.Address, len(sortedValidators))
	for i, v := range sortedValidators {
		addresses[i] = v.Address()
	}
	return addresses, nil
}
func (db *StakeStateDB) GetValidatorIndex(validatorAddress common.Address) (*big.Int, bool, error) {
	sortedValidators, err := db.GetAndSortValidators(true)
	if err != nil {
		return nil, false, err
	}
	index := sort.Search(len(sortedValidators), func(i int) bool {
		// So sánh địa chỉ tìm kiếm với địa chỉ tại vị trí i
		return bytes.Compare(sortedValidators[i].Address().Bytes(), validatorAddress.Bytes()) >= 0
	})
	// Kiểm tra xem có thực sự tìm thấy tại chỉ số đó không
	if index < len(sortedValidators) && sortedValidators[index].Address() == validatorAddress {
		return big.NewInt(int64(index)), true, nil
	}

	return nil, false, nil // Không tìm thấy
}

// --- Quản lý State: Commit, Discard, ... ---

// ⭐ HÀM MỚI: SetStakeBatch stores a serialized batch of stake data.
func (db *StakeStateDB) SetStakeBatch(batch []byte) {
	db.stakeBatch = batch
}

// ⭐ HÀM MỚI: GetStakeBatch retrieves and clears the stored stake batch.
func (db *StakeStateDB) GetStakeBatch() []byte {
	batch := db.stakeBatch
	db.stakeBatch = nil // Clear after retrieval
	return batch
}

func (db *StakeStateDB) GetStorage() storage.Storage {
	return db.db
}

// IntermediateRoot applies dirty validator states to the in-memory trie and returns the new root hash.
// This function is crucial for the commit process.
func (db *StakeStateDB) IntermediateRoot(isLockProcess ...bool) (common.Hash, error) {
	fileLogger, _ := loggerfile.NewFileLogger(fmt.Sprintf("IntermediateRoot_" + ".log"))

	var lockProcess bool
	if len(isLockProcess) > 0 {
		lockProcess = isLockProcess[0]
	} else {
		lockProcess = true // Default to true
	}

	if lockProcess {
		if db.lockedFlag.Load() {
			err := errors.New("IntermediateRoot (lockProcess=true): db.lockedFlag is already locked")
			logger.Error(err.Error())
			return common.Hash{}, err
		}
		db.lockedFlag.Store(true)
	} else {
		if !db.lockedFlag.Load() {
			err := errors.New("IntermediateRoot (lockProcess=false): db.lockedFlag is not locked")
			logger.Error(err.Error())
			return common.Hash{}, err
		}
		defer func() {
			db.dirtyValidators = sync.Map{}
			db.lockedFlag.Store(false)
		}()
	}

	if db.trie == nil {
		return common.Hash{}, errors.New("trie is nil")
	}

	var (
		updateErr  error
		hasChanges bool = false
	)

	db.dirtyValidators.Range(func(key, value interface{}) bool {
		hasChanges = true
		address, ok1 := key.(common.Address)
		if !ok1 {
			logger.Warn("Skipping invalid key in dirtyValidators", "keyType", fmt.Sprintf("%T", key))
			return true // Bỏ qua và tiếp tục
		}
		var bytesToStore []byte
		if value != nil {
			vs, ok2 := value.(state.ValidatorState)
			fileLogger.Info("IntermediateRoot: vs", vs)
			if !ok2 {
				// Nếu value không nil nhưng không phải ValidatorState, đây là lỗi
				updateErr = fmt.Errorf("invalid value type for address %s", address.Hex())
				return false // Dừng lại
			}
			var err error
			bytesToStore, err = vs.Marshal()
			if err != nil {
				updateErr = fmt.Errorf("marshal error for %s: %w", address.Hex(), err)
				return false // stop iteration
			}
		}
		fileLogger.Info("Update: %V", address, hex.EncodeToString(bytesToStore))

		if err := db.trie.Update(address.Bytes(), bytesToStore); err != nil {
			updateErr = fmt.Errorf("trie update error for %s: %w", address.Hex(), err)
			return false // stop iteration
		}
		return true
	})

	if updateErr != nil {
		return common.Hash{}, updateErr
	}

	if hasChanges {

		newHash := db.trie.Hash()
		logger.Debug("Calculated new intermediate hash for stake state", "newHash", newHash)
		fileLogger.Info("IntermediateRoot: hasChanges", newHash)

		return newHash, nil
	} else {
		nHash := db.trie.Hash()
		fileLogger.Info("IntermediateRoot: nHash", nHash)

		return nHash, nil // Return current hash if no changes
	}
}

func (db *StakeStateDB) Commit() (common.Hash, error) {
	fileLogger, _ := loggerfile.NewFileLogger(fmt.Sprintf("Commit" + ".log"))

	db.muCommit.Lock()
	defer db.muCommit.Unlock()

	if !db.lockedFlag.Load() {
		return common.Hash{}, errors.New("Commit: db is not already locked")
	}
	db.lockedFlag.Store(true) // Lock for IntermediateRoot

	committedHash, nodeSet, _, err := db.trie.Commit(true)
	if err != nil {
		return common.Hash{}, fmt.Errorf("trie Commit calculation failed: %w", err)
	}

	// IntermediateRoot(false) will apply changes, clear the dirty map, and unlock the flag.
	intermediateHash, err := db.IntermediateRoot(false)
	if err != nil {
		// On error, IntermediateRoot's defer handles cleanup.
		return common.Hash{}, fmt.Errorf("trie IntermediateRoot calculation failed: %w", err)
	}
	fileLogger.Info("IntermediateRoot: intermediateHash", intermediateHash)

	fileLogger.Info("IntermediateRoot: committedHash", committedHash)

	if intermediateHash != committedHash {
		return common.Hash{}, fmt.Errorf("root hash mismatch after commit (intermediate: %s, commit: %s)", intermediateHash, committedHash)
	}

	if nodeSet != nil && len(nodeSet.Nodes) > 0 {
		batch := make([][2][]byte, 0, len(nodeSet.Nodes))
		for _, node := range nodeSet.Nodes {
			batch = append(batch, [2][]byte{node.Hash.Bytes(), node.Blob})
		}

		// Prepare batch for network transfer if this is a master node.
		if config.ConfigApp != nil && config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			if len(batch) > 0 {
				data, err := storage.SerializeBatch(batch)
				if err != nil {
					fileLogger.Info("Commit (StakeStateDB): Failed to serialize batch for network transfer", "error", err)
					// Log the error but continue the commit; local persistence is the priority.
					logger.Error("Commit (StakeStateDB): Failed to serialize batch for network transfer", "error", err)
				} else {
					db.SetStakeBatch(data)
					fileLogger.Info("Commit (StakeStateDB): Serialized stake batch for network transfer", "size_bytes intermediateHash", len(data), intermediateHash)

					logger.Debug("Commit (StakeStateDB): Serialized stake batch for network transfer", "size_bytes", len(data))
				}
			}
		}

		if err := db.db.BatchPut(batch); err != nil {
			return common.Hash{}, fmt.Errorf("DB BatchPut failed: %w", err)
		}
	}

	newTrie, err := p_trie.New(committedHash, db.db, true)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to load trie for new root %s: %w", committedHash, err)
	}

	db.trie = newTrie
	db.originRootHash = committedHash

	return committedHash, nil
}

func (db *StakeStateDB) Discard() error {
	if db.lockedFlag.Load() {
		return errors.New("Discard: db is locked")
	}
	db.dirtyValidators = sync.Map{}
	newTrie, err := p_trie.New(db.originRootHash, db.db, true)
	if err != nil {
		return fmt.Errorf("failed to reload trie to %s: %w", db.originRootHash, err)
	}
	db.trie = newTrie
	return nil
}

package account_state_db

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync" // Keep sync package
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"

	// Assume these paths are correct for your project structure
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/state" // Assuming AccountState implementation is here
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	p_trie "github.com/meta-node-blockchain/meta-node/pkg/trie"
	"github.com/meta-node-blockchain/meta-node/types"
)

// AccountStateDB manages account states using a Merkle Patricia Trie and a cache for dirty states.
// It is designed to be concurrency-safe for operations on different accounts,
// and uses specific locking for structural changes and commit operations.
type AccountStateDB struct {
	trie *p_trie.MerklePatriciaTrie // The underlying Merkle Patricia Trie storing account states

	originRootHash common.Hash     // The root hash of the trie when the DB was initialized or last committed/reloaded
	db             storage.Storage // The persistent key-value store backing the trie

	// dirtyAccounts caches account states that have been modified but not yet committed.
	// Uses sync.Map for fine-grained locking on individual key reads/writes.
	dirtyAccounts sync.Map

	// muStruct protects structural changes to the AccountStateDB, specifically:
	// - Reassignment of the dirtyAccounts map (in ReloadTrie, Discard, IntermediateRoot, CopyFrom)
	// - Reassignment of the trie reference (in ReloadTrie, Commit, CopyFrom)
	// - Iteration over dirtyAccounts (in IntermediateRoot, CopyFrom)
	// RLock is used in getOrCreateAccountState to allow concurrent reads/stores via sync.Map API
	// while preventing the map or trie reference from being replaced.

	lockedFlag atomic.Bool // CHANGED: Use atomic.Bool

	// and updating the live trie reference) happens atomically.
	muCommit sync.Mutex

	accountBatch []byte // Batch of account data prepared for network transfer during commit (used by master nodes)
}

// NewAccountStateDB creates a new instance of AccountStateDB.
func NewAccountStateDB(
	trie *p_trie.MerklePatriciaTrie,
	db storage.Storage,
) *AccountStateDB {
	if trie == nil {
		// Handle case where trie might be nil, perhaps initialize an empty one?
		// For now, assume a valid trie is passed.
		logger.Error("NewAccountStateDB received a nil trie")
		// Depending on requirements, might return nil or an error, or create an empty trie.
		// Returning struct with nil trie will likely cause panics later.
		// Let's assume trie is required.
	}
	if db == nil {
		logger.Error("NewAccountStateDB received a nil db storage")
		// Storage is essential.
		return nil // Or return an error
	}
	return &AccountStateDB{
		trie:           trie,
		db:             db,
		originRootHash: trie.Hash(),
		dirtyAccounts:  sync.Map{}, // Initialize sync.Map
	}
}

// ReloadTrie replaces the current trie with a new one based on the given root hash.
// It clears the dirty account cache. This requires exclusive access to modify the structure.
func (db *AccountStateDB) ReloadTrie(rootHash common.Hash) error {

	if db.lockedFlag.Load() {
		panic("ReloadTrie db.lockedFlag is already locked")
		// return errors.New("ReloadTrie db.lockedFlag is already locked")
	}

	newTrie, err := p_trie.New(rootHash, db.db, true)
	if err != nil {
		logger.Error("ReloadTrie: Failed to create new trie instance", "hash", rootHash, "error", err)
		return fmt.Errorf("failed to load trie for root %s: %w", rootHash, err)
	}
	db.trie = newTrie
	db.originRootHash = rootHash
	db.dirtyAccounts = sync.Map{} // Clear dirty accounts under lock

	return nil
}

// AccountState retrieves the state for a given address.
// It fetches from the dirty cache first, then the underlying trie.
// If the account doesn't exist, a new state is created and cached.
func (db *AccountStateDB) AccountState(address common.Address) (types.AccountState, error) {
	return db.getOrCreateAccountState(address)
}

// GetAll retrieves all account states directly from the *committed* state in the trie.
// Note: This does NOT include uncommitted changes from the dirtyAccounts cache.
// If you need a complete snapshot including dirty states, you would need to
// acquire muStruct, get all from trie, and then merge in the dirtyAccounts data.
func (db *AccountStateDB) GetAll() (map[common.Address]types.AccountState, error) {
	// --- Acquire Read Lock to safely access db.trie ---
	// Protects against db.trie being replaced by ReloadTrie, Commit, CopyFrom.
	trieToUse := db.trie // Get the reference under lock
	// --- Read Lock Released ---

	if trieToUse == nil {
		logger.Error("GetAll: Trie is nil")
		return nil, errors.New("account state DB has a nil trie")
	}

	allAccounts := make(map[common.Address]types.AccountState)
	// Assume trie.GetAll() returns map[string][]byte or similar
	allData, err := trieToUse.GetAll()
	if err != nil {
		logger.Error("GetAll: Error retrieving data from trie", "error", err)
		return nil, fmt.Errorf("error getting all data from trie: %w", err)
	}

	for addressStr, accountStateBytes := range allData {
		// Need to convert key (likely []byte or string) back to common.Address
		address := common.HexToAddress(addressStr) // Chuyển đổi string sang common.Address

		// Unmarshal the value into an AccountState implementation
		accountState := &state.AccountState{} // Use the concrete type
		err := accountState.Unmarshal(accountStateBytes)
		if err != nil {
			// Consider logging the error and skipping the account?
			logger.Warn("Failed to unmarshal account state during GetAll", "address", address.Hex(), "error", err)
			continue // Skip corrupted data
		}
		allAccounts[address] = accountState
	}
	logger.Debug("GetAll: Retrieved accounts from committed trie state", "count", len(allAccounts))
	return allAccounts, nil
}

// --- Modification Methods ---
// These methods modify an account's state. They first retrieve the state
// (creating it if necessary), modify it, and then mark it as dirty using setDirtyAccountState.

func (db *AccountStateDB) SubPendingBalance(address common.Address, amount *big.Int) error {

	if db.lockedFlag.Load() {
		panic("SubPendingBalance db.lockedFlag is already locked")
		// return errors.New("SubPendingBalance db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SubPendingBalance: %w", err)
	}
	if as == nil {
		return errors.New("SubPendingBalance: account state is nil")
	} // Safety check
	err = as.SubPendingBalance(amount)
	if err != nil {
		return fmt.Errorf("SubPendingBalance: %w", err)
	}
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) RefreshPendingBalance(address common.Address) error {

	if db.lockedFlag.Load() {
		panic("RefreshPendingBalance db.lockedFlag is already locked")
		// return errors.New("RefreshPendingBalance db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("RefreshPendingBalance: %w", err)
	}
	if as == nil {
		return errors.New("RefreshPendingBalance: account state is nil")
	} // Safety check

	pendingBalance := as.PendingBalance()
	// Check if subtraction is necessary/possible
	if pendingBalance != nil && pendingBalance.Sign() > 0 {
		err = as.SubPendingBalance(pendingBalance)
		if err != nil {
			// Log or handle potential underflow if SubPendingBalance can fail
			logger.Warn("RefreshPendingBalance: Error subtracting pending balance", "address", address.Hex(), "error", err)
			// Decide whether to continue or return error based on AccountState logic
			return fmt.Errorf("RefreshPendingBalance: sub pending failed: %w", err)
		}
		as.AddBalance(pendingBalance) // Add the amount back to the main balance
	}
	// No else needed: if pending is zero or nil, nothing to subtract or add

	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) AddPendingBalance(address common.Address, amount *big.Int) error {

	if db.lockedFlag.Load() {
		panic("AddPendingBalance: db.lockedFlag is already locked")

		//return errors.New("AddPendingBalance db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("AddPendingBalance: %w", err)
	}
	if as == nil {
		return errors.New("AddPendingBalance: account state is nil")
	} // Safety check
	as.AddPendingBalance(amount) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) PlusOneNonce(address common.Address) error {

	if db.lockedFlag.Load() {
		panic("PlusOneNonce: db.lockedFlag is already locked")
		//return errors.New("PlusOneNonce db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("PlusOneNonce: %w", err)
	}
	if as == nil {
		return errors.New("PlusOneNonce: account state is nil")
	} // Safety check
	as.PlusOneNonce()
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetAccountType(address common.Address, accountTypeNew pb.ACCOUNT_TYPE) error {

	if db.lockedFlag.Load() {
		panic("SetAccountType: db.lockedFlag is already locked")
		// return errors.New("SetAccountType db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetAccountType: %w", err)
	}
	if as == nil {
		return errors.New("SetAccountType: account state is nil")
	} // Safety check
	err = as.SetAccountType(accountTypeNew) // Assuming this can fail (e.g., invalid type transition)
	if err != nil {
		return fmt.Errorf("SetAccountType: %w", err)
	}
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetPublicKeyBls(address common.Address, publicKeyBls []byte) error {

	if db.lockedFlag.Load() {
		panic("SetPublicKeyBls: db.lockedFlag is already locked")
		//return errors.New("SetPublicKeyBls db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetPublicKeyBls: %w", err)
	}
	if as == nil {
		return errors.New("SetPublicKeyBls: account state is nil")
	} // Safety check
	as.SetPublicKeyBls(publicKeyBls)
	db.setDirtyAccountState(as)
	return nil
}
func (db *AccountStateDB) GetPublicKeyBls(address common.Address) ([]byte, error) {
	if db.lockedFlag.Load() {
		panic("GetPublicKeyBls: db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return nil, fmt.Errorf("GetPublicKeyBls: %w", err)
	}
	if as == nil {
		return nil, errors.New("GetPublicKeyBls: account state is nil")
	} // Safety check
	return as.PublicKeyBls(), nil
}

func (db *AccountStateDB) AddBalance(address common.Address, amount *big.Int) error {

	if db.lockedFlag.Load() {
		panic("AddBalance: db.lockedFlag is already locked")
		// return errors.New("AddBalance db.lockedFlag is already locked")
	}

	if amount == nil || amount.Sign() <= 0 {
		// Adding zero or negative is a no-op or potentially an error depending on semantics
		// logger.Debug("AddBalance: Attempted to add zero or negative amount", "address", address.Hex(), "amount", amount)
		return nil // Or return an error if negative amounts are invalid
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("AddBalance: %w", err)
	}
	if as == nil {
		return errors.New("AddBalance: account state is nil")
	} // Safety check
	as.AddBalance(amount) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SubBalance(address common.Address, amount *big.Int) error {

	if db.lockedFlag.Load() {
		panic("SubBalance: db.lockedFlag is already locked")
		// return errors.New("SubBalance db.lockedFlag is already locked")
	}
	if amount == nil || amount.Sign() <= 0 {
		// Subtracting zero or negative is a no-op or potentially an error
		// logger.Debug("SubBalance: Attempted to subtract zero or negative amount", "address", address.Hex(), "amount", amount)
		return nil // Or return an error if negative amounts are invalid
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SubBalance: %w", err)
	}
	if as == nil {
		return errors.New("SubBalance: account state is nil")
	} // Safety check
	err = as.SubBalance(amount) // This can fail (insufficient funds)
	if err != nil {
		return fmt.Errorf("SubBalance: %w", err)
	}
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SubTotalBalance(address common.Address, amount *big.Int) error {

	if db.lockedFlag.Load() {
		panic("SubTotalBalance: db.lockedFlag is already locked")
		// return errors.New("SubTotalBalance db.lockedFlag is already locked")
	}
	if amount == nil || amount.Sign() <= 0 {
		return nil // Or error
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SubTotalBalance: %w", err)
	}
	if as == nil {
		return errors.New("SubTotalBalance: account state is nil")
	} // Safety check
	err = as.SubTotalBalance(amount) // This can fail
	if err != nil {
		return fmt.Errorf("SubTotalBalance: %w", err)
	}
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetNonce(address common.Address, nonce uint64) error {

	if db.lockedFlag.Load() {
		panic("SetNonce: db.lockedFlag is already locked")
		// return errors.New("SetNonce db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetNonce: %w", err)
	}
	if as == nil {
		return errors.New("SetNonce: account state is nil")
	} // Safety check
	as.SetNonce(nonce) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetLastHash(address common.Address, hash common.Hash) error {

	if db.lockedFlag.Load() {
		panic("SetLastHash: db.lockedFlag is already locked")
		// return errors.New("SetLastHash db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetLastHash: %w", err)
	}
	if as == nil {
		return errors.New("SetLastHash: account state is nil")
	} // Safety check
	as.SetLastHash(hash) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) GetLastHash(address common.Address) (common.Hash, error) {

	if db.lockedFlag.Load() {
		panic("SetLastHash: db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return common.Hash{}, fmt.Errorf("SetLastHash: %w", err)
	}
	if as == nil {
		return common.Hash{}, errors.New("SetLastHash: account state is nil")
	} // Safety check
	return as.LastHash(), nil
}

func (db *AccountStateDB) SetNewDeviceKey(address common.Address, newDeviceKey common.Hash) error {

	if db.lockedFlag.Load() {
		panic("SetNewDeviceKey: db.lockedFlag is already locked")
		// return errors.New("SetNewDeviceKey db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetNewDeviceKey: %w", err)
	}
	if as == nil {
		return errors.New("SetNewDeviceKey: account state is nil")
	} // Safety check
	as.SetNewDeviceKey(newDeviceKey) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

// SetState explicitly sets an account state, marking it dirty.
// Useful for initializing or overwriting an account state directly.
func (db *AccountStateDB) SetState(as types.AccountState) {
	if as == nil {
		logger.Warn("SetState: Attempted to set nil account state")
		return
	}
	// Use the public setter which correctly uses the internal setDirtyAccountState
	db.setDirtyAccountState(as)
}

// --- Smart Contract State Methods ---

func (db *AccountStateDB) SetCreatorPublicKey(
	address common.Address,
	creatorPublicKey p_common.PublicKey,
) error {

	if db.lockedFlag.Load() {
		panic("SetCreatorPublicKey: db.lockedFlag is already locked")
		// return errors.New("SetCreatorPublicKey db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetCreatorPublicKey: %w", err)
	}
	if as == nil {
		return errors.New("SetCreatorPublicKey: account state is nil")
	} // Safety check
	as.SetCreatorPublicKey(creatorPublicKey) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetCodeHash(address common.Address, codeHash common.Hash) error {

	if db.lockedFlag.Load() {
		panic("SetCodeHash: db.lockedFlag is already locked")
		// return errors.New("SetCodeHash db.lockedFlag is already locked")
	}

	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetCodeHash: %w", err)
	}
	if as == nil {
		return errors.New("SetCodeHash: account state is nil")
	} // Safety check
	as.SetCodeHash(codeHash) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetStorageRoot(address common.Address, storageRoot common.Hash) error {

	if db.lockedFlag.Load() {
		panic("SetStorageRoot: db.lockedFlag is already locked")
		// return errors.New("SetStorageRoot db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetStorageRoot: %w", err)
	}
	if as == nil {
		return errors.New("SetStorageRoot: account state is nil")
	} // Safety check
	as.SetStorageRoot(storageRoot) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) SetStorageAddress(
	address common.Address,
	storageAddress common.Address,
) error {

	if db.lockedFlag.Load() {
		panic("SetStorageAddress: db.lockedFlag is already locked")
		// return errors.New("SetStorageAddress db.lockedFlag is already locked")
	}
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("SetStorageAddress: %w", err)
	}
	if as == nil {
		return errors.New("SetStorageAddress: account state is nil")
	} // Safety check
	as.SetStorageAddress(storageAddress) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

func (db *AccountStateDB) AddLogHash(address common.Address, logsHash common.Hash) error {
	as, err := db.getOrCreateAccountState(address)
	if err != nil {
		return fmt.Errorf("AddLogHash: %w", err)
	}
	if as == nil {
		return errors.New("AddLogHash: account state is nil")
	} // Safety check
	as.AddLogHash(logsHash) // Assuming this cannot fail
	db.setDirtyAccountState(as)
	return nil
}

// --- State Management Methods ---

// Discard reverts all uncommitted changes by clearing the dirty cache
// and reloading the trie from the last committed state (originRootHash).
func (db *AccountStateDB) Discard() (err error) {

	if db.lockedFlag.Load() {
		panic("Discard: db.lockedFlag is already locked")

		// return errors.New("Discard db.lockedFlag is already locked")
	}
	// Clear dirty accounts first
	db.dirtyAccounts = sync.Map{}

	// Reload trie from the original hash
	originHash := db.originRootHash
	currentDb := db.db // Use the existing db instance

	// Check if db is nil before using it
	if currentDb == nil {
		logger.Error("Discard: Database instance is nil")
		return errors.New("cannot discard, database instance is nil")
	}

	db.trie, err = p_trie.New(originHash, currentDb, true)
	if err != nil {
		logger.Error("Discard: Failed to reload trie from origin hash", "hash", originHash, "error", err)
		// State is now potentially inconsistent. The dirty map is cleared, but the trie failed to load.
		// This is a critical state.
		return fmt.Errorf("failed to reload trie to %s after discard: %w", originHash, err)
	}

	logger.Info("Discard successful, reverted to root hash", "hash", originHash)
	return nil
}

// SetAccountBatch stores a serialized batch of account data, typically used for network transfer.
func (db *AccountStateDB) SetAccountBatch(batch []byte) {
	// If this can be called concurrently with GetAccountBatch or Commit, it needs locking.
	db.accountBatch = batch
}

// GetAccountBatch retrieves and clears the stored account batch.
func (db *AccountStateDB) GetAccountBatch() []byte {
	// If this can be called concurrently with SetAccountBatch or Commit, it needs locking.
	batch := db.accountBatch
	db.accountBatch = nil // Clear after retrieval
	return batch
}

// Commit persists all dirty account states to the trie and the underlying database.
// It calculates the new root hash and updates the state database instance.
func (db *AccountStateDB) Commit() (common.Hash, error) {

	// 1. Apply dirty changes to the in-memory trie representation
	// IntermediateRoot handles locking muStruct for iterating and clearing dirtyAccounts.
	// intermediateHash, err := db.IntermediateRoot(false) // isLog = false
	// defer func() {
	// 	db.lockedFlag.Store(false) // CHANGED: Use atomic Store
	// }()
	// if err != nil {
	// 	logger.Error("Commit: Error applying changes during IntermediateRoot", "error", err)
	// 	// Attempt to discard changes to revert to the last known good state?
	// 	// Discard() acquires muStruct, which is fine as IntermediateRoot released it.
	// 	// However, discard might fail too.
	// 	// For now, return the error directly.
	// 	return common.Hash{}, fmt.Errorf("commit failed during IntermediateRoot: %w", err)
	// }

	if !db.lockedFlag.Load() {
		panic("Commit: db.lockedFlag is not already locked")
	}
	// Lock the entire commit process to ensure atomicity
	db.muCommit.Lock()
	defer db.muCommit.Unlock()

	// 1. Apply dirty changes to the in-memory trie representation
	// IntermediateRoot handles locking muStruct for iterating and clearing dirtyAccounts.
	intermediateHash, err := db.IntermediateRoot(false) // isLog = false
	defer func() {
		db.lockedFlag.Store(false) // CHANGED: Use atomic Store
	}()
	if err != nil {
		logger.Error("Commit: Error applying changes during IntermediateRoot", "error", err)
		// Attempt to discard changes to revert to the last known good state?
		// Discard() acquires muStruct, which is fine as IntermediateRoot released it.
		// However, discard might fail too.
		// For now, return the error directly.
		return common.Hash{}, fmt.Errorf("commit failed during IntermediateRoot: %w", err)
	}

	// At this point, db.trie (in memory) reflects the state matching intermediateHash,
	// and db.dirtyAccounts has been cleared.

	// 2. Commit the in-memory trie to generate database nodes
	// We might need a copy if trie.Commit modifies the receiver, but let's assume
	// it calculates nodes based on the current state without modifying db.trie directly,
	// OR that modification is acceptable because we replace db.trie later anyway.
	// Let's proceed without Copy() for now, assuming Commit calculates based on current state.
	// The 'true' argument generates the node set needed for persistence.
	committedHash, nodeSet, oldKeys, err := db.trie.Commit(true)
	if err != nil {
		logger.Error("Commit: Error during trie Commit calculation", "error", err)
		// The in-memory trie might be in a partially committed state internally.
		// Attempting to discard might be the best recovery.
		return common.Hash{}, fmt.Errorf("trie Commit calculation failed: %w", err)
	}

	// Sanity check: Hash from applying updates should match hash from commit calculation.
	if intermediateHash != committedHash {
		logger.Error("Commit: Root hash mismatch between IntermediateRoot and Commit calculation",
			"intermediate", intermediateHash, "commit", committedHash)
		// This indicates a potential bug in the trie implementation or the commit logic.
		// State is inconsistent.
		return common.Hash{}, fmt.Errorf(
			"root hash mismatch after commit calculation (intermediate: %s, commit: %s)",
			intermediateHash, committedHash,
		)
	}
	finalHash := committedHash // Use the hash from the successful commit calculation

	// 3. Handle old keys (optional, e.g., delete from archive DB)
	// Currently commented out in original logic.
	if len(oldKeys) > 0 {
		logger.Debug("Commit: Identified old keys to potentially prune", "count", len(oldKeys))
		// Example:
		// pruneBatch := db.db.NewBatch()
		// for _, key := range oldKeys {
		//     pruneBatch.Delete(key)
		// }
		// if err := pruneBatch.Write(); err != nil {
		//     logger.Error("Commit: Failed to prune old keys from DB", "error", err)
		//     // This might not be fatal for the commit itself, but indicates pruning issues.
		// }
	}

	// 4. Persist the new trie nodes to the database
	if nodeSet != nil && len(nodeSet.Nodes) > 0 {
		batch := make([][2][]byte, 0, len(nodeSet.Nodes))
		for _, node := range nodeSet.Nodes {
			if node.Hash == (common.Hash{}) {
				logger.Error("Commit: Trying to save node with empty hash, skipping.")
				// This should ideally not happen with a correct trie implementation.
				continue
			}
			batch = append(batch, [2][]byte{node.Hash.Bytes(), node.Blob})
		}

		// Prepare batch for network transfer if needed (e.g., master node role)
		// Needs config check and node != nil
		if config.ConfigApp != nil && config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			if len(batch) > 0 { // Only serialize if there's data
				data, err := storage.SerializeBatch(batch)
				if err != nil {
					// Log error but likely continue the commit, as local persistence is key.
					logger.Error("Commit: Failed to serialize batch for network transfer", "error", err)
				} else {
					db.SetAccountBatch(data)
					logger.Debug("Commit: Serialized account batch for network transfer", "size_bytes", len(data))
				}
			}
		}

		// Write batch to persistent storage
		if len(batch) > 0 { // Only write if there are nodes to write
			logger.Debug("Commit: Writing batch to DB", "num_nodes", len(batch))
			err := db.db.BatchPut(batch)
			if err != nil {
				logger.Error("Commit: Error during DB BatchPut", "error", err)
				// Failed to save nodes, the DB state does not match the calculated finalHash.
				// This is a critical failure. The in-memory trie might be based on finalHash,
				// but the DB doesn't reflect it.
				return common.Hash{}, fmt.Errorf("DB BatchPut failed: %w", err)
			}
		} else {
			logger.Debug("Commit: No new nodes generated by trie commit.")
		}

	} else {
		logger.Debug("Commit: No new nodes to write to DB (nodeSet is nil or empty)")
	}

	// 5. Create a *new* trie instance reflecting the committed state.
	// This ensures the live 'db.trie' points to a trie instance consistent
	// with the state in the database identified by 'finalHash'.
	// The old 'db.trie' instance might have internal state modified by Commit().
	newTrie, err := p_trie.New(finalHash, db.db, true)
	if err != nil {
		// Release struct lock before returning error
		logger.Error("Commit: Failed to create new trie instance after DB write", "hash", finalHash, "error", err)
		// DB state likely matches finalHash, but in-memory state is inconsistent. Critical error.
		return common.Hash{}, fmt.Errorf("failed to load trie for new root %s after commit: %w", finalHash, err)
	}

	// 6. Update the live trie reference and origin hash
	db.trie = newTrie
	db.originRootHash = finalHash

	// --- Release structural lock ---
	return finalHash, nil
}

// --- Hàm với logging chi tiết ---
func (db *AccountStateDB) IntermediateRoot(isLockProcess ...bool) (common.Hash, error) {
	var lockProcess bool
	if len(isLockProcess) > 0 {
		lockProcess = isLockProcess[0]
	} else {
		lockProcess = true // Gán mặc định là true
	}
	// Nếu isLockProcess là true và mutex đang bị lock, báo lỗi
	if lockProcess {
		// Thử lock mutex, nếu mutex đã bị lock thì sẽ báo lỗi
		if db.lockedFlag.Load() {
			errMsg := "IntermediateRoot (lockProcess=true): db.lockedFlag is already locked"
			log.Println("error:", errMsg)
			return common.Hash{}, errors.New(errMsg)
		}
		db.lockedFlag.Store(true) // CHANGED: Use atomic Store
		// Lock mutex thành công
		logger.Debug("Structure lock acquired, processing locked")
	} else {

		if !db.lockedFlag.Load() {
			errMsg := "error: db.lockedFlag is not locked"
			log.Println(errMsg)
			return common.Hash{}, errors.New(errMsg)
		}
		defer func() {
			db.dirtyAccounts = sync.Map{}
			db.lockedFlag.Store(false) // CHANGED: Use atomic Store
			logger.Debug("Structure lock released, processing unlocked")
		}()
	}

	if db.trie == nil {
		logger.Error("Trie is nil, cannot proceed")
		return common.Hash{}, errors.New("cannot calculate intermediate root, trie is nil")
	}

	logger.Debug("Initial state", "originRootHash", db.originRootHash)

	var (
		updateErr     error
		processedKeys int  = 0
		hasChanges    bool = false
		keysToProcess []common.Address
	)

	cloned := make(map[common.Address]types.AccountState)

	// Bước 1: Thu thập keys (vì sync.Map.Range không cho biết trước số lượng)
	db.dirtyAccounts.Range(func(key, value interface{}) bool {
		address, ok1 := key.(common.Address)
		state, ok2 := value.(types.AccountState)
		if !ok1 || !ok2 || state == nil {
			logger.Warn("Skipping invalid entry in dirtyAccounts", "keyType", fmt.Sprintf("%T", key), "valueType", fmt.Sprintf("%T", value))
			return true // continue
		}

		cloned[address] = state
		keysToProcess = append(keysToProcess, address)
		return true
	})

	totalDirty := len(keysToProcess)
	logger.Debug("Starting update from dirtyAccounts", "count", totalDirty)

	if totalDirty > 0 {
		hasChanges = true
	}
	// Bước 2: Iterate over dirty accounts and update the in-memory trie
	for processedKeys, address := range keysToProcess {
		currentKeyIndex := processedKeys + 1

		logCtx := []interface{}{"address", address.Hex(), "progress", fmt.Sprintf("%d/%d", currentKeyIndex, totalDirty)}
		logger.Debug("Processing dirty account", logCtx...)

		as, ok := cloned[address]
		if !ok {
			errMsg := fmt.Sprintf("missing account state for address %s in cloned map", address.Hex())
			logger.Error("Stopping iteration due to missing account state", logCtx...)
			updateErr = errors.New(errMsg)
			break
		}
		if as == nil {
			errMsg := fmt.Sprintf("nil value for address %s in dirtyAccounts", address.Hex())
			logger.Error("Stopping iteration due to nil account state value", logCtx...)
			updateErr = errors.New(errMsg)
			break
		}

		b, err := as.Marshal()
		if err != nil {
			logger.Error("Stopping iteration due to Marshal error", append(logCtx, "error", err)...)
			updateErr = fmt.Errorf("marshal error for %s: %w", address.Hex(), err)
			break
		}

		logger.Debug("Updating trie with marshalled data", append(logCtx, "as", as)...)

		if err := db.trie.Update(address.Bytes(), b); err != nil {
			logger.Error("Stopping iteration due to Trie update error", append(logCtx, "error", err)...)
			updateErr = fmt.Errorf("trie update error for %s: %w", address.Hex(), err)
			break
		}

		logger.Debug("Trie update successful for address", logCtx...)
	}

	if updateErr != nil {
		logger.Error("Failed during dirtyAccounts update loop, clearing dirty map", "error", updateErr, "processedBeforeError", processedKeys)
		logger.Debug("Clearing dirtyAccounts map after error")
		return common.Hash{}, updateErr
	}

	logger.Debug("Finished processing dirtyAccounts", "processedTotal", processedKeys)

	var newHash common.Hash
	if hasChanges {
		logger.Debug("Changes detected, calculating new trie hash...", "originRootHash", db.originRootHash)
		newHash = db.trie.Hash()
		logger.Debug("Calculated new intermediate hash", "newHash", newHash, "processedKeys", processedKeys)

		if newHash == db.originRootHash {
			logger.Warn("Trie hash calculated but matches originRootHash despite processing changes", "hash", newHash, "processedKeys", processedKeys)
		}

		// ✅ Cập nhật originRootHash sau khi tính hash mới
		db.originRootHash = newHash
	} else {
		newHash = db.trie.Hash()
		db.originRootHash = newHash
		logger.Debug("No changes detected in dirtyAccounts, intermediate hash remains origin hash", "hash", newHash)
	}

	logger.Debug("Clearing dirtyAccounts map after successful processing")

	// logger.Info("Intermediate root calculation finished", "finalHash", newHash)
	return newHash, nil
}

// setDirtyAccountState stores the account state in the concurrent map.
// This relies on sync.Map's internal safety for concurrent Store calls on potentially different keys.
// No external lock needed here for the Store operation itself.
func (db *AccountStateDB) setDirtyAccountState(as types.AccountState) {
	if as == nil {
		logger.Warn("setDirtyAccountState: Attempted to store nil account state")
		return // Avoid storing nil interface
	}
	if db.lockedFlag.Load() {
		panic("Error: setDirtyAccountState db.lockedFlag is already locked!")
	}
	db.dirtyAccounts.Store(as.Address(), as)
	// logger.Trace("Marked account dirty", "address", as.Address().Hex()) // Optional detailed logging
}

// PublicSetDirtyAccountState provides a public way to mark an account as dirty.
// Relies on sync.Map's internal safety.
func (db *AccountStateDB) PublicSetDirtyAccountState(as types.AccountState) {
	// Delegates to the internal, potentially traced, version
	db.setDirtyAccountState(as)
}

// getOrCreateAccountState retrieves an account state, optimized for concurrency.
// It first checks the dirty cache (sync.Map). If not found (cache miss),
// it reads from the underlying trie. If not in the trie, it creates a new state.
// The retrieved or newly created state is then stored back into the dirty cache *only on cache miss*.
// Uses RLock on muStruct to allow concurrent reads/stores via sync.Map API
// while preventing structural changes (reassignment of dirtyAccounts or trie) concurrently.
func (db *AccountStateDB) getOrCreateAccountState(
	address common.Address,
) (types.AccountState, error) {
	// Kiểm tra db nil để tránh panic
	if db == nil {
		return nil, errors.New("getOrCreateAccountState called on nil AccountStateDB")
	}

	// --- Khóa Đọc muStruct ---
	// Bảo vệ chống lại việc db.dirtyAccounts hoặc db.trie bị thay thế đồng thời
	// bởi ReloadTrie, Discard, Commit, CopyFrom. Cho phép hàm này thực thi đồng thời.

	// --- Đã có Khóa Đọc ---

	// 1. Kiểm tra cache dirty (sync.Map.Load an toàn cho đọc đồng thời)
	value, ok := db.dirtyAccounts.Load(address)
	if ok {
		// Cache hit
		accountState, valid := value.(types.AccountState)
		if valid && accountState != nil {
			// Tìm thấy state hợp lệ, không nil trong cache. Trả về ngay.
			// logger.Trace("getOrCreateAccountState: Cache hit", "address", address.Hex())
			return accountState, nil
		}
		// Xử lý kiểu không hợp lệ hoặc giá trị nil tìm thấy trong cache (không nên xảy ra nếu dùng đúng)
		if !valid {
			logger.Error("getOrCreateAccountState: Invalid type found in dirty cache map", "address", address.Hex(), "type", fmt.Sprintf("%T", value))
			// Fallthrough để đọc từ trie, có thể ghi đè mục cache xấu sau này.
		}
		if accountState == nil {
			logger.Error("getOrCreateAccountState: Found nil account state in dirty cache map", "address", address.Hex())
			// Fallthrough để đọc từ trie, có thể ghi đè mục cache xấu sau này.
		}
		// Nếu fallthrough ở đây, 'ok' là true, nhưng dữ liệu xấu. Tiến hành như cache miss.
	}

	// --- Cache miss hoặc mục cache không hợp lệ ---
	// logger.Trace("getOrCreateAccountState: Cache miss", "address", address.Hex())

	// Đảm bảo trie có thể truy cập (kiểm tra được bảo vệ bởi RLock)
	trieToUse := db.trie
	if trieToUse == nil {
		logger.Error("getOrCreateAccountState: Trie is nil during cache miss lookup", "address", address.Hex())
		return nil, errors.New("account state DB has a nil trie")
	}

	// 2. Lấy từ Trie cơ sở (trie.Get nên an toàn cho đọc đồng thời)
	// Việc này vẫn cần thiết để chuẩn bị dữ liệu cho LoadOrStore nếu cache miss
	bData, err := trieToUse.Get(address.Bytes())
	if err != nil {
		// Ghi log lỗi nhưng trả về lỗi để caller xử lý
		logger.Error("getOrCreateAccountState: Error getting data from Trie", "address", address.Hex(), "error", err, "originRootHash %v", db.originRootHash)
		return nil, fmt.Errorf("error getting %s from Trie: %w", address.Hex(), err)
	}

	// Biến tạm để giữ state sẽ được cung cấp cho LoadOrStore
	var stateToPotentiallyStore types.AccountState

	// 3. Nếu không tìm thấy trong Trie, tạo account state mới
	if len(bData) == 0 {
		// Sử dụng constructor của implementation cụ thể
		newState := state.NewAccountState(address)
		if newState == nil {
			// Constructor không nên trả về nil
			logger.Error("getOrCreateAccountState: NewAccountState returned nil", "address", address.Hex())
			return nil, fmt.Errorf("failed to create new account state for %s", address.Hex())
		}
		logger.Debug("getOrCreateAccountState: Prepared new AccountState (not found in trie)", "address", address.Hex())
		stateToPotentiallyStore = newState
	} else {
		// 4. Nếu tìm thấy trong Trie, unmarshal dữ liệu
		loadedAs := &state.AccountState{} // Tạo instance của kiểu cụ thể để unmarshal vào
		err = loadedAs.Unmarshal(bData)
		if err != nil {
			logger.Error("getOrCreateAccountState: Error unmarshalling account state from Trie", "address", address.Hex(), "error", err)
			return nil, fmt.Errorf("error unmarshalling %s from Trie: %w", address.Hex(), err)
		}
		stateToPotentiallyStore = loadedAs // Gán state đã unmarshal thành công
		logger.Debug("getOrCreateAccountState: Loaded AccountState from Trie", "address", address.Hex())
	}
	if db.lockedFlag.Load() {
		return stateToPotentiallyStore, nil
	} else {
		actualValue, loaded := db.dirtyAccounts.LoadOrStore(address, stateToPotentiallyStore)

		finalAs, castOk := actualValue.(types.AccountState)
		if !castOk || finalAs == nil {

			logger.Error("getOrCreateAccountState: Invalid type/nil found in cache via LoadOrStore",
				"address", address.Hex(),
				"type", fmt.Sprintf("%T", actualValue),
				"is_nil", actualValue == nil,
				"loaded_flag", loaded)
			return nil, fmt.Errorf("invalid type or nil value found/stored in cache for %s", address.Hex())
		}
		return finalAs, nil
	}

}

// Storage returns the underlying storage instance.
func (db *AccountStateDB) Storage() storage.Storage {
	// Accessing db.db might need protection if it could be reassigned,
	// but currently only happens in New/CopyFrom. Assume read is safe.
	return db.db
}

// CopyFrom copies the state (trie reference, origin hash, dirty accounts)
// from another AccountStateDB (source) to this one (destination).
// Requires locking both source and destination structures to ensure atomicity
// and prevent races during the copy process.
func (db *AccountStateDB) CopyFrom(sourceDB types.AccountStateDB) error {

	if db == nil {
		return errors.New("CopyFrom called on nil destination AccountStateDB")
	}

	// Type assert to access implementation details (mutexes, specific fields).
	// This restricts CopyFrom to work only between *AccountStateDB instances.
	asDB, ok := sourceDB.(*AccountStateDB)
	if !ok {
		return errors.New("CopyFrom requires the source to be an *AccountStateDB instance")
	}
	if asDB == nil {
		return errors.New("CopyFrom called with nil source AccountStateDB")
	}

	// Avoid self-copy, which would cause deadlock on locks.
	if db == asDB {
		return errors.New("cannot CopyFrom self")
	}

	if db.lockedFlag.Load() {
		// return errors.New("CopyFrom db.lockedFlag is already locked")
		panic("CopyFrom: db.lockedFlag is already locked")
	}

	// --- Snapshot source dirty accounts while holding source lock ---
	tempDirty := make(map[common.Address]types.AccountState) // Use concrete types for map
	copyErr := false                                         // Flag for errors during range/copy

	asDB.dirtyAccounts.Range(func(key, value interface{}) bool {
		addr, okKey := key.(common.Address)
		stateVal, okVal := value.(types.AccountState)

		if !okKey || !okVal || stateVal == nil {
			logger.Error("CopyFrom: Invalid entry found in source dirtyAccounts map during copy",
				"key_type", fmt.Sprintf("%T", key), "value_type", fmt.Sprintf("%T", value))
			// Skip this entry or abort? Aborting seems safer.
			copyErr = true
			return false // Stop ranging
		}

		// *** Potential Deep Copy Needed Here ***
		// If AccountState contains mutable fields (slices, maps, pointers to mutable data),
		// a shallow copy (just assigning stateVal) means both source and destination
		// DBs will share the *same underlying mutable object*. Modifications in one
		// after the copy will affect the other, which is usually NOT desired.
		// A deep copy mechanism is needed if AccountState is not immutable.
		// Example using an interface (adapt as needed):
		// if copier, ok := stateVal.(interface{ DeepCopy() types.AccountState }); ok {
		//     tempDirty[addr] = copier.DeepCopy()
		// } else {
		//     logger.Warn("CopyFrom: AccountState does not implement DeepCopy, performing shallow copy.", "address", addr.Hex())
		//	   tempDirty[addr] = stateVal // Fallback to shallow copy
		// }
		tempDirty[addr] = stateVal // Current: Performing shallow copy - review if AccountState is mutable!

		return true
	})

	if copyErr {
		// Locks will be released by defers
		return errors.New("error encountered while copying dirty account entries")
	}

	// --- Copy other relevant fields from source *while holding both locks* ---
	// This prevents races where source fields might change after source unlock
	// but before destination assignment.

	// Copying the trie reference is potentially risky if the source trie can be modified
	// externally *after* this copy operation but *before* the destination uses it.
	// A full trie copy (e.g., `asDB.trie.Copy()`) might be needed for true isolation,
	// but that can be expensive. Copying the reference assumes the source trie state
	// corresponding to sourceOriginHash is effectively immutable or won't be changed
	// in a way that breaks the destination before its next commit/reload.
	sourceTrie := asDB.trie
	sourceOriginHash := asDB.originRootHash
	sourceDb := asDB.db // Assume DB storage instance reference is safe to share/copy

	// --- Apply changes to destination DB (already locked) ---
	db.trie = sourceTrie // Reference copy - be aware of implications
	db.originRootHash = sourceOriginHash
	db.db = sourceDb

	// Replace destination dirty map with the snapshot
	db.dirtyAccounts = sync.Map{} // Start with a fresh map for the destination
	for key, value := range tempDirty {
		db.dirtyAccounts.Store(key, value) // Populate the new map
	}

	logger.Info("CopyFrom completed successfully")
	// Locks will be released by defers
	return nil
}

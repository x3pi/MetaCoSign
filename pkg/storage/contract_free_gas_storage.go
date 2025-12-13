package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/ldb_storage"
	"github.com/syndtr/goleveldb/leveldb"
)

// Key prefixes for LevelDB
const (
	// Reverse index: cfg:<contract_address> -> paddedId
	PREFIX_CONTRACT_FREE_GAS_INDEX = "cfg:"
	// Contract data: cfgd:<paddedId> -> JSON {address, added_at}
	PREFIX_CONTRACT_FREE_GAS_DATA = "cfgd:"
	// Count: cfgc:count -> total count
	PREFIX_CONTRACT_FREE_GAS_COUNT = "cfgc:count"

	CONTRACT_ID_PADDING_LENGTH = 10 // Hỗ trợ tối đa 10 tỷ contracts
)

// ContractFreeGasData - struct đơn giản để lưu thông tin contract
type ContractFreeGasData struct {
	ContractAddress string `json:"contract_address"`
	AddedAt         int64  `json:"added_at"`
}

type ContractFreeGasStorage struct {
	ldb *ldb_storage.LevelDBStorage
	mu  sync.RWMutex
}

func NewContractFreeGasStorage(ldb *ldb_storage.LevelDBStorage) *ContractFreeGasStorage {
	return &ContractFreeGasStorage{ldb: ldb}
}

// ========== HELPER FUNCTIONS ==========
func padContractID(id uint64) string {
	idStr := strconv.FormatUint(id, 10)
	if len(idStr) >= CONTRACT_ID_PADDING_LENGTH {
		return idStr
	}
	return strings.Repeat("0", CONTRACT_ID_PADDING_LENGTH-len(idStr)) + idStr
}

func unpadContractID(paddedID string) (uint64, error) {
	return strconv.ParseUint(paddedID, 10, 64)
}

// ========== COUNT OPERATIONS ==========
func (s *ContractFreeGasStorage) getCount() (uint64, error) {
	value, err := s.ldb.Get([]byte(PREFIX_CONTRACT_FREE_GAS_COUNT))
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}

func (s *ContractFreeGasStorage) setCount(count uint64) error {
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, count)
	return s.ldb.Put([]byte(PREFIX_CONTRACT_FREE_GAS_COUNT), value)
}

// ========== REVERSE INDEX OPERATIONS ==========
func (s *ContractFreeGasStorage) getReverseIndex(contractAddress ethCommon.Address) (paddedID string, exists bool, err error) {
	key := PREFIX_CONTRACT_FREE_GAS_INDEX + contractAddress.Hex()
	value, err := s.ldb.Get([]byte(key))
	if err != nil {
		if err == leveldb.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return string(value), true, nil
}

func (s *ContractFreeGasStorage) saveReverseIndex(contractAddress ethCommon.Address, paddedID string) error {
	key := PREFIX_CONTRACT_FREE_GAS_INDEX + contractAddress.Hex()
	return s.ldb.Put([]byte(key), []byte(paddedID))
}

func (s *ContractFreeGasStorage) deleteReverseIndex(contractAddress ethCommon.Address) error {
	key := PREFIX_CONTRACT_FREE_GAS_INDEX + contractAddress.Hex()
	return s.ldb.Delete([]byte(key))
}

// ========== ADD CONTRACT ==========
func (s *ContractFreeGasStorage) AddContract(contractAddress ethCommon.Address) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Kiểm tra xem contract đã tồn tại chưa
	_, exists, err := s.getReverseIndex(contractAddress)
	if err != nil {
		return fmt.Errorf("failed to check reverse index: %w", err)
	}
	if exists {
		return fmt.Errorf("contract %s already exists", contractAddress.Hex())
	}

	// Lấy count hiện tại
	count, err := s.getCount()
	if err != nil {
		return fmt.Errorf("failed to get count: %w", err)
	}

	// Thêm contract mới với ID = count + 1
	newID := count + 1
	paddedID := padContractID(newID)

	// Tạo contract data
	contractData := &ContractFreeGasData{
		ContractAddress: contractAddress.Hex(),
		AddedAt:         time.Now().Unix(),
	}

	// Marshal contract data thành JSON
	contractValue, err := json.Marshal(contractData)
	if err != nil {
		return fmt.Errorf("failed to marshal contract data: %w", err)
	}

	// Prepare count value
	countValue := make([]byte, 8)
	binary.BigEndian.PutUint64(countValue, newID)

	// ========== BATCH WRITE (ATOMIC) ==========
	batch := new(leveldb.Batch)

	// 1. Write contract data
	dataKey := PREFIX_CONTRACT_FREE_GAS_DATA + paddedID
	batch.Put([]byte(dataKey), contractValue)

	// 2. Write reverse index
	indexKey := PREFIX_CONTRACT_FREE_GAS_INDEX + contractAddress.Hex()
	batch.Put([]byte(indexKey), []byte(paddedID))

	// 3. Update count
	batch.Put([]byte(PREFIX_CONTRACT_FREE_GAS_COUNT), countValue)

	// Write batch atomically
	if err := s.ldb.WriteBatch(batch, nil); err != nil {
		return fmt.Errorf("failed to write batch: %w", err)
	}

	return nil
}

// ========== REMOVE CONTRACT ==========
func (s *ContractFreeGasStorage) RemoveContract(contractAddress ethCommon.Address) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Lấy paddedID từ reverse index
	paddedID, exists, err := s.getReverseIndex(contractAddress)
	if err != nil {
		return fmt.Errorf("failed to get reverse index: %w", err)
	}
	if !exists {
		return fmt.Errorf("contract %s not found", contractAddress.Hex())
	}

	// Parse paddedID to get foundID
	foundID, err := unpadContractID(paddedID)
	if err != nil {
		return fmt.Errorf("failed to parse paddedID: %w", err)
	}

	// Lấy count
	count, err := s.getCount()
	if err != nil {
		return fmt.Errorf("failed to get count: %w", err)
	}

	// ========== BATCH DELETE/SWAP (ATOMIC) ==========
	batch := new(leveldb.Batch)

	// 1. Xóa reverse index cho contract hiện tại
	indexKey := PREFIX_CONTRACT_FREE_GAS_INDEX + contractAddress.Hex()
	batch.Delete([]byte(indexKey))

	// Strategy: swap với entry cuối rồi xóa entry cuối
	if foundID == count {
		// Nếu xóa entry cuối thì chỉ cần xóa entry đó
		dataKey := PREFIX_CONTRACT_FREE_GAS_DATA + paddedID
		batch.Delete([]byte(dataKey))
	} else {
		// 2. Lấy contract data ở vị trí cuối
		lastPaddedID := padContractID(count)
		lastDataKey := PREFIX_CONTRACT_FREE_GAS_DATA + lastPaddedID
		lastContractValue, err := s.ldb.Get([]byte(lastDataKey))
		if err != nil {
			return fmt.Errorf("failed to get last contract: %w", err)
		}

		// 3. Parse last contract data để update reverse index
		lastContractData := &ContractFreeGasData{}
		if err := json.Unmarshal(lastContractValue, lastContractData); err != nil {
			return fmt.Errorf("failed to unmarshal last contract: %w", err)
		}

		lastContractAddress := ethCommon.HexToAddress(lastContractData.ContractAddress)

		// 4. Swap: Move last contract to foundID position
		dataKey := PREFIX_CONTRACT_FREE_GAS_DATA + paddedID
		batch.Put([]byte(dataKey), lastContractValue)

		// 5. Update reverse index for swapped contract
		lastIndexKey := PREFIX_CONTRACT_FREE_GAS_INDEX + lastContractAddress.Hex()
		batch.Put([]byte(lastIndexKey), []byte(paddedID))

		// 6. Xóa entry cuối
		batch.Delete([]byte(lastDataKey))
	}

	// 7. Giảm count
	newCount := count - 1
	countValue := make([]byte, 8)
	binary.BigEndian.PutUint64(countValue, newCount)
	batch.Put([]byte(PREFIX_CONTRACT_FREE_GAS_COUNT), countValue)

	// Write batch atomically
	if err := s.ldb.WriteBatch(batch, nil); err != nil {
		return fmt.Errorf("failed to write batch: %w", err)
	}

	return nil
}

// ========== GET CONTRACTS WITH PAGINATION ==========
func (s *ContractFreeGasStorage) GetContracts(page, pageSize int) ([]*ContractFreeGasData, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Lấy tổng số contracts
	count, err := s.getCount()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get count: %w", err)
	}

	total := int(count)

	// Tính toán range để lấy
	startID := uint64(page*pageSize + 1)
	endID := uint64((page + 1) * pageSize)

	if startID > count {
		return []*ContractFreeGasData{}, total, nil
	}
	if endID > count {
		endID = count
	}

	// Lấy contract data trong range
	contracts := make([]*ContractFreeGasData, 0, int(endID-startID+1))
	for id := startID; id <= endID; id++ {
		paddedID := padContractID(id)
		dataKey := PREFIX_CONTRACT_FREE_GAS_DATA + paddedID

		value, err := s.ldb.Get([]byte(dataKey))
		if err != nil {
			if err == leveldb.ErrNotFound {
				continue
			}
			return nil, 0, fmt.Errorf("failed to get contract at ID %d: %w", id, err)
		}

		contractData := &ContractFreeGasData{}
		if err := json.Unmarshal(value, contractData); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal contract at ID %d: %w", id, err)
		}

		contracts = append(contracts, contractData)
	}

	return contracts, total, nil
}

// ========== CHECK IF CONTRACT EXISTS ==========
func (s *ContractFreeGasStorage) HasContract(contractAddress ethCommon.Address) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists, err := s.getReverseIndex(contractAddress)
	return exists, err
}

// ========== GET CONTRACT BY ADDRESS ==========
func (s *ContractFreeGasStorage) GetContract(contractAddress ethCommon.Address) (*ContractFreeGasData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paddedID, exists, err := s.getReverseIndex(contractAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get reverse index: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("contract %s not found", contractAddress.Hex())
	}

	dataKey := PREFIX_CONTRACT_FREE_GAS_DATA + paddedID
	value, err := s.ldb.Get([]byte(dataKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get contract data: %w", err)
	}

	contractData := &ContractFreeGasData{}
	if err := json.Unmarshal(value, contractData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract data: %w", err)
	}

	return contractData, nil
}

// ========== GET TOTAL COUNT ==========
func (s *ContractFreeGasStorage) GetTotalCount() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getCount()
}

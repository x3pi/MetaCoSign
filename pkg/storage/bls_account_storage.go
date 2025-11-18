package storage

import (
	fmt "fmt"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/ldb_storage"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
)

// Key prefixes for LevelDB
const (
	PREFIX_BLS_ACCOUNT     = "ba:" // bls account: ba:{address} -> BlsAccountData
	PREFIX_PENDING_TX      = "pt:" // pending tx: pt:{address} -> PendingTransaction
	PREFIX_BLS_CONFIRMED   = "bc:" // bls:confirmed:{blsPublicKey} -> BlsAccountList (only confirmed)
	PREFIX_BLS_UNCONFIRMED = "bu:" // bls:unconfirmed:{blsPublicKey} -> BlsAccountList (only unconfirmed)
)

type BlsAccountStorage struct {
	ldb *ldb_storage.LevelDBStorage
}

func NewBlsAccountStorage(ldb *ldb_storage.LevelDBStorage) *BlsAccountStorage {
	return &BlsAccountStorage{ldb: ldb}
}

// SaveBlsAccount lưu thông tin account (Protobuf)
func (s *BlsAccountStorage) SaveBlsAccount(data *pb.BlsAccountData) error {
	key := PREFIX_BLS_ACCOUNT + ethCommon.BytesToAddress(data.Address).Hex()
	value, err := proto.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal account data: %w", err)
	}

	return s.ldb.Put([]byte(key), value)
}

func (s *BlsAccountStorage) GetBlsAccount(address ethCommon.Address) (*pb.BlsAccountData, error) {
	key := PREFIX_BLS_ACCOUNT + address.Hex()
	value, err := s.ldb.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get account data: %w", err)
	}

	data := &pb.BlsAccountData{}
	if err := proto.Unmarshal(value, data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account data: %w", err)
	}
	return data, nil
}

func (s *BlsAccountStorage) SavePendingTransaction(tx *pb.PendingTransaction) error {
	key := PREFIX_PENDING_TX + ethCommon.BytesToAddress(tx.Address).Hex()
	value, err := proto.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal pending transaction: %w", err)
	}

	return s.ldb.Put([]byte(key), value)
}
func (s *BlsAccountStorage) GetPendingTransaction(address ethCommon.Address) (*pb.PendingTransaction, error) {
	key := PREFIX_PENDING_TX + address.Hex()
	value, err := s.ldb.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	tx := &pb.PendingTransaction{}
	if err := proto.Unmarshal(value, tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending tx: %w", err)
	}
	return tx, nil
}
func (s *BlsAccountStorage) DeletePendingTransaction(address ethCommon.Address) error {
	key := PREFIX_PENDING_TX + address.Hex()
	return s.ldb.Delete([]byte(key))
}
func (s *BlsAccountStorage) AddAccountToBlsPublicKey(blsPublicKey []byte, address ethCommon.Address, isConfirmed bool) error {
	var prefix string
	if isConfirmed {
		prefix = PREFIX_BLS_CONFIRMED
	} else {
		prefix = PREFIX_BLS_UNCONFIRMED
	}

	key := prefix + ethCommon.Bytes2Hex(blsPublicKey)

	// Lấy list hiện tại
	list := &pb.BlsAccountList{}
	value, err := s.ldb.Get([]byte(key))
	if err == nil {
		proto.Unmarshal(value, list)
	}

	// Kiểm tra duplicate
	addressBytes := address.Bytes()
	for _, entry := range list.Accounts {
		if ethCommon.BytesToAddress(entry.Address) == address {
			return nil // Already exists
		}
	}
	// Thêm address mới
	entry := &pb.BlsAccountList_AccountEntry{
		Address:     addressBytes,
		IsConfirmed: isConfirmed,
	}
	list.Accounts = append(list.Accounts, entry)
	// Marshal và lưu
	newValue, err := proto.Marshal(list)
	if err != nil {
		return err
	}

	return s.ldb.Put([]byte(key), newValue)
}
func (s *BlsAccountStorage) GetAccountsByBlsPublicKey(blsPublicKey []byte, page, pageSize int, isConfirmed bool) ([]*pb.BlsAccountData, int, error) {
	var prefix string
	if isConfirmed {
		prefix = PREFIX_BLS_CONFIRMED
	} else {
		prefix = PREFIX_BLS_UNCONFIRMED
	}
	key := prefix + ethCommon.Bytes2Hex(blsPublicKey)
	value, err := s.ldb.Get([]byte(key))
	if err != nil {
		return nil, 0, err
	}
	// Unmarshal list
	list := &pb.BlsAccountList{}
	if err := proto.Unmarshal(value, list); err != nil {
		return nil, 0, err
	}

	total := len(list.Accounts)
	start := page * pageSize
	end := start + pageSize

	if start >= total {
		return []*pb.BlsAccountData{}, total, nil
	}
	if end > total {
		end = total
	}

	// Lấy chi tiết từng account
	accounts := make([]*pb.BlsAccountData, 0, end-start)
	for i := start; i < end; i++ {
		addr := ethCommon.BytesToAddress(list.Accounts[i].Address)
		accountData, err := s.GetBlsAccount(addr)
		if err != nil {
			continue
		}
		accounts = append(accounts, accountData)
	}

	return accounts, total, nil
}

// MarkAccountConfirmed đánh dấu account đã được confirm
func (s *BlsAccountStorage) MarkAccountConfirmed(
	address ethCommon.Address,
	confirmTxHash []byte,
	blsPublicKey []byte,
) error {
	data, err := s.GetBlsAccount(address)
	if err != nil {
		return err
	}

	data.IsConfirmed = true
	data.ConfirmedAt = time.Now().Unix()
	data.ConfirmTxHash = confirmTxHash

	// Lưu account data
	if err := s.SaveBlsAccount(data); err != nil {
		return err
	}

	// Di chuyển từ unconfirmed → confirmed
	if err := s.removeAccountFromList(blsPublicKey, address, false); err != nil {
		return fmt.Errorf("failed to remove from unconfirmed: %w", err)
	}

	if err := s.AddAccountToBlsPublicKey(blsPublicKey, address, true); err != nil {
		return fmt.Errorf("failed to add to confirmed: %w", err)
	}

	return nil
}

func (s *BlsAccountStorage) removeAccountFromList(
	blsPublicKey []byte,
	address ethCommon.Address,
	isConfirmed bool,
) error {
	var prefix string
	if isConfirmed {
		prefix = PREFIX_BLS_CONFIRMED
	} else {
		prefix = PREFIX_BLS_UNCONFIRMED
	}

	key := prefix + ethCommon.Bytes2Hex(blsPublicKey)
	value, err := s.ldb.Get([]byte(key))
	if err != nil {
		return err
	}

	list := &pb.BlsAccountList{}
	if err := proto.Unmarshal(value, list); err != nil {
		return err
	}

	newAccounts := make([]*pb.BlsAccountList_AccountEntry, 0, len(list.Accounts))
	for _, entry := range list.Accounts {
		if ethCommon.BytesToAddress(entry.Address) != address {
			newAccounts = append(newAccounts, entry)
		}
	}

	list.Accounts = newAccounts

	newValue, err := proto.Marshal(list)
	if err != nil {
		return err
	}

	return s.ldb.Put([]byte(key), newValue)
}

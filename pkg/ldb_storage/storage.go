package ldb_storage

import (
	"errors"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDBStorage struct {
	db    *leveldb.DB
	Mutex sync.Mutex
}

func NewLevelDBStorage(path string) (*LevelDBStorage, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("lỗi mở LevelDB tại '%s': %w", path, err)
	}
	return &LevelDBStorage{db: db}, nil
}

func (s *LevelDBStorage) Put(key, value []byte) error {
	return s.db.Put(key, value, nil)
}

func (s *LevelDBStorage) Get(key []byte) ([]byte, error) {
	return s.db.Get(key, nil)
}

func (s *LevelDBStorage) Delete(key []byte) error {
	return s.db.Delete(key, nil)
}
func (s *LevelDBStorage) Has(key []byte, ro *opt.ReadOptions) (bool, error) {
	_, err := s.db.Get(key, ro)

	if err == nil {
		return true, nil
	}
	if errors.Is(err, leveldb.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// Thêm phương thức này để hỗ trợ ghi theo lô
func (s *LevelDBStorage) WriteBatch(batch *leveldb.Batch, wo *opt.WriteOptions) error {
	return s.db.Write(batch, wo)
}
func (s *LevelDBStorage) NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator {
	return s.db.NewIterator(slice, ro)
}
func (s *LevelDBStorage) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

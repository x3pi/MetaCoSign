package block

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	"github.com/meta-node-blockchain/meta-node/types"
)

var (
	lastBlockHashKey common.Hash = common.BytesToHash(crypto.Keccak256([]byte("lastBlockHashKey")))
)

type BlockDatabase struct {
	db        storage.Storage
	bockBatch []byte
}

func NewBlockDatabase(
	db storage.Storage,

) *BlockDatabase {
	return &BlockDatabase{
		db: db,
	}
}
func (blockDatabase *BlockDatabase) SetBlockBatch(batch []byte) {
	blockDatabase.bockBatch = batch
}

func (blockDatabase *BlockDatabase) GetBlockBatch() []byte {
	batch := blockDatabase.bockBatch
	blockDatabase.bockBatch = nil
	return batch
}

// GetDB returns the underlying ShardelDB instance.
func (blockDatabase *BlockDatabase) GetDB() storage.Storage {
	return blockDatabase.db
}

// func (blockDatabase *BlockDatabase) SetNode(node *node.HostNode) {
// 	blockDatabase.node = node
// }

// func (blockDatabase *BlockDatabase) SaveBlock(block types.Block) error {
// 	// Encode block to bytes
// 	blockBytes, err := block.Marshal()
// 	if err != nil {
// 		return err
// 	}

// 	// Save block to database
// 	blockHash := block.Header().Hash()
// 	if err := blockDatabase.db.Put(blockHash.Bytes(), blockBytes); err != nil {
// 		return err
// 	}

// 	return nil
// }

// SaveLastBlock saves the last block's hash to the database.
func (blockDatabase *BlockDatabase) SaveLastBlock(block types.Block) error {

	// Encode block to bytes
	blockBytes, err := block.Marshal()
	if err != nil {
		return err
	}
	// if err := blockDatabase.db.Put(lastBlockHashKey.Bytes(), blockBytes); err != nil {
	// 	return err
	// }

	var batch [][2][]byte

	batch = append(batch, [2][]byte{lastBlockHashKey.Bytes(), blockBytes})
	batch = append(batch, [2][]byte{block.Header().Hash().Bytes(), blockBytes})
	blockDatabase.db.BatchPut(batch)
	if len(batch) > 0 { // Chỉ thực hiện BatchPut nếu có dữ liệu
		if err := blockDatabase.db.BatchPut(batch); err != nil {
			return err
		}
		if config.ConfigApp.ServiceType == p_common.ServiceTypeMaster {
			data, err := storage.SerializeBatch(batch)
			if err != nil {
				logger.Error(fmt.Sprintf("Error marshaling receipt: %v", err))
			}
			blockDatabase.SetBlockBatch(data)
		}
	}
	return nil
}

func (blockDatabase *BlockDatabase) GetBlockByHash(blockHash common.Hash) (types.Block, error) {
	// Try to load the block from the database
	blockBytes, err := blockDatabase.db.Get(blockHash.Bytes())
	if err != nil {
		return nil, err
	}
	block := &Block{}

	err = block.Unmarshal(blockBytes)

	if err != nil {
		return nil, err
	}
	return block, nil
}

func (blockDatabase *BlockDatabase) GetBlockByHashFromDb(blockHash common.Hash) (types.Block, error) {
	// Try to load the block from the database
	blockBytes, err := blockDatabase.db.Get(blockHash.Bytes())
	if err != nil {
		return nil, err
	}
	block := &Block{}

	err = block.Unmarshal(blockBytes)

	if err != nil {
		return nil, err
	}

	return block, nil
}

// GetLastBlock retrieves the last block from the database.
func (blockDatabase *BlockDatabase) GetLastBlock() (types.Block, error) {
	bl, err := blockDatabase.GetBlockByHash(lastBlockHashKey)
	return bl, err
}

// GetLastBlock retrieves the last block from the database.
func (blockDatabase *BlockDatabase) GetLastBlockFromDb() (types.Block, error) {
	return blockDatabase.GetBlockByHashFromDb(lastBlockHashKey)
}

package executor

import (
	"fmt"

	"github.com/meta-node-blockchain/meta-node/pkg/block"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

type RequestHandler struct {
	storageManager *storage.StorageManager
	chainState     *blockchain.ChainState
}

func NewRequestHandler(storageManager *storage.StorageManager, chainState *blockchain.ChainState) *RequestHandler {
	return &RequestHandler{
		storageManager: storageManager,
		chainState:     chainState,
	}
}

// HandleBlockRequest processes a BlockRequest and returns a ValidatorList.
func (rh *RequestHandler) HandleBlockRequest(request *pb.BlockRequest) (*pb.ValidatorList, error) {
	blockNumber := request.GetBlockNumber()
	logger.Error("Handling block request for block number:", blockNumber)
	blockHash, ok := blockchain.GetBlockChainInstance().GetBlockHashByNumber(blockNumber)
	if !ok {
		return nil, fmt.Errorf("cannot find block hash for block number %d", blockNumber)
	}

	blockData, err := rh.chainState.GetBlockDatabase().GetBlockByHash(blockHash)
	if err != nil {
		return nil, fmt.Errorf("could not get block data by hash %s: %w", blockHash, err)
	}

	blockDatabase := block.NewBlockDatabase(rh.storageManager.GetStorageBlock())
	chainStateNew, err := blockchain.NewChainState(rh.storageManager, blockDatabase, blockData.Header(), rh.chainState.GetConfig(), rh.chainState.GetFreeFeeAddress())
	if err != nil {
		return nil, fmt.Errorf("could not create new chain state: %w", err)
	}

	validators, err := chainStateNew.GetStakeStateDB().GetAllValidators()
	if err != nil {
		return nil, fmt.Errorf("could not get all validators from stake DB: %w", err)
	}
	// Map the database validators to protobuf validators.
	validatorList := &pb.ValidatorList{}
	for _, dbValidator := range validators {
		val := &pb.Validator{
			Address:                    dbValidator.Address().Hex(),
			PrimaryAddress:             dbValidator.PrimaryAddress(),
			WorkerAddress:              dbValidator.WorkerAddress(),
			P2PAddress:                 dbValidator.P2PAddress(),
			Name:                       dbValidator.Name(),
			Description:                dbValidator.Description(),
			Website:                    dbValidator.Website(),
			Image:                      dbValidator.Image(),
			CommissionRate:             dbValidator.CommissionRate(),
			MinSelfDelegation:          dbValidator.MinSelfDelegation().String(),
			TotalStakedAmount:          dbValidator.TotalStakedAmount().String(),
			AccumulatedRewardsPerShare: dbValidator.AccumulatedRewardsPerShare().String(),
			PubkeyBls:                  dbValidator.PubKeyBls(),
			PubkeySecp:                 dbValidator.PubKeySecp(),
		}
		validatorList.Validators = append(validatorList.Validators, val)
	}

	return validatorList, nil
}

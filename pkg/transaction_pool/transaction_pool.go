package transaction_pool

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	blst "github.com/meta-node-blockchain/meta-node/pkg/bls/blst/bindings/go"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/types"
)

type TransactionPool struct {
	transactions    []types.Transaction
	transactionKeys map[string]bool // Use a map to store transaction keys for quick existence checks

	aggSign *blst.P2Aggregate
	mutex   sync.Mutex
}

func NewTransactionPool() *TransactionPool {
	return &TransactionPool{
		transactions:    make([]types.Transaction, 0), // Initialize the transactions slice
		transactionKeys: make(map[string]bool),        // Initialize the transactionKeys map
		aggSign:         new(blst.P2Aggregate)}
}

func (tp *TransactionPool) CountTransactions() int {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	return len(tp.transactions)
}

func (tp *TransactionPool) AddTransaction(tx types.Transaction) error {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()
	return tp.addTransaction(tx)
}

func (tp *TransactionPool) AddTransactions(txs []types.Transaction) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	for _, tx := range txs {
		tp.addTransaction(tx)
	}
}

func (tp *TransactionPool) addTransaction(tx types.Transaction) error {
	key := tx.FromAddress().String() + strconv.FormatUint(tx.GetNonce(), 10) // Combine FromAddress and Nonce for a unique key

	// Check if the transaction already exists in the pool
	if _, exists := tp.transactionKeys[key]; exists {
		logger.Info("Transaction already exists in pool, skipping", "key", key)
		return fmt.Errorf("transaction already exists in pool, skipping")
	}

	// Add the transaction to the pool
	tp.transactions = append(tp.transactions, tx)
	tp.transactionKeys[key] = true // Add the key to the transactionKeys map

	// Aggregate the signature chưa rõ mục đích bước này là gì
	// p := new(blst.P2Affine)
	// p.Uncompress(tx.Sign().Bytes())
	// if err != nil {
	// 	logger.Error("Error uncompressing signature ", err)
	// 	return
	// }
	// tp.aggSign.Add(p, false)
	return nil
}

// TransactionsWithAggSign returns transactions and aggregate sign
// and clear transactions
func (tp *TransactionPool) TransactionsWithAggSign() ([]types.Transaction, []byte) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	tx := tp.transactions
	// sign := tp.aggSign.ToAffine().Compress()

	// Clear the transaction pool
	tp.transactions = make([]types.Transaction, 0)
	tp.transactionKeys = make(map[string]bool) // Clear the transaction keys as well
	tp.aggSign = new(blst.P2Aggregate)

	return tx, nil
}

func (tp *TransactionPool) GetTransactionByHash(hashToFind common.Hash) (types.Transaction, bool) {
	tp.mutex.Lock()
	defer tp.mutex.Unlock()

	if hashToFind == (common.Hash{}) {
		logger.Warn("GetTransactionByHash called with a zero hash value.")
		return nil, false
	}

	for _, tx := range tp.transactions {
		ethTx := tx.ToEthTransaction()
		if ethTx == nil {
			continue
		} else {
			currentTxHash := ethTx.Hash()

			if currentTxHash == (common.Hash{}) {
				logger.Warn(
					"Transaction in pool has a zero hash",
					"fromAddress", tx.FromAddress().String(),
					"nonce", tx.GetNonce(),
				)
				continue
			}

			if currentTxHash == hashToFind {
				return tx, true
			}
		}

	}

	return nil, false
}

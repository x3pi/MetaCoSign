package tx_processor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain/trace"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain/vm_processor"
	"github.com/meta-node-blockchain/meta-node/pkg/trie_database"
	"github.com/meta-node-blockchain/meta-node/pkg/utils"

	mt_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/grouptxns"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/receipt"
	"github.com/meta-node-blockchain/meta-node/types"
)

type ProcessResult struct {
	Transactions     []types.Transaction
	Receipts         []types.Receipt
	ExecuteSCResults []types.ExecuteSCResult
	Root             common.Hash
	StakeStatesRoot  common.Hash
	Error            error
	EventLogs        map[common.Address][]types.EventLog
}

// ProcessTransactions processes a batch of transactions.
func ProcessTransactions(ctx context.Context, chainState *blockchain.ChainState, groupedGroups []grouptxns.RelativeGroup, enableTrace bool, isCache bool) (
	ProcessResult,
	error,
) {
	lastBlockHeader := chainState.GetcurrentBlockHeader()

	var funcCtx context.Context
	var funcSpan *trace.Span
	if enableTrace {
		tracedCtx, actualSpan := trace.StartSpan(ctx, "TxProcessor.processGroupsConcurrently", map[string]interface{}{
			"groupCount": len(groupedGroups),
		})
		funcCtx = tracedCtx
		funcSpan = actualSpan
		defer funcSpan.End() // Kết thúc span khi hàm này thoát
	} else {
		funcCtx = ctx // Sử dụng context gốc (có thể là blockCtx)
		funcSpan = nil
	}

	// *** Call the new function for concurrent processing ***
	allTransactions, allReceipts, allExecuteSCResults := processGroupsConcurrently(funcCtx, chainState, groupedGroups, *lastBlockHeader, enableTrace, isCache)

	// Get event logs (potentially modified by concurrent processing)
	eventLogs := chainState.GetSmartContractDB().EventLogs()

	// Note: Ensure accountStateDB is safe for concurrent reads/writes or handle synchronization appropriately.

	trie_database.GetTrieDatabaseManager().IntermediateRoot()

	root, err := chainState.GetAccountStateDB().IntermediateRoot(true)
	if err != nil {
		panic(" Lỗi lấy IntermediateRoot accountStateDB root")
	}

	stakeRoot, err := chainState.GetStakeStateDB().IntermediateRoot(true)
	if err != nil {
		panic(" Lỗi lấy IntermediateRoot accountStateDB root")
	}

	// Prepare and send the final result
	processResult := ProcessResult{
		Transactions:     allTransactions,
		Receipts:         allReceipts,
		ExecuteSCResults: allExecuteSCResults,
		Root:             root,
		Error:            nil,
		EventLogs:        eventLogs,
		StakeStatesRoot:  stakeRoot,
	}
	return processResult, nil
}

// ProcessTransactions processes a batch of transactions.
func ProcessTransactionsRemote(ctx context.Context, chainState *blockchain.ChainState, groupedGroups []grouptxns.RelativeGroup, enableTrace bool, isCache bool) (
	ProcessResult,
	error,
) {
	lastBlockHeader := chainState.GetcurrentBlockHeader()

	var funcCtx context.Context
	var funcSpan *trace.Span
	if enableTrace {
		tracedCtx, actualSpan := trace.StartSpan(ctx, "TxProcessor.processGroupsConcurrently", map[string]interface{}{
			"groupCount": len(groupedGroups),
		})
		funcCtx = tracedCtx
		funcSpan = actualSpan
		defer funcSpan.End() // Kết thúc span khi hàm này thoát
	} else {
		funcCtx = ctx // Sử dụng context gốc (có thể là blockCtx)
		funcSpan = nil
	}

	// *** Call the new function for concurrent processing ***
	allTransactions, allReceipts, allExecuteSCResults := processGroupsConcurrently(funcCtx, chainState, groupedGroups, *lastBlockHeader, enableTrace, isCache)

	// Get event logs (potentially modified by concurrent processing)
	eventLogs := chainState.GetSmartContractDB().EventLogs()

	// Note: Ensure accountStateDB is safe for concurrent reads/writes or handle synchronization appropriately.

	root, err := chainState.GetAccountStateDB().IntermediateRoot(true)
	if err != nil {
		panic(" Lỗi lấy IntermediateRoot accountStateDB root")
	}

	stakeRoot, err := chainState.GetStakeStateDB().IntermediateRoot(true)
	if err != nil {
		panic(" Lỗi lấy IntermediateRoot accountStateDB root")
	}

	// Prepare and send the final result

	processResult := ProcessResult{
		Transactions:     allTransactions,
		Receipts:         allReceipts,
		ExecuteSCResults: allExecuteSCResults,
		Root:             root,
		Error:            nil,
		EventLogs:        eventLogs,
		StakeStatesRoot:  stakeRoot,
	}
	// Send result to channel
	// Consider if sending on the channel should happen outside the lock if it blocks
	// Return results
	return processResult, nil
}

func processGroupsConcurrently(
	ctx context.Context,
	chainState *blockchain.ChainState,
	groupedGroups []grouptxns.RelativeGroup,
	lastBlockHeader types.BlockHeader,
	enableTrace bool,
	isCache bool,
) (
	[]types.Transaction,
	[]types.Receipt,
	[]types.ExecuteSCResult,
) {

	var funcCtx context.Context
	var funcSpan *trace.Span

	// Bắt đầu span cho hàm này (nếu được bật)
	if enableTrace {
		tracedCtx, actualSpan := trace.StartSpan(ctx, "TxProcessor.processGroupsConcurrently", map[string]interface{}{
			"groupCount": len(groupedGroups),
		})
		funcCtx = tracedCtx
		funcSpan = actualSpan
		defer funcSpan.End() // Kết thúc span khi hàm này thoát
	} else {
		funcCtx = ctx // Sử dụng context gốc (có thể là blockCtx)
		funcSpan = nil
	}

	allTransactions := []types.Transaction{}
	allReceipts := []types.Receipt{}
	allExecuteSCResults := []types.ExecuteSCResult{}

	ch := make(chan grouptxns.GroupResult, len(groupedGroups))
	var wg sync.WaitGroup
	wg.Add(len(groupedGroups))

	go func() {
		wg.Wait()
		close(ch)
	}()

	for i, group := range groupedGroups {
		id := group.GroupID
		combinedHash := sha256.Sum256([]byte(fmt.Sprintf("%x%d", lastBlockHeader.LastBlockHash(), id)))
		ethAddressBytes := combinedHash[12:]
		mvmId := common.BytesToAddress(ethAddressBytes)
		var groupCtx context.Context
		var groupSpan *trace.Span

		// Bắt đầu span cho mỗi group (nếu được bật)
		if enableTrace {
			// Sử dụng funcCtx làm parent context
			tracedGroupCtx, actualGroupSpan := trace.StartSpan(funcCtx, fmt.Sprintf("TxProcessor.ProcessGroup-%d", i), map[string]interface{}{
				"groupID":   group.GroupID,
				"itemCount": len(group.Items),
			})
			groupCtx = tracedGroupCtx
			groupSpan = actualGroupSpan
			// Lưu ý: Không defer End() ở đây vì span này thuộc về goroutine sẽ chạy
		} else {
			groupCtx = funcCtx // Sử dụng context gốc của hàm này
			groupSpan = nil
		}

		go func(gCtx context.Context, gSpan *trace.Span, groupItems []grouptxns.Item, mvmId common.Address, traceEnabled bool) {
			// Kết thúc span của group này khi goroutine hoàn thành (nếu span tồn tại)
			if traceEnabled && gSpan != nil {
				defer gSpan.End()
			}
			defer wg.Done()
			result := processSingleGroup(groupCtx, chainState, groupItems, mvmId, lastBlockHeader, traceEnabled, isCache)
			ch <- result
		}(groupCtx, groupSpan, group.Items, mvmId, enableTrace)
	}
	if enableTrace {
		funcSpan.AddEvent("AllGoroutinesLaunched", nil)
	}

	for gRs := range ch {
		allTransactions = append(allTransactions, gRs.Transactions...)
		allReceipts = append(allReceipts, gRs.Receipts...)
		allExecuteSCResults = append(allExecuteSCResults, gRs.ExecuteSCResults...)
	}
	if enableTrace {
		funcSpan.AddEvent("ResultsCollected", map[string]interface{}{
			"totalTxs":      len(allTransactions),
			"totalReceipts": len(allReceipts),
		})
	}

	return allTransactions, allReceipts, allExecuteSCResults
}

func processSingleGroup(
	ctx context.Context,
	chainState *blockchain.ChainState,
	groupItems []grouptxns.Item,
	mvmId common.Address,
	lastBlockHeader types.BlockHeader,
	enableTrace bool,
	isCache bool,
) grouptxns.GroupResult {
	gRs := grouptxns.GroupResult{
		Transactions:     []types.Transaction{},
		Receipts:         []types.Receipt{},
		ExecuteSCResults: []types.ExecuteSCResult{},
		Error:            nil,
	}

	hasFailed := false // Đánh dấu nếu 1 tx đã bị lỗi
	blockTime := uint64(time.Now().Unix())
	for _, item := range groupItems {
		tx := item.Tx
		var txCtx context.Context
		var txSpan *trace.Span

		if enableTrace {
			tracedTxCtx, actualTxSpan := trace.StartSpan(ctx, "TxProcessor.ProcessSingleTransaction", map[string]interface{}{
				"txHash":   tx.Hash().Hex(),
				"from":     tx.FromAddress().Hex(),
				"to":       tx.ToAddress().Hex(),
				"isCall":   tx.IsCallContract(),
				"isDeploy": tx.IsDeployContract(),
			})
			txCtx = tracedTxCtx
			txSpan = actualTxSpan
			defer txSpan.End()
		} else {
			txCtx = ctx
			txSpan = nil
		}

		toAddress := tx.ToAddress()
		if tx.IsDeployContract() {
			toAddress = common.Address{}
		}
		// ❗ Nếu đã có lỗi trước đó, tạo receipt lỗi cho tx này và tiếp tục
		if hasFailed {
			rcp := createErrorReceipt(tx, toAddress, fmt.Errorf("skipped due to previous transaction failure"))
			gRs.Receipts = append(gRs.Receipts, rcp)
			gRs.Transactions = append(gRs.Transactions, tx)
			continue
		}

		// Phần xử lý bình thường
		as, _ := chainState.GetAccountStateDB().AccountState(tx.FromAddress())
		var err error

		var rcp types.Receipt
		// ++++++++++ chỗ này PearTNhat cần xử lý stake +++++++++++++++++++
		if tx.ToAddress() == utils.GetAddressSelector(mt_common.IDENTIFIER_STAKE) {
			validatorHandler, err := GetValidatorHandler()
			if err != nil {
				logger.Error("Lỗi khi tạo ValidatorHandler: %v", err)
				rcp = createErrorReceipt(tx, toAddress, err)
				gRs.Receipts = append(gRs.Receipts, rcp)
				hasFailed = true
				continue
			}
			rcp, exRs, txFailed := validatorHandler.HandleTransaction(txCtx, chainState, tx, enableTrace)
			gRs.Receipts = append(gRs.Receipts, rcp)
			gRs.Transactions = append(gRs.Transactions, tx)
			if exRs != nil { // exRs có thể nil trong một số trường hợp lỗi
				gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)
			}
			if txFailed {
				hasFailed = true
			}
			continue // Chuyển sang transaction tiếp theo
		}
		// if tx.ToAddress() == common.HexToAddress("0x55eBc0976Ef439FCdA5b73dcFe593BaeE6354685") {
		// 	logger.Error("Xử lý FileHandler cho tx %s", tx.Hash().Hex())
		// 	fileHandler, err := GetFileHandler()
		// 	if err != nil {
		// 		logger.Error("Lỗi khi tạo FileHandler: %v", err)
		// 		rcp = createErrorReceipt(tx, toAddress, err)
		// 		gRs.Receipts = append(gRs.Receipts, rcp)
		// 		hasFailed = true
		// 		continue
		// 	}
		// 	rcp, exRs, txFailed, isPassed := fileHandler.HandleFileTransaction(txCtx, tx, chainState, enableTrace)
		// 	if !isPassed {
		// 		gRs.Receipts = append(gRs.Receipts, rcp)
		// 		gRs.Transactions = append(gRs.Transactions, tx)
		// 		if exRs != nil { // exRs có thể nil trong một số trường hợp lỗi
		// 			gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)
		// 		}
		// 		if txFailed {
		// 			hasFailed = true
		// 		}
		// 		continue // Chuyển sang transaction tiếp theo
		// 	}
		// }
		if tx.ToAddress() == utils.GetAddressSelector(mt_common.ACCOUNT_SETTING_ADDRESS_SELECT) {
			dataInput := tx.CallData().Input()
			if len(dataInput) < 4 {
				logger.Error("Invalid calldata: less than 4 bytes")
				err := errors.New("invalid calldata")
				rcp := createErrorReceipt(tx, toAddress, err)
				gRs.Receipts = append(gRs.Receipts, rcp)
				gRs.Transactions = append(gRs.Transactions, tx)
				hasFailed = true
				continue
			}

			selector := dataInput[:4]
			fromAddr := tx.FromAddress()

			switch {
			case tx.GetNonce() == 0 && bytes.Equal(selector, utils.GetFunctionSelector("setBlsPublicKey(bytes)")):
				plk, err := UnpackSetBlsPublicKeyInput(dataInput)
				if err != nil {
					logger.Error("UnpackSetBlsPublicKeyInput failed:", err)
					panic(err)
				}
				if len(as.PublicKeyBls()) != 0 {
					panic("PublicKeyBls already exists")
				}
				err = chainState.GetAccountStateDB().SetPublicKeyBls(fromAddr, plk)
				if err != nil {
					rcp := createErrorReceipt(tx, toAddress, err)
					gRs.Receipts = append(gRs.Receipts, rcp)
					gRs.Transactions = append(gRs.Transactions, tx)
					logger.Error("SetPublicKeyBls failed for tx %s: %v", tx.Hash().Hex(), err)
					hasFailed = true
					continue
				}
			case tx.GetNonce() != 0 && bytes.Equal(selector, utils.GetFunctionSelector("setAccountType(uint8)")):
				acType, err := UnpackSetAccountTypeInput(dataInput)
				if err != nil {
					logger.Error("UnpackSetAccountTypeInput failed:", err)
					panic(err)
				}
				err = chainState.GetAccountStateDB().SetAccountType(fromAddr, acType)
				if err != nil {
					logger.Error("SetAccountType failed:", err)
					panic(err)
				}
			default:
				err := errors.New("invalid selector")
				rcp := createErrorReceipt(tx, toAddress, err)
				gRs.Receipts = append(gRs.Receipts, rcp)
				gRs.Transactions = append(gRs.Transactions, tx)
				logger.Error("Transaction failed for tx %s: %v", tx.Hash().Hex(), err)
				hasFailed = true
				continue
			}

			rcp := receipt.NewReceipt(
				tx.Hash(), tx.FromAddress(), toAddress, tx.Amount(),
				pb.RECEIPT_STATUS_RETURNED, nil, pb.EXCEPTION_NONE,
				mt_common.MINIMUM_BASE_FEE, mt_common.TRANSFER_GAS_COST,
				[]types.EventLog{}, 0, common.Hash{}, 0,
			)

			var exRs types.ExecuteSCResult
			vmP := vm_processor.NewVmProcessor(chainState, tx.ToAddress(), enableTrace, blockTime)

			exRs, err = vmP.ExecuteNonceOnly(txCtx, tx, true)
			if err != nil {
				rcp = createErrorReceipt(tx, toAddress, err)
				if exRs != nil {
					rcp.UpdateExecuteResult(exRs.ReceiptStatus(), exRs.Return(), exRs.Exception(), exRs.GasUsed(), exRs.EventLogs())
					gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)
				}
				gRs.Receipts = append(gRs.Receipts, rcp)
				gRs.Transactions = append(gRs.Transactions, tx)
				logger.Error("executeTransactionWithMvmId failed for tx %s: %v", tx.Hash().Hex(), err)
				hasFailed = true // ❗ Đánh dấu lỗi
				continue
			}
			rcp.UpdateExecuteResult(exRs.ReceiptStatus(), exRs.Return(), exRs.Exception(), exRs.GasUsed(), exRs.EventLogs())
			chainState.GetAccountStateDB().SetLastHash(tx.FromAddress(), tx.Hash())
			chainState.GetAccountStateDB().SetNewDeviceKey(tx.FromAddress(), tx.NewDeviceKey())

			gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)
			gRs.Receipts = append(gRs.Receipts, rcp)
			gRs.Transactions = append(gRs.Transactions, tx)
			continue
		}

		rcp = receipt.NewReceipt(
			tx.Hash(), tx.FromAddress(), toAddress, tx.Amount(),
			pb.RECEIPT_STATUS_RETURNED, nil, pb.EXCEPTION_NONE,
			mt_common.MINIMUM_BASE_FEE, mt_common.TRANSFER_GAS_COST,
			[]types.EventLog{}, 0, common.Hash{}, 0,
		)

		var exRs types.ExecuteSCResult
		vmP := vm_processor.NewVmProcessor(chainState, tx.ToAddress(), enableTrace, blockTime)
		if tx.IsDeployContract() || !isCache {
			vmP = vm_processor.NewVmProcessor(chainState, mvmId, enableTrace, blockTime)
		}

		exRs, err = vmP.ExecuteTransactionWithMvmId(txCtx, tx, false, isCache)
		if err != nil {
			rcp = createErrorReceipt(tx, toAddress, err)
			if exRs != nil {
				rcp.UpdateExecuteResult(exRs.ReceiptStatus(), exRs.Return(), exRs.Exception(), exRs.GasUsed(), exRs.EventLogs())
				gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)
			}
			gRs.Receipts = append(gRs.Receipts, rcp)
			gRs.Transactions = append(gRs.Transactions, tx)
			logger.Error("executeTransactionWithMvmId failed for tx %s: %v", tx.Hash().Hex(), err)
			hasFailed = true // ❗ Đánh dấu lỗi
			continue
		}
		rcp.UpdateExecuteResult(exRs.ReceiptStatus(), exRs.Return(), exRs.Exception(), exRs.GasUsed(), exRs.EventLogs())
		chainState.GetAccountStateDB().SetLastHash(tx.FromAddress(), tx.Hash())
		chainState.GetAccountStateDB().SetNewDeviceKey(tx.FromAddress(), tx.NewDeviceKey())

		gRs.ExecuteSCResults = append(gRs.ExecuteSCResults, exRs)

		gRs.Receipts = append(gRs.Receipts, rcp)
		gRs.Transactions = append(gRs.Transactions, tx)
		logs := rcp.EventLogs()
		for i, log := range logs {
			logger.Error("===== EventLog #%d =====", i+1)
			// Transaction hash, address, data
			logger.Error("TransactionHash: %s", hex.EncodeToString(log.TransactionHash))
			logger.Error("Address: %s", hex.EncodeToString(log.Address))
			logger.Error("Data: %s", hex.EncodeToString(log.Data))

			// Topics
			for j, topic := range log.Topics {
				logger.Error("  Topic[%d]: %s", j, hex.EncodeToString(topic))
			}

			logger.Error("=========================")
		}

	}

	return gRs
}

func createErrorReceipt(tx types.Transaction, toAddress common.Address, err error) types.Receipt {
	return receipt.NewReceipt(
		tx.Hash(), tx.FromAddress(), toAddress, tx.Amount(),
		pb.RECEIPT_STATUS_TRANSACTION_ERROR, []byte(err.Error()), pb.EXCEPTION_NONE,
		mt_common.MINIMUM_BASE_FEE, 0, []types.EventLog{}, 0, common.Hash{}, 0,
	)
}

package vm_processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain"
	"github.com/meta-node-blockchain/meta-node/pkg/blockchain/trace"
	mt_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/mvm"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/smart_contract"
	"github.com/meta-node-blockchain/meta-node/pkg/trie"
	"github.com/meta-node-blockchain/meta-node/pkg/trie_database"
	"github.com/meta-node-blockchain/meta-node/pkg/utils"
	"github.com/meta-node-blockchain/meta-node/types"
)

// VmProcessor struct now includes a flag to control tracing internally.
type VmProcessor struct {
	chainState     *blockchain.ChainState
	mvmId          common.Address // Consider if this is still needed here or managed by the caller.
	tracingEnabled bool
	blockTime      uint64
}

// NewVmProcessor tạo một thực thể VmProcessor mới và thiết lập trạng thái trace.
func NewVmProcessor(cs *blockchain.ChainState, mvmId common.Address, enableTrace bool, blockTime uint64) *VmProcessor {
	return &VmProcessor{
		chainState:     cs,
		mvmId:          mvmId,
		tracingEnabled: enableTrace,
		blockTime:      blockTime,
	}
}

// ExecuteTransactionWithMvmId thực thi giao dịch, sử dụng cờ tracingEnabled nội bộ.
func (vmP *VmProcessor) ExecuteTransactionWithMvmId(
	ctx context.Context, // Context gốc từ caller
	tx types.Transaction,
	extendedMode bool,
	isCache bool,
) (types.ExecuteSCResult, error) {
	var execCtx context.Context = ctx // Mặc định dùng context gốc
	var span *trace.Span = nil        // Mặc định span là nil

	if vmP.tracingEnabled { // Chỉ tạo span gốc nếu flag bật
		var actualSpan *trace.Span
		execCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.ExecuteTransactionWithMvmId", map[string]interface{}{
			"txHash":       tx.Hash().Hex(),
			"from":         tx.FromAddress().Hex(),
			"to":           tx.ToAddress().Hex(),
			"value":        tx.Amount().String(),
			"gasLimit":     tx.MaxGas(),
			"gasPrice":     tx.MaxGasPrice(),
			"nonce":        tx.GetNonce(),
			"isReadOnly":   tx.GetReadOnly(),
			"isDeploy":     tx.IsDeployContract(),
			"isCall":       tx.IsCallContract(),
			"extendedMode": extendedMode,
			"mvmId":        vmP.mvmId.Hex(),
		})
		span = actualSpan
		defer span.End() // Defer End cho span gốc này
	}

	if tx.GetReadOnly() {
		if span != nil {
			span.AddEvent("HandlingReadOnlyTransaction", nil)
		}
		combinedHash := sha256.Sum256([]byte(fmt.Sprintf("%x%d%d", tx.Hash(), rand.Int63(), time.Now().UnixNano())))
		ethAddressBytes := combinedHash[12:]
		mvmIdReadOnly := common.BytesToAddress(ethAddressBytes)
		if span != nil {
			span.SetAttribute("readOnlyMvmId", mvmIdReadOnly.Hex())
		}
		mvmROnly := mvm.GetOrCreateMVMApi(mvmIdReadOnly, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), extendedMode)
		mvmROnly.SetRelatedAddresses(tx.RelatedAddresses())
		result := vmP.readOnlyCall(execCtx, tx, mvmROnly)
		if span != nil {
			span.SetAttribute("readOnlyResultStatus", result.ReceiptStatus().String())
			span.SetAttribute("readOnlyResultGasUsed", result.GasUsed())
			span.SetAttribute("readOnlyResultReturnHex", hex.EncodeToString(result.Return()))
		}
		return result, nil
	}

	if span != nil {
		span.AddEvent("HandlingWriteTransaction", map[string]interface{}{"actualMvmId": vmP.mvmId.Hex()})
	}
	if isCache {
		mvm.ProtectMVMApi(vmP.mvmId)
	}
	mvmE := mvm.GetOrCreateMVMApi(vmP.mvmId, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), extendedMode)
	mvmE.SetRelatedAddresses(tx.RelatedAddresses())
	if tx.IsRegularTransaction() || tx.ToAddress() == utils.GetAddressSelector(mt_common.ACCOUNT_SETTING_ADDRESS_SELECT) {
		rs, err := vmP.sendNative(execCtx, tx, mvmE)
		if err != nil && span != nil {
			span.SetError(err)
		} else if span != nil {
			span.SetAttribute("executeResultStatus", rs.ReceiptStatus().String())
			span.SetAttribute("executeResultGasUsed", rs.GasUsed())
			span.SetAttribute("executeResultReturnHex", hex.EncodeToString(rs.Return()))
		}
		return rs, err

	}

	if tx.IsDeployContract() {
		if span != nil {
			span.AddEvent("HandlingDeployContract", map[string]interface{}{
				"deployDataCodeLength": len(tx.DeployData().Code()),
				"storageAddress":       tx.DeployData().StorageAddress().Hex(),
			})
		}
		result, err := vmP.deploySmartContract(execCtx, tx, mvmE, vmP.mvmId, isCache)
		if err != nil && span != nil {
			span.SetError(err)
		} else if span != nil {
			span.SetAttribute("deployResultStatus", result.ReceiptStatus().String())
			span.SetAttribute("deployResultGasUsed", result.GasUsed())
		}
		return result, err
	}

	if span != nil {
		span.AddEvent("HandlingCallContract", map[string]interface{}{
			"callDataInputLength": len(tx.CallData().Input()),
		})
	}
	toAccountState, err := vmP.chainState.GetAccountStateDB().AccountState(tx.ToAddress())
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get 'to' account state: %w", err)
		if span != nil {
			span.AddEvent("ErrorGettingToAccountState", map[string]interface{}{"error": wrappedErr.Error()})
			span.SetError(wrappedErr)
		}
		logger.Error("ClearMVM: 1", vmP.mvmId)
		mvm.ClearMVMApi(vmP.mvmId)
		return vmP.invalidTransactionResponse(execCtx, tx, "error getting 'to' account state"), nil
	}

	if !vmP.IsValidSmartContractCall(toAccountState, tx) {
		if span != nil {
			reason := "invalid smart contract call"
			details := map[string]interface{}{
				"toAccountExists": toAccountState != nil,
			}
			if toAccountState != nil {
				scState := toAccountState.SmartContractState()
				details["isContract"] = scState != nil
				if scState != nil {
					details["contractStorageRoot"] = scState.StorageRoot().Hex()
					details["expectedStorageRoot"] = vmP.chainState.GetSmartContractDB().StorageRoot(tx.ToAddress()).Hex()
					reason = "account not found, not a contract, or storage root mismatch"
				} else {
					reason = "target account is not a contract"
				}
			} else {
				reason = "target account not found"
			}
			details["reason"] = reason
			span.AddEvent("InvalidSmartContractCall", details)
		}
		// logger.Error("ClearMVM: 2", vmP.mvmId)

		// mvm.ClearMVMApi(vmP.mvmId)
		return vmP.invalidTransactionResponse(execCtx, tx, "invalid smart contract call"), nil
	}
	if span != nil {
		span.AddEvent("SmartContractCallValidationPassed", nil)
	}

	if span != nil {
		span.AddEvent("ExecutingSmartContractViaMVM", nil)
	}
	rs, err := vmP.executeSmartContract(execCtx, tx, mvmE, isCache)
	if err != nil && span != nil {
		span.SetError(err)
	} else if span != nil {
		span.SetAttribute("executeResultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("executeResultGasUsed", rs.GasUsed())
		span.SetAttribute("executeResultReturnHex", hex.EncodeToString(rs.Return()))
	}
	return rs, err
}

// deploySmartContract xử lý việc deploy smart contract.
func (vmP *VmProcessor) deploySmartContract(
	ctx context.Context, // Context từ caller
	tx types.Transaction,
	mvmE *mvm.MVMApi,
	mvmId common.Address,
	isCache bool,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil          // Khởi tạo nil
	var deployCtx context.Context = ctx // Mặc định dùng context vào
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()
	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		deployCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.deploySmartContract", map[string]interface{}{
			"mvmId":         mvmId.Hex(),
			"from":          tx.FromAddress().Hex(),
			"value":         tx.Amount().String(),
			"gasLimit":      tx.MaxGas(),
			"gasPrice":      tx.MaxGasPrice(),
			"nonce":         tx.GetNonce(),
			"codeSizeBytes": len(tx.DeployData().Code()),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmDeploy", map[string]interface{}{
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
			"isDebug":     tx.GetIsDebug(),
			"commit":      true,
		})
	}
	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.Deploy( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.DeployData().Code(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmId, tx.Hash().Bytes(), tx.GetIsDebug(), isCache,
	)
	if span != nil { // GUARD
		span.AddEvent("MvmDeployFinished", map[string]interface{}{
			"status":        mvmResult.Status.String(),
			"exception":     mvmResult.Exception.String(),
			"gasUsed":       mvmResult.GasUsed,
			"returnLen":     len(mvmResult.Return),
			"returnHex":     hex.EncodeToString(mvmResult.Return),
			"balanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"nonceChange":   len(mvmResult.MapNonce) > 0,
			"codeChange":    len(mvmResult.MapCodeChange) > 0,
			"storageChange": len(mvmResult.MapStorageChange) > 0,
		})
		span.AddEvent("UpdatingStateDBAfterDeploy", nil)
	}

	_, err := vmP.updateStateDB(deployCtx, tx, mvmResult, mvmId, isFree) // Truyền deployCtx xuống
	if err != nil {
		wrappedErr := fmt.Errorf("failed to update state DB after deploy: %w", err)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("StateDBUpdateAfterDeployFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		rs, _ := vmP.MvmResultToExecuteResult(deployCtx, tx, mvmResult) // Vẫn convert
		return rs, wrappedErr
	}
	if span != nil { // GUARD
		span.AddEvent("StateDBUpdateAfterDeployFinished", nil)
	}

	rs, errConvert := vmP.MvmResultToExecuteResult(deployCtx, tx, mvmResult) // Truyền deployCtx
	if errConvert != nil {
		wrappedErr := fmt.Errorf("failed to convert MVM result to execute result after deploy: %w", errConvert)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("MvmResultToExecuteResultFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
	}

	if span != nil { // GUARD
		span.AddEvent("ClearingMVMApiAfterDeploy", map[string]interface{}{"mvmIdToClear": mvmId.Hex()})
	}
	logger.Error("ClearMVM: 3: %v", mvmId)

	mvm.ClearMVMApi(mvmId) // Luôn clear MVM API
	return rs, nil
}

// readOnlyCall xử lý lời gọi chỉ đọc.
func (vmP *VmProcessor) readOnlyCall(
	ctx context.Context,
	tx types.Transaction,
	mvmE *mvm.MVMApi,
) types.ExecuteSCResult {
	var span *trace.Span = nil // Khởi tạo nil
	// var readOnlyCtx context.Context = ctx // Không cần tạo context mới nếu không dùng
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.readOnlyCall", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"to":          tx.ToAddress().Hex(),
			"value":       tx.Amount().String(),
			"gasLimit":    tx.MaxGas(),
			"gasPrice":    tx.MaxGasPrice(),
			"inputLen":    len(tx.CallData().Input()),
			"inputHex":    hex.EncodeToString(tx.CallData().Input()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmCallReadOnly", map[string]interface{}{
			"commit":  true, // Giữ nguyên logic gốc commit=true
			"isDebug": tx.GetIsDebug(),
		})
		span.SetAttribute("mvmCallCommitFlag", true)
	}
	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.Call( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.CallData().Input(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), true, tx.Hash().Bytes(), tx.GetIsDebug(),
	)

	if span != nil { // GUARD
		span.AddEvent("MvmCallReadOnlyFinished", map[string]interface{}{
			"status":    mvmResult.Status.String(),
			"exception": mvmResult.Exception.String(),
			"gasUsed":   mvmResult.GasUsed,
			"returnLen": len(mvmResult.Return),
			"returnHex": hex.EncodeToString(mvmResult.Return),
		})
	}

	readOnlyMvmId := mvmE.GetKey()
	if span != nil { // GUARD
		span.AddEvent("ClearingMVMApiAfterReadOnlyCall", map[string]interface{}{"mvmIdToClear": readOnlyMvmId.Hex()})
	}
	logger.Error("ClearMVM: 4", readOnlyMvmId)

	mvm.ClearMVMApi(readOnlyMvmId) // Luôn clear MVM API

	rs := smart_contract.NewExecuteSCResult(tx.Hash(), mvmResult.Status, mvmResult.Exception, mvmResult.Return, mvmResult.GasUsed, common.Hash{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if span != nil { // GUARD
		span.SetAttribute("resultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("resultGasUsed", rs.GasUsed())
	}
	return rs
}

// executeSmartContract xử lý việc thực thi smart contract (call).
func (vmP *VmProcessor) executeSmartContract(
	ctx context.Context,
	tx types.Transaction,
	mvmE *mvm.MVMApi,
	isCache bool,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil        // Khởi tạo nil
	var execCtx context.Context = ctx // Mặc định dùng context vào
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		execCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.executeSmartContract", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"to":          tx.ToAddress().Hex(),
			"value":       tx.Amount().String(),
			"gasLimit":    tx.MaxGas(),
			"gasPrice":    tx.MaxGasPrice(),
			"nonce":       hex.EncodeToString(tx.GetNonce32Bytes()),
			"inputLen":    len(tx.CallData().Input()),
			"inputHex":    hex.EncodeToString(tx.CallData().Input()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmExecute", map[string]interface{}{
			"isDebug": tx.GetIsDebug(),
		})
	}
	var mvmResult *mvm.MVMExecuteResult
	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	if isCache {
		mvmResult = mvmE.Execute( // Luôn gọi MVM
			tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.CallData().Input(), tx.Amount(), tx.MaxGasPrice(), maxGas,
			lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
			lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), tx.Hash().Bytes(), tx.GetIsDebug(),
		)
	} else {
		mvmResult = mvmE.Call( // Luôn gọi MVM
			tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.CallData().Input(), tx.Amount(), tx.MaxGasPrice(), maxGas,
			lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
			lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), false, tx.Hash().Bytes(), tx.GetIsDebug(),
		)
	}
	if span != nil { // GUARD
		span.AddEvent("MvmExecuteFinished", map[string]interface{}{
			"status":        mvmResult.Status.String(),
			"exception":     mvmResult.Exception.String(),
			"gasUsed":       mvmResult.GasUsed,
			"returnLen":     len(mvmResult.Return),
			"returnHex":     hex.EncodeToString(mvmResult.Return),
			"balanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"nonceChange":   len(mvmResult.MapNonce) > 0,
			"codeChange":    len(mvmResult.MapCodeChange) > 0,
			"storageChange": len(mvmResult.MapStorageChange) > 0,
		})
	}

	currentMvmId := mvmE.GetKey()
	if span != nil { // GUARD
		span.AddEvent("UpdatingStateDBAfterExecute", map[string]interface{}{"mvmIdToUpdate": currentMvmId.Hex()})
	}

	_, err := vmP.updateStateDB(execCtx, tx, mvmResult, currentMvmId, isFree) // Pass execCtx xuống
	if err != nil {
		wrappedErr := fmt.Errorf("failed to update state DB after execute: %w", err)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("StateDBUpdateAfterExecuteFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		rs, _ := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult) // Vẫn convert
		return rs, wrappedErr
	}
	if span != nil { // GUARD
		span.AddEvent("StateDBUpdateAfterExecuteFinished", nil)
	}

	rs, errConvert := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult) // Pass execCtx
	if errConvert != nil {
		wrappedErr := fmt.Errorf("failed to convert MVM result to execute result after execute: %w", errConvert)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("MvmResultToExecuteResultFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
	}

	if span != nil { // GUARD
		span.AddEvent("MVMApiPersistsAfterExecute", map[string]interface{}{"mvmId": currentMvmId.Hex()})
	}
	if !isCache {
		mvm.ClearMVMApi(mvmE.GetKey())
	}
	return rs, nil
}

// executeSmartContract xử lý việc thực thi smart contract (call).
func (vmP *VmProcessor) sendNative(
	ctx context.Context,
	tx types.Transaction,
	mvmE *mvm.MVMApi,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil        // Khởi tạo nil
	var execCtx context.Context = ctx // Mặc định dùng context vào
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		execCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.sendNative", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"to":          tx.ToAddress().Hex(),
			"value":       tx.Amount().String(),
			"gasLimit":    tx.MaxGas(),
			"gasPrice":    tx.MaxGasPrice(),
			"nonce":       hex.EncodeToString(tx.GetNonce32Bytes()),
			"inputLen":    len(tx.CallData().Input()),
			"inputHex":    hex.EncodeToString(tx.CallData().Input()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmExecute", map[string]interface{}{
			"isDebug": tx.GetIsDebug(),
		})
	}
	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.SendNative( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(),
	)
	if span != nil { // GUARD
		span.AddEvent("MvmExecuteFinished", map[string]interface{}{
			"status":        mvmResult.Status.String(),
			"exception":     mvmResult.Exception.String(),
			"gasUsed":       mvmResult.GasUsed,
			"returnLen":     len(mvmResult.Return),
			"returnHex":     hex.EncodeToString(mvmResult.Return),
			"balanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"nonceChange":   len(mvmResult.MapNonce) > 0,
			"codeChange":    len(mvmResult.MapCodeChange) > 0,
			"storageChange": len(mvmResult.MapStorageChange) > 0,
		})
	}

	currentMvmId := mvmE.GetKey()
	if span != nil { // GUARD
		span.AddEvent("UpdatingStateDBAfterExecute", map[string]interface{}{"mvmIdToUpdate": currentMvmId.Hex()})
	}

	_, err := vmP.updateStateDB(execCtx, tx, mvmResult, currentMvmId, isFree) // Pass execCtx xuống
	if err != nil {
		wrappedErr := fmt.Errorf("failed to update state DB after execute: %w", err)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("StateDBUpdateAfterExecuteFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		rs, _ := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult) // Vẫn convert
		return rs, wrappedErr
	}
	if span != nil { // GUARD
		span.AddEvent("StateDBUpdateAfterExecuteFinished", nil)
	}

	rs, errConvert := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult) // Pass execCtx
	if errConvert != nil {
		wrappedErr := fmt.Errorf("failed to convert MVM result to execute result after execute: %w", errConvert)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("MvmResultToExecuteResultFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
	}

	if span != nil { // GUARD
		span.AddEvent("MVMApiPersistsAfterExecute", map[string]interface{}{"mvmId": currentMvmId.Hex()})
	}

	return rs, nil
}

// invalidTransactionResponse tạo kết quả lỗi cho giao dịch không hợp lệ.
func (vmP *VmProcessor) invalidTransactionResponse(
	ctx context.Context,
	tx types.Transaction,
	reason string, // Expect lowercase reason
) types.ExecuteSCResult {
	var span *trace.Span = nil // Khởi tạo nil

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.invalidTransactionResponse", map[string]interface{}{
			"txHash": tx.Hash().Hex(),
			"reason": reason,
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	errorBytes := []byte(reason)
	rs := smart_contract.NewErrorExecuteSCResult(tx.Hash(), *pb.RECEIPT_STATUS_THREW.Enum(), *pb.EXCEPTION_NONE.Enum(), errorBytes)

	if span != nil { // GUARD
		span.SetAttribute("resultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("resultException", rs.Exception().String())
		span.SetAttribute("errorBytesHex", hex.EncodeToString(errorBytes))
	}
	return rs
}

// MvmResultToExecuteResult chuyển đổi kết quả từ MVM sang ExecuteSCResult.
func (vmP *VmProcessor) MvmResultToExecuteResult(
	ctx context.Context,
	transaction types.Transaction,
	mvmRs *mvm.MVMExecuteResult,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil // Khởi tạo nil

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.MvmResultToExecuteResult", map[string]interface{}{
			"txHash":           transaction.Hash().Hex(),
			"mvmStatus":        mvmRs.Status.String(),
			"mvmException":     mvmRs.Exception.String(),
			"mvmGasUsed":       mvmRs.GasUsed,
			"mvmReturnLen":     len(mvmRs.Return),
			"mvmReturnHex":     hex.EncodeToString(mvmRs.Return),
			"numAddBalance":    len(mvmRs.MapAddBalance),
			"numSubBalance":    len(mvmRs.MapSubBalance),
			"numNonceChange":   len(mvmRs.MapNonce),
			"numCodeHash":      len(mvmRs.MapCodeHash),
			"numCodeChange":    len(mvmRs.MapCodeChange),
			"numStorageChange": len(mvmRs.MapStorageChange),
			"numEventLogs":     len(mvmRs.JEventLogs.Logs),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	transactionHash := transaction.Hash()

	// --- Revert Handling ---
	if mvmRs.Status != pb.RECEIPT_STATUS_RETURNED {
		if span != nil { // GUARD
			span.AddEvent("HandlingRevertedTransactionResult", map[string]interface{}{
				"status":    mvmRs.Status.String(),
				"exception": mvmRs.Exception.String(),
			})
		}
		amount := transaction.Amount()
		mapAddBalance := make(map[string][]byte)
		mapSubBalance := make(map[string][]byte)
		if len(mvmRs.MapAddBalance) > 0 || len(mvmRs.MapSubBalance) > 0 {
			// Sao chép map để tránh nil pointer nếu map ban đầu là nil (dù trường hợp này ít xảy ra)
			mapAddBalance = make(map[string][]byte, len(mvmRs.MapAddBalance))
			for k, v := range mvmRs.MapAddBalance {
				mapAddBalance[k] = v
			}
			mapSubBalance = make(map[string][]byte, len(mvmRs.MapSubBalance))
			for k, v := range mvmRs.MapSubBalance {
				mapSubBalance[k] = v
			}
			if span != nil { // GUARD
				span.AddEvent("UsingMvmBalanceMapsForRevert", map[string]interface{}{
					"addCount": len(mapAddBalance),
					"subCount": len(mapSubBalance),
				})
			}
		} else if amount.Cmp(big.NewInt(0)) > 0 {
			fromAddress := transaction.FromAddress()
			toAddress := transaction.ToAddress()
			fromAddressHex := hex.EncodeToString(fromAddress.Bytes())
			toAddressHex := hex.EncodeToString(toAddress.Bytes())
			mapAddBalance[fromAddressHex] = amount.Bytes()
			mapSubBalance[toAddressHex] = amount.Bytes()
			if span != nil { // GUARD
				span.AddEvent("UsingTxAmountForRevert", map[string]interface{}{
					"from":   fromAddressHex,
					"to":     toAddressHex,
					"amount": amount.String(),
				})
			}
		}

		rs := smart_contract.NewExecuteSCResult(
			transactionHash, mvmRs.Status, mvmRs.Exception, mvmRs.Return,
			mvmRs.GasUsed, common.Hash{}, mapAddBalance, mapSubBalance,
			mvmRs.MapNonce, nil, nil, nil, nil, nil, nil, nil,
		)
		if span != nil { // GUARD
			span.SetAttribute("finalResultStatus", rs.ReceiptStatus().String())
			span.SetAttribute("finalResultException", rs.Exception().String())
			span.SetAttribute("finalGasUsed", rs.GasUsed())
		}
		return rs, nil
	}

	// --- Success Handling ---
	if span != nil { // GUARD
		span.AddEvent("HandlingSuccessfulTransactionResult", nil)
	}

	// --- Storage Roots ---
	storageRoots := make(map[string][]byte)
	storageRootDetails := make(map[string]string)
	fetchErrors := []string{}
	if len(mvmRs.MapStorageChange) > 0 {
		storageAddresses := make([]string, 0, len(mvmRs.MapStorageChange))
		for address := range mvmRs.MapStorageChange {
			storageAddresses = append(storageAddresses, address)
		}
		if span != nil { // GUARD
			span.AddEvent("FetchingStorageRoots", map[string]interface{}{"addresses": storageAddresses})
		}
		for address := range mvmRs.MapStorageChange {
			addr := common.HexToAddress(address)
			as := vmP.chainState.GetAccountStateDB()
			if as != nil {
				accountState, err := as.AccountState(addr)
				if err != nil {
					wrappedErr := fmt.Errorf("error getting account state for %s: %w", address, err)
					errMsg := wrappedErr.Error()
					// fetchErrors = append(fetchErrors, errMsg)
					logger.Error(errMsg)
					storageRoots[address] = trie.EmptyRootHash.Bytes()
					storageRootDetails[address] = fmt.Sprintf("Error: %s -> %s", errMsg, trie.EmptyRootHash.Hex())
					panic(fmt.Sprintf("error getting account state : %v", err))
					// continue
				}
				if accountState != nil {
					smartContractState := accountState.SmartContractState()
					if smartContractState == nil {
						warnErr := fmt.Errorf("smart contract state not found for address %s during root fetch", address)
						warnMsg := warnErr.Error()
						logger.Warn(warnMsg)
						// fetchErrors = append(fetchErrors, warnMsg)
						storageRoots[address] = trie.EmptyRootHash.Bytes()
						storageRootDetails[address] = fmt.Sprintf("Warning: %s -> %s", warnMsg, trie.EmptyRootHash.Hex())
						panic(fmt.Sprintf("smart contract state not found for address : %v", err))

						// continue
					}
					root := smartContractState.StorageRoot()
					storageRoots[address] = root.Bytes()
					storageRootDetails[address] = root.Hex()
				} else {
					warnErr := fmt.Errorf("account state is nil for address %s during root fetch", address)
					warnMsg := warnErr.Error()
					logger.Warn(warnMsg)
					fetchErrors = append(fetchErrors, warnMsg)
					storageRoots[address] = trie.EmptyRootHash.Bytes()
					storageRootDetails[address] = fmt.Sprintf("Warning: %s -> %s", warnMsg, trie.EmptyRootHash.Hex())
				}
			} else {
				err := fmt.Errorf("account state db is nil")
				errMsg := err.Error()
				fetchErrors = append(fetchErrors, errMsg)
				logger.Error(errMsg)
				storageRoots[address] = trie.EmptyRootHash.Bytes()
				storageRootDetails[address] = fmt.Sprintf("Error: %s -> %s", errMsg, trie.EmptyRootHash.Hex())
			}
		}
		if span != nil { // GUARD
			if len(fetchErrors) > 0 {
				span.SetAttribute("storageRootFetchErrors", fetchErrors)
			}
			span.SetAttribute("storageRootsCollectedDetails", storageRootDetails)
		}
	}

	// --- Event Logs ---
	eventLogs := mvmRs.EventLogs(transactionHash)
	logsHash := smart_contract.GetLogsHash(eventLogs)
	if span != nil { // GUARD
		span.AddEvent("ProcessingEventLogs", map[string]interface{}{
			"eventLogCount": len(eventLogs),
			"logsHash":      logsHash.Hex(),
		})
	}
	if len(eventLogs) > 0 {
		logSummaries := []map[string]interface{}{}
		for i, log := range eventLogs {
			topic0Hex := "N/A"
			if len(log.Topics()) > 0 {
				topic0Hex = log.Topics()[0]
			}
			logSummaries = append(logSummaries, map[string]interface{}{
				"index": i, "address": log.Address().Hex(), "topic0": topic0Hex,
				"numTopics": len(log.Topics()), "dataSize": len(log.Data()),
			})
		}
		if span != nil { // GUARD
			span.SetAttribute("eventLogSummaries", logSummaries)
		}
		if vmP.chainState != nil && vmP.chainState.GetSmartContractDB() != nil {
			vmP.chainState.GetSmartContractDB().AddEventLogs(eventLogs) // DB Add happens regardless of trace
		} else {
			logger.Warn("Smart contract DB is nil, cannot add event logs")
		}
	}

	// --- Deploy Info ---
	mapCreatorPubkey := make(map[string][]byte)
	mapStorageAddress := make(map[string]common.Address)
	deployInfoDetails := make(map[string]map[string]string)
	extractErrors := []string{}
	if len(mvmRs.MapCodeHash) > 0 {
		codeHashAddresses := make([]string, 0, len(mvmRs.MapCodeHash))
		for a := range mvmRs.MapCodeHash {
			codeHashAddresses = append(codeHashAddresses, a)
		}
		if span != nil { // GUARD
			span.AddEvent("ExtractingDeployInfo", map[string]interface{}{"addresses": codeHashAddresses})
		}
		as := vmP.chainState.GetAccountStateDB()
		if as != nil {
			for a := range mvmRs.MapCodeHash {
				addr := common.HexToAddress(a)
				details := map[string]string{"address": a}
				accountState, err := as.AccountState(addr)
				if err != nil {
					wrappedErr := fmt.Errorf("error getting account state for deployed contract %s: %w", a, err)
					errMsg := wrappedErr.Error()
					extractErrors = append(extractErrors, errMsg)
					logger.Error(errMsg)
					details["error"] = errMsg
					deployInfoDetails[a] = details
					continue
				}
				if accountState != nil {
					scState := accountState.SmartContractState()
					if scState == nil {
						err := fmt.Errorf("smartContractState is nil for deployed contract %s", a)
						errMsg := err.Error()
						extractErrors = append(extractErrors, errMsg)
						logger.Error(errMsg)
						details["error"] = errMsg
						deployInfoDetails[a] = details
						continue
					}
					creatorKey := scState.CreatorPublicKey()
					storageAddr := scState.StorageAddress()
					mapCreatorPubkey[a] = creatorKey.Bytes()
					mapStorageAddress[a] = storageAddr
					details["creatorKey"] = hex.EncodeToString(creatorKey.Bytes())
					details["storageAddr"] = storageAddr.Hex()
					deployInfoDetails[a] = details
				} else {
					warnErr := fmt.Errorf("account state is nil for deployed contract %s", a)
					warnMsg := warnErr.Error()
					logger.Warn(warnMsg)
					extractErrors = append(extractErrors, warnMsg)
					details["error"] = warnMsg
					deployInfoDetails[a] = details
				}
			}
		} else {
			err := fmt.Errorf("account state db is nil")
			errMsg := err.Error()
			extractErrors = append(extractErrors, errMsg)
			logger.Error(errMsg)
		}
		if span != nil { // GUARD
			if len(extractErrors) > 0 {
				span.SetAttribute("deployInfoExtractErrors", extractErrors)
			}
			span.SetAttribute("deployInfoCollectedDetails", deployInfoDetails)
		}
	}

	// Enhance maps for tracing
	mapAddBalanceStr := mapBytesToString(mvmRs.MapAddBalance)
	mapSubBalanceStr := mapBytesToString(mvmRs.MapSubBalance)
	mapNonceStr := mapBytesToNonceString(mvmRs.MapNonce)
	mapCodeHashStr := mapBytesToHashString(mvmRs.MapCodeHash)
	mapStorageRootsStr := mapBytesToHashString(storageRoots)
	mapStorageAddressStr := mapAddressToString(mapStorageAddress)
	mapCreatorPubkeyStr := mapBytesToString(mapCreatorPubkey)

	rs := smart_contract.NewExecuteSCResult(
		transactionHash, mvmRs.Status, mvmRs.Exception, mvmRs.Return, mvmRs.GasUsed,
		logsHash,
		mvmRs.MapAddBalance, mvmRs.MapSubBalance, mvmRs.MapNonce,
		mvmRs.MapCodeHash, storageRoots,
		mapStorageAddress, mapCreatorPubkey,
		nil, nil,
		eventLogs,
	)

	if span != nil { // GUARD for final attributes
		span.SetAttribute("finalResultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("finalResultException", rs.Exception().String())
		span.SetAttribute("finalGasUsed", rs.GasUsed())
		span.SetAttribute("finalLogsHash", rs.LogsHash().Hex())
		span.SetAttribute("finalMapAddBalance", mapAddBalanceStr)
		span.SetAttribute("finalMapSubBalance", mapSubBalanceStr)
		span.SetAttribute("finalMapNonce", mapNonceStr)
		span.SetAttribute("finalMapCodeHash", mapCodeHashStr)
		span.SetAttribute("finalMapStorageRoots", mapStorageRootsStr)
		span.SetAttribute("finalMapStorageAddress", mapStorageAddressStr)
		span.SetAttribute("finalMapCreatorPubkey", mapCreatorPubkeyStr)
		span.SetAttribute("finalEventLogCount", len(rs.EventLogs()))
	}

	return rs, nil
}

func (vmP *VmProcessor) MvmResultToExecuteResultOffChain(
	ctx context.Context,
	transaction types.Transaction,
	mvmRs *mvm.MVMExecuteResult,
) (types.ExecuteSCResult, error) {

	transactionHash := transaction.Hash()

	return smart_contract.NewExecuteSCResult(
		transactionHash,
		mvmRs.Status,
		mvmRs.Exception,
		mvmRs.Return,
		mvmRs.GasUsed,
		common.Hash{},

		mvmRs.MapAddBalance,
		mvmRs.MapSubBalance,

		mvmRs.MapCodeHash,
		nil,

		nil,
		nil,

		nil,

		nil,
		nil,
		nil,
	), nil
}

// updateStateDB cập nhật trạng thái DB dựa trên kết quả MVM.
func (vmP *VmProcessor) updateStateDB(
	ctx context.Context,
	transaction types.Transaction,
	mvmRs *mvm.MVMExecuteResult,
	mvmId common.Address,
	isFreeGass bool,
) (bool, error) {
	var span *trace.Span = nil // Khởi tạo nil

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.updateStateDB", map[string]interface{}{
			"txHash":       transaction.Hash().Hex(),
			"mvmStatus":    mvmRs.Status.String(),
			"mvmException": mvmRs.Exception.String(),
			"mvmId":        mvmId.Hex(),
			"isReverted":   mvmRs.Status == pb.RECEIPT_STATUS_THREW || mvmRs.Status == pb.RECEIPT_STATUS_HALTED,
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	hasChanges := false
	changesSummary := make(map[string]interface{})
	updateErrors := []string{}
	var finalErr error

	// --- Revert Handling ---
	if mvmRs.Status == pb.RECEIPT_STATUS_THREW || mvmRs.Status == pb.RECEIPT_STATUS_HALTED {
		if span != nil { // GUARD
			span.AddEvent("HandlingRevertedState", map[string]interface{}{
				"status":    mvmRs.Status.String(),
				"exception": mvmRs.Exception.String(),
			})
		}
		trie_database.GetTrieDatabaseManager().FindAndSetTrieDatabasesByMvmID(mvmId, trie_database.Reverted)
		if span != nil { // GUARD
			span.AddEvent("MarkedTrieDBAsReverted", map[string]interface{}{"mvmId": mvmId.Hex()})
		}
		amount := transaction.Amount()
		if amount.Cmp(big.NewInt(0)) > 0 {
			fromAddr := transaction.FromAddress()
			toAddr := transaction.ToAddress()
			if span != nil { // GUARD
				span.AddEvent("RevertingTransactionAmount", map[string]interface{}{
					"amount": amount.String(),
					"from":   fromAddr.Hex(),
					"to":     toAddr.Hex(),
				})
				span.SetAttribute("revertedAmountFrom", fromAddr.Hex())
				span.SetAttribute("revertedAmountTo", toAddr.Hex())
			}
			vmP.chainState.GetAccountStateDB().AddPendingBalance(fromAddr, amount)
			vmP.chainState.GetAccountStateDB().SubTotalBalance(toAddr, amount)
		}
		if span != nil { // GUARD before return
			span.SetAttribute("hasChanges", false)
			span.SetAttribute("changesSummary", changesSummary)
			if len(updateErrors) > 0 {
				span.SetAttribute("updateWarningsOrErrors", updateErrors)
			}
		}

		if len(mvmRs.MapNonce) > 0 {
			details := make(map[string]string)
			fatalError := false
			for address, nonceBytes := range mvmRs.MapNonce {
				fmtAddress := common.HexToAddress(address)
				newNonceBig := big.NewInt(0).SetBytes(nonceBytes)
				newNonce, err := utils.BigIntToUint64(newNonceBig)
				if err != nil {
					errMsg := fmt.Sprintf("failed to convert nonce %s for %s: %v", newNonceBig.String(), address, err)
					if span != nil { // GUARD
						span.SetAttribute("nonceConversionError_"+address, errMsg)
					}
					logger.Warn(errMsg)
					updateErrors = append(updateErrors, errMsg)
					details[address] = fmt.Sprintf("ConversionError: %s", newNonceBig.String())
					continue
				}
				err = vmP.chainState.GetAccountStateDB().SetNonce(fmtAddress, newNonce)
				if err != nil {
					finalErr = fmt.Errorf("failed to set nonce %d for %s: %w", newNonce, address, err)
					if span != nil { // GUARD
						span.SetError(finalErr)
					}
					logger.Error(finalErr.Error())
					updateErrors = append(updateErrors, finalErr.Error())
					details[address] = fmt.Sprintf("SetError: %d", newNonce)
					fatalError = true
					break
				} else {
					details[address] = strconv.FormatUint(newNonce, 10)
				}
			}
			if span != nil { // GUARD
				span.AddEvent("UpdatingNonces", map[string]interface{}{"count": len(mvmRs.MapNonce), "details": details})
			}
			changesSummary["nonce"] = details
			if len(mvmRs.MapNonce) > 0 {
				hasChanges = true
			}
			if fatalError {
				if span != nil { // GUARD before return
					span.SetAttribute("hasChanges", hasChanges)
					span.SetAttribute("changesSummary", changesSummary)
					if len(updateErrors) > 0 {
						span.SetAttribute("updateWarningsOrErrors", updateErrors)
					}
				}
				return hasChanges, finalErr
			}
		}

		return false, nil
	}

	// --- Success Handling ---
	if span != nil { // GUARD
		span.AddEvent("HandlingSuccessfulStateUpdate", nil)
	}

	// --- AddBalance ---
	if len(mvmRs.MapAddBalance) > 0 {
		details := make(map[string]string)
		for address, addAmountBytes := range mvmRs.MapAddBalance {
			fmtAddress := common.HexToAddress(address)
			addAmount := big.NewInt(0).SetBytes(addAmountBytes)
			vmP.chainState.GetAccountStateDB().AddPendingBalance(fmtAddress, addAmount)
			details[address] = addAmount.String()
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingAddedBalances", map[string]interface{}{"count": len(mvmRs.MapAddBalance), "details": details})
		}
		changesSummary["addBalance"] = details
		hasChanges = true
	}

	// --- SubBalance ---
	if len(mvmRs.MapSubBalance) > 0 {
		details := make(map[string]string)
		fatalError := false
		for address, subAmountBytes := range mvmRs.MapSubBalance {
			fmtAddress := common.HexToAddress(address)
			subAmount := big.NewInt(0).SetBytes(subAmountBytes)
			logger.Error("subAmount: %s", subAmount.String())
			logger.Error("fmtAddress: %s", fmtAddress)

			err := vmP.chainState.GetAccountStateDB().SubTotalBalance(fmtAddress, subAmount)
			if err != nil {
				finalErr = fmt.Errorf("failed to subtract total balance for %s (amount: %s): %w", address, subAmount.String(), err)
				if span != nil { // GUARD
					span.SetError(finalErr)
				}
				logger.Error(finalErr.Error())
				updateErrors = append(updateErrors, finalErr.Error())
				details[address] = fmt.Sprintf("Error: %s", subAmount.String())
				fatalError = true
				break
			} else {
				details[address] = subAmount.String()
			}
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingSubtractedBalances", map[string]interface{}{"count": len(mvmRs.MapSubBalance), "details": details})
		}
		changesSummary["subBalance"] = details
		if len(mvmRs.MapSubBalance) > 0 {
			hasChanges = true
		}
		if fatalError {
			if span != nil { // GUARD before return
				span.SetAttribute("hasChanges", hasChanges)
				span.SetAttribute("changesSummary", changesSummary)
				if len(updateErrors) > 0 {
					span.SetAttribute("updateWarningsOrErrors", updateErrors)
				}
			}
			return hasChanges, finalErr
		}
	}

	// --- Nonce ---
	if len(mvmRs.MapNonce) > 0 {
		details := make(map[string]string)
		fatalError := false
		for address, nonceBytes := range mvmRs.MapNonce {
			fmtAddress := common.HexToAddress(address)
			newNonceBig := big.NewInt(0).SetBytes(nonceBytes)
			newNonce, err := utils.BigIntToUint64(newNonceBig)
			if err != nil {
				errMsg := fmt.Sprintf("failed to convert nonce %s for %s: %v", newNonceBig.String(), address, err)
				if span != nil { // GUARD
					span.SetAttribute("nonceConversionError_"+address, errMsg)
				}
				logger.Warn(errMsg)
				updateErrors = append(updateErrors, errMsg)
				details[address] = fmt.Sprintf("ConversionError: %s", newNonceBig.String())
				continue
			}
			err = vmP.chainState.GetAccountStateDB().SetNonce(fmtAddress, newNonce)
			if err != nil {
				finalErr = fmt.Errorf("failed to set nonce %d for %s: %w", newNonce, address, err)
				if span != nil { // GUARD
					span.SetError(finalErr)
				}
				logger.Error(finalErr.Error())
				updateErrors = append(updateErrors, finalErr.Error())
				details[address] = fmt.Sprintf("SetError: %d", newNonce)
				fatalError = true
				break
			} else {
				details[address] = strconv.FormatUint(newNonce, 10)
			}
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingNonces", map[string]interface{}{"count": len(mvmRs.MapNonce), "details": details})
		}
		changesSummary["nonce"] = details
		if len(mvmRs.MapNonce) > 0 {
			hasChanges = true
		}
		if fatalError {
			if span != nil { // GUARD before return
				span.SetAttribute("hasChanges", hasChanges)
				span.SetAttribute("changesSummary", changesSummary)
				if len(updateErrors) > 0 {
					span.SetAttribute("updateWarningsOrErrors", updateErrors)
				}
			}
			return hasChanges, finalErr
		}
	}

	// --- Deploy State ---
	if len(mvmRs.MapCodeHash) > 0 {
		details := make(map[string]map[string]string)
		var creatorPublicKey mt_common.PublicKey
		var storageAddress common.Address
		determinedDeployInfo := false
		fatalError := false
		// Determine creator/storage...
		if transaction.IsDeployContract() {
			storageAddress = transaction.DeployData().StorageAddress()
			determinedDeployInfo = true
		} else if transaction.IsCallContract() {
			originSmartContractAs, err := vmP.chainState.GetAccountStateDB().AccountState(transaction.ToAddress())
			if err != nil {
				finalErr = fmt.Errorf("failed to get origin contract state %s for internal deploy: %w", transaction.ToAddress().Hex(), err)
				fatalError = true
			} else if originSmartContractAs == nil || originSmartContractAs.SmartContractState() == nil {
				finalErr = errors.New("origin contract state " + transaction.ToAddress().Hex() + " is nil or not a contract for internal deploy")
				fatalError = true
			} else {
				originScState := originSmartContractAs.SmartContractState()
				creatorPublicKey = originScState.CreatorPublicKey()
				storageAddress = originScState.StorageAddress()
				determinedDeployInfo = true
				if span != nil { // GUARD
					span.AddEvent("DeterminedDeployInfoFromOriginContract", map[string]interface{}{
						"originContract": transaction.ToAddress().Hex(), "creatorKey": hex.EncodeToString(creatorPublicKey.Bytes()), "storageAddr": storageAddress.Hex(),
					})
				}
			}
			if fatalError {
				if span != nil {
					span.SetError(finalErr)
				}
				logger.Error(finalErr.Error())
				updateErrors = append(updateErrors, finalErr.Error())
				if span != nil { // GUARD before return
					span.SetAttribute("hasChanges", hasChanges)
					span.SetAttribute("changesSummary", changesSummary)
					if len(updateErrors) > 0 {
						span.SetAttribute("updateWarningsOrErrors", updateErrors)
					}
				}
				return hasChanges, finalErr
			}
		}
		if !determinedDeployInfo {
			finalErr = errors.New("could not determine creator/storage info for deploy state update")
			if span != nil {
				span.SetError(finalErr)
			}
			logger.Error(finalErr.Error())
			if span != nil { // GUARD before return
				span.SetAttribute("hasChanges", hasChanges)
				span.SetAttribute("changesSummary", changesSummary)
				if len(updateErrors) > 0 {
					span.SetAttribute("updateWarningsOrErrors", updateErrors)
				}
			}
			return hasChanges, finalErr
		}
		// Apply state...
		for address, newCodeHashBytes := range mvmRs.MapCodeHash {
			addrDetails := map[string]string{}
			fmtAddress := common.HexToAddress(address)
			newCodeHash := common.BytesToHash(newCodeHashBytes)
			addrDetails["codeHash"] = newCodeHash.Hex()
			asState, err := vmP.chainState.GetAccountStateDB().AccountState(fmtAddress)
			if err != nil {
				finalErr = fmt.Errorf("error getting account state for new contract %s: %w", address, err)
				fatalError = true
			} else if asState == nil {
				finalErr = errors.New("account state is nil for new contract " + address + " after MVM execution")
				fatalError = true
			}
			if fatalError {
				if span != nil {
					span.SetError(finalErr)
				}
				logger.Error(finalErr.Error())
				addrDetails["error"] = finalErr.Error()
				details[address] = addrDetails
				break
			}
			asState.SetCreatorPublicKey(creatorPublicKey)
			asState.SetStorageAddress(storageAddress)
			asState.SetCodeHash(newCodeHash)
			addrDetails["creatorKeySet"] = hex.EncodeToString(creatorPublicKey.Bytes())
			addrDetails["storageAddrSet"] = storageAddress.Hex()
			if _, storageChanged := mvmRs.MapStorageChange[address]; !storageChanged {
				asState.SetStorageRoot(trie.EmptyRootHash)
				addrDetails["storageRootSet"] = trie.EmptyRootHash.Hex()
				if span != nil { // GUARD
					span.AddEvent("SetEmptyStorageRootForNewContract", map[string]interface{}{"address": address})
				}
			} else {
				addrDetails["storageRootSet"] = "(Deferred)"
			}
			vmP.chainState.GetAccountStateDB().SetState(asState)
			details[address] = addrDetails
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingDeployedContractsState", map[string]interface{}{"count": len(mvmRs.MapCodeHash), "details": details})
		}
		changesSummary["deployState"] = details
		if len(mvmRs.MapCodeHash) > 0 {
			hasChanges = true
		}
		if fatalError {
			if span != nil { // GUARD before return
				span.SetAttribute("hasChanges", hasChanges)
				span.SetAttribute("changesSummary", changesSummary)
				if len(updateErrors) > 0 {
					span.SetAttribute("updateWarningsOrErrors", updateErrors)
				}
			}
			return hasChanges, finalErr
		}
	}

	// --- Code Change ---
	if len(mvmRs.MapCodeChange) > 0 {
		details := make(map[string]map[string]string)
		for address, code := range mvmRs.MapCodeChange {
			addrDetails := map[string]string{}
			fmtAddress := common.HexToAddress(address)
			codeHashBytes, ok := mvmRs.MapCodeHash[address]
			if !ok {
				errMsg := fmt.Sprintf("code hash not found for code change at address %s. Skipping code save.", address)
				if span != nil { // GUARD
					span.SetAttribute("missingCodeHashError_"+address, errMsg)
				}
				logger.Warn(errMsg)
				updateErrors = append(updateErrors, errMsg)
				addrDetails["error"] = errMsg
				addrDetails["codeSize"] = strconv.Itoa(len(code))
				details[address] = addrDetails
				continue
			}
			codeHash := common.BytesToHash(codeHashBytes)
			vmP.chainState.GetSmartContractDB().SetCode(fmtAddress, codeHash, code)
			addrDetails["codeHash"] = codeHash.Hex()
			addrDetails["codeSize"] = strconv.Itoa(len(code))
			details[address] = addrDetails
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingContractCode", map[string]interface{}{"count": len(mvmRs.MapCodeChange), "details": details})
		}
		changesSummary["codeChange"] = details
		if len(mvmRs.MapCodeChange) > 0 {
			hasChanges = true
		}
	}

	// --- Storage Change ---
	if len(mvmRs.MapStorageChange) > 0 {
		details := make(map[string]map[string]string)
		totalSlotsUpdated := 0
		for address, rawStorages := range mvmRs.MapStorageChange {
			slotDetails := make(map[string]string)
			fmtAddress := common.HexToAddress(address)
			for slotHex, value := range rawStorages {
				slotBytes := common.FromHex(slotHex)
				vmP.chainState.GetSmartContractDB().SetStorageValue(fmtAddress, slotBytes, value)
				slotDetails[slotHex] = hex.EncodeToString(value)
				totalSlotsUpdated++
			}
			details[address] = slotDetails
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingContractStorage", map[string]interface{}{
				"addressCount": len(mvmRs.MapStorageChange),
				"totalSlots":   totalSlotsUpdated,
				"details":      details,
			})
		}
		changesSummary["storageSlots"] = details
		if len(mvmRs.MapStorageChange) > 0 {
			hasChanges = true
		}
	}

	// --- Storage Root Update ---
	if len(mvmRs.MapStorageChange) > 0 {
		details := make(map[string]string)
		fatalError := false
		for address := range mvmRs.MapStorageChange {
			fmtAddress := common.HexToAddress(address)
			newStorageRoot := vmP.chainState.GetSmartContractDB().StorageRoot(fmtAddress)
			err := vmP.chainState.GetAccountStateDB().SetStorageRoot(fmtAddress, newStorageRoot)
			if err != nil {
				finalErr = fmt.Errorf("failed to set storage root %s for %s: %w", newStorageRoot.Hex(), address, err)
				if span != nil { // GUARD
					span.SetError(finalErr)
				}
				logger.Error(finalErr.Error())
				updateErrors = append(updateErrors, finalErr.Error())
				details[address] = fmt.Sprintf("SetError: %s", newStorageRoot.Hex())
				fatalError = true
				break
			} else {
				details[address] = newStorageRoot.Hex()
			}
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingStorageRootsInAccountState", map[string]interface{}{"count": len(mvmRs.MapStorageChange), "details": details})
		}
		changesSummary["storageRootUpdate"] = details
		if fatalError {
			if span != nil { // GUARD before return
				span.SetAttribute("hasChanges", hasChanges)
				span.SetAttribute("changesSummary", changesSummary)
				if len(updateErrors) > 0 {
					span.SetAttribute("updateWarningsOrErrors", updateErrors)
				}
			}
			return hasChanges, finalErr
		}
	}

	// --- MapFullDbHash ---
	if len(mvmRs.MapFullDbHash) > 0 {
		details := make(map[string]map[string]string)
		dbHashUpdateErrors := []string{}
		for addressHex, newHashBytes := range mvmRs.MapFullDbHash {
			addrDetails := map[string]string{"newPartialHash": common.Bytes2Hex(newHashBytes)}
			fmtAddress := common.HexToAddress(addressHex)
			accountState, err := vmP.chainState.GetAccountStateDB().AccountState(fmtAddress)
			if err != nil {
				errMsg := fmt.Sprintf("error getting account state for %s during MapFullDbHash update: %v", addressHex, err)
				dbHashUpdateErrors = append(dbHashUpdateErrors, errMsg)
				logger.Error(errMsg)
				addrDetails["error"] = errMsg
				details[addressHex] = addrDetails
				continue
			}
			smartContractState := accountState.SmartContractState()
			if smartContractState == nil {
				errMsg := fmt.Sprintf("account %s does not have SmartContractState, skipping MapFullDbHash update.", addressHex)
				dbHashUpdateErrors = append(dbHashUpdateErrors, errMsg)
				logger.Warn(errMsg)
				addrDetails["warning"] = errMsg
				details[addressHex] = addrDetails
				continue
			}
			currentMapFullDbHash := smartContractState.MapFullDbHash()
			newMapFullDbHash := common.BytesToHash(newHashBytes)
			combinedHash := combineHashes(currentMapFullDbHash, newMapFullDbHash)
			smartContractState.SetMapFullDbHash(combinedHash)
			vmP.chainState.GetAccountStateDB().SetState(accountState)
			addrDetails["previousHash"] = currentMapFullDbHash.Hex()
			addrDetails["combinedHash"] = combinedHash.Hex()
			details[addressHex] = addrDetails
		}
		if span != nil { // GUARD
			span.AddEvent("UpdatingMapFullDbHash", map[string]interface{}{"count": len(mvmRs.MapFullDbHash), "details": details})
			if len(dbHashUpdateErrors) > 0 {
				span.SetAttribute("mapFullDbHashUpdateErrors", dbHashUpdateErrors)
			}
		}
		changesSummary["mapFullDbHash"] = details
		if len(mvmRs.MapFullDbHash) > 0 {
			hasChanges = true
		}
	}

	if mvmRs.GasUsed > 0 && !isFreeGass {
		fromAddr := transaction.FromAddress()

		vmP.chainState.GetAccountStateDB().SubTotalBalance(fromAddr, big.NewInt(int64(mvmRs.GasUsed)))

	}

	// --- Final Return ---
	if span != nil { // GUARD
		span.SetAttribute("hasChanges", hasChanges)
		span.SetAttribute("changesSummary", changesSummary)
		if len(updateErrors) > 0 {
			span.SetAttribute("updateWarningsOrErrors", updateErrors)
		}
	}
	return hasChanges, finalErr // finalErr will be nil if no fatal error occurred
}

// --- Helper functions for tracing (mapBytesToString, etc.) ---
func mapBytesToString(m map[string][]byte) map[string]string {
	if m == nil {
		return nil
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		res[k] = hex.EncodeToString(v)
	}
	return res
}
func mapBytesToNonceString(m map[string][]byte) map[string]string {
	if m == nil {
		return nil
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		val := big.NewInt(0).SetBytes(v)
		u64Val, err := utils.BigIntToUint64(val)
		if err != nil {
			res[k] = fmt.Sprintf("ErrorConv(%s)", val.String())
		} else {
			res[k] = strconv.FormatUint(u64Val, 10)
		}
	}
	return res
}
func mapBytesToHashString(m map[string][]byte) map[string]string {
	if m == nil {
		return nil
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		res[k] = common.BytesToHash(v).Hex()
	}
	return res
}
func mapAddressToString(m map[string]common.Address) map[string]string {
	if m == nil {
		return nil
	}
	res := make(map[string]string, len(m))
	for k, v := range m {
		res[k] = v.Hex()
	}
	return res
}

// --- End Helper functions ---

// combineHashes kết hợp hai hash.
func combineHashes(hash1, hash2 common.Hash) common.Hash {
	if hash1 == (common.Hash{}) {
		return hash2
	}
	if hash2 == (common.Hash{}) {
		return hash1
	}
	combinedBytes := append(hash1.Bytes(), hash2.Bytes()...)
	combinedHashBytes := crypto.Keccak256(combinedBytes)
	return common.BytesToHash(combinedHashBytes)
}

// isValidSmartContractCall kiểm tra tính hợp lệ của lời gọi SC.
func (vmP *VmProcessor) IsValidSmartContractCall(toAccountState types.AccountState, tx types.Transaction) bool {
	if toAccountState == nil {
		logger.Debug("isValidSmartContractCall: false (toAccountState is nil)")
		return false
	}
	scState := toAccountState.SmartContractState()
	if scState == nil {
		logger.Debug("isValidSmartContractCall: false (scState is nil)")
		return false
	}
	expectedStorageRoot := vmP.chainState.GetSmartContractDB().StorageRoot(tx.ToAddress())
	actualStorageRoot := scState.StorageRoot()
	isValid := actualStorageRoot == expectedStorageRoot
	if !isValid {
		logger.Debug("isValidSmartContractCall: false (storage root mismatch)", "expected", expectedStorageRoot.Hex(), "actual", actualStorageRoot.Hex(), "address", tx.ToAddress().Hex())
	} else {
		logger.Debug("isValidSmartContractCall: true", "address", tx.ToAddress().Hex())
	}
	return isValid
}

// --- Debug and Sub functions ---

func (vmP *VmProcessor) ExecuteTransactionWithMvmIdDebug(
	ctx context.Context,
	tx types.Transaction, extendedMode bool,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil         // Khởi tạo nil
	var debugCtx context.Context = ctx // Mặc định dùng context vào
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		debugCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.ExecuteTransactionWithMvmIdDebug", map[string]interface{}{
			"txHash":       tx.Hash().Hex(),
			"from":         tx.FromAddress().Hex(),
			"to":           tx.ToAddress().Hex(),
			"value":        tx.Amount().String(),
			"gasLimit":     tx.MaxGas(),
			"gasPrice":     tx.MaxGasPrice(),
			"nonce":        tx.GetNonce(),
			"extendedMode": extendedMode,
			"blockNumber":  lastBlockHeader.BlockNumber() + 1,
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	combinedHash := sha256.Sum256([]byte(fmt.Sprintf("%x%d%d", tx.Hash(), rand.Int63(), time.Now().UnixNano())))
	ethAddressBytes := combinedHash[12:]
	mvmIdDebug := common.BytesToAddress(ethAddressBytes)
	if span != nil { // GUARD
		span.SetAttribute("debugMvmId", mvmIdDebug.Hex())
	}

	mvmDebug := mvm.GetOrCreateMVMApi(mvmIdDebug, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), extendedMode)
	mvmDebug.SetRelatedAddresses(tx.RelatedAddresses())

	result := vmP.callDebug(debugCtx, tx, mvmDebug) // Truyền debugCtx
	if span != nil {                                // GUARD
		span.SetAttribute("debugResultStatus", result.ReceiptStatus().String())
		span.SetAttribute("debugResultGasUsed", result.GasUsed())
		span.SetAttribute("debugResultReturnHex", hex.EncodeToString(result.Return()))
	}
	logger.Error("ClearMVM: 4", mvmIdDebug)

	mvm.ClearMVMApi(mvmIdDebug) // Luôn clear
	if span != nil {            // GUARD
		span.AddEvent("ClearedDebugMVMApi", map[string]interface{}{"mvmIdCleared": mvmIdDebug.Hex()})
	}

	return result, nil
}

func (vmP *VmProcessor) callDebug(
	ctx context.Context,
	tx types.Transaction, mvmE *mvm.MVMApi,
) types.ExecuteSCResult {
	var span *trace.Span = nil             // Khởi tạo nil
	var callDebugCtx context.Context = ctx // Mặc định dùng context vào
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		callDebugCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.callDebug", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"to":          tx.ToAddress().Hex(),
			"value":       tx.Amount().String(),
			"inputLen":    len(tx.CallData().Input()),
			"inputHex":    hex.EncodeToString(tx.CallData().Input()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmCallDebug", map[string]interface{}{
			"commit":  false,
			"isDebug": tx.GetIsDebug(),
			"nonce":   hex.EncodeToString(tx.GetNonce32Bytes()),
		})
	}

	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.Call( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.CallData().Input(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), false, tx.Hash().Bytes(), tx.GetIsDebug(),
	)

	if span != nil { // GUARD
		span.AddEvent("MvmCallDebugFinished", map[string]interface{}{
			"status":           mvmResult.Status.String(),
			"exception":        mvmResult.Exception.String(),
			"gasUsed":          mvmResult.GasUsed,
			"returnLen":        len(mvmResult.Return),
			"returnHex":        hex.EncodeToString(mvmResult.Return),
			"numLogs":          len(mvmResult.JEventLogs.Logs),
			"potBalanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"potNonceChange":   len(mvmResult.MapNonce) > 0,
			"potCodeChange":    len(mvmResult.MapCodeChange) > 0,
			"potStorageChange": len(mvmResult.MapStorageChange) > 0,
		})
	}

	rs, err := vmP.mvmResultToExecuteResultDebug(callDebugCtx, tx, mvmResult, lastBlockHeader) // Truyền callDebugCtx
	if err != nil {
		wrappedErr := fmt.Errorf("error converting debug MVM result: %w", err)
		if span != nil { // GUARD
			span.SetError(wrappedErr)
			span.AddEvent("MvmResultConversionDebugFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		errorBytes := []byte(wrappedErr.Error())
		return smart_contract.NewErrorExecuteSCResult(tx.Hash(), *pb.RECEIPT_STATUS_TRANSACTION_ERROR.Enum(), *pb.EXCEPTION_NONE.Enum(), errorBytes)
	}

	if span != nil { // GUARD
		span.SetAttribute("resultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("resultException", rs.Exception().String())
		span.SetAttribute("resultGasUsed", rs.GasUsed())
	}
	return rs
}

func (vmP *VmProcessor) mvmResultToExecuteResultDebug(
	ctx context.Context,
	transaction types.Transaction,
	mvmRs *mvm.MVMExecuteResult,
	lastBlockHeader types.BlockHeader,
) (types.ExecuteSCResult, error) {
	var span *trace.Span = nil // Khởi tạo nil

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.mvmResultToExecuteResultDebug", map[string]interface{}{
			"txHash":       transaction.Hash().Hex(),
			"mvmStatus":    mvmRs.Status.String(),
			"mvmException": mvmRs.Exception.String(),
			"mvmGasUsed":   mvmRs.GasUsed,
			"mvmReturnLen": len(mvmRs.Return),
			"mvmReturnHex": hex.EncodeToString(mvmRs.Return),
			"numLogs":      len(mvmRs.JEventLogs.Logs),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	transactionHash := transaction.Hash()
	eventLogs := mvmRs.EventLogs(transactionHash)
	logsHash := smart_contract.GetLogsHash(eventLogs)

	if span != nil { // GUARD
		span.AddEvent("ProcessingDebugResult", map[string]interface{}{
			"status":    mvmRs.Status.String(),
			"exception": mvmRs.Exception.String(),
			"gasUsed":   mvmRs.GasUsed,
			"returnHex": hex.EncodeToString(mvmRs.Return),
			"logCount":  len(eventLogs),
			"logsHash":  logsHash.Hex(),
		})
	}

	if len(eventLogs) > 0 {
		logSummaries := []map[string]interface{}{}
		for i, log := range eventLogs {
			topic0Hex := "N/A"
			if len(log.Topics()) > 0 {
				topic0Hex = log.Topics()[0]
			}
			logSummaries = append(logSummaries, map[string]interface{}{
				"index": i, "address": log.Address().Hex(), "topic0": topic0Hex,
				"numTopics": len(log.Topics()), "dataSize": len(log.Data()),
			})
		}
		if span != nil { // GUARD
			span.SetAttribute("debugEventLogSummaries", logSummaries)
		}
	}

	rs := smart_contract.NewExecuteSCResult(
		transactionHash, mvmRs.Status, mvmRs.Exception, mvmRs.Return, mvmRs.GasUsed,
		logsHash, nil, nil, nil, nil, nil, nil, nil, nil, nil, eventLogs,
	)

	if span != nil { // GUARD
		span.SetAttribute("finalDebugResultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("finalDebugResultException", rs.Exception().String())
		span.SetAttribute("finalDebugResultGasUsed", rs.GasUsed())
		span.SetAttribute("finalDebugLogsHash", rs.LogsHash().Hex())
		span.SetAttribute("finalDebugEventLogCount", len(rs.EventLogs()))
	}

	return rs, nil
}

func (vmP *VmProcessor) ExecuteTransactionWithMvmIdSub(
	ctx context.Context,
	tx types.Transaction, extendedMode bool,
) (types.ExecuteSCResult, bool, error) {
	var span *trace.Span = nil       // Khởi tạo nil
	var subCtx context.Context = ctx // Mặc định dùng context vào

	logger.Warn("ExecuteTransactionWithMvmIdSub using potentially shared MVM ID", "mvmId", vmP.mvmId.Hex())

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		subCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.ExecuteTransactionWithMvmIdSub", map[string]interface{}{
			"txHash":       tx.Hash().Hex(),
			"from":         tx.FromAddress().Hex(),
			"to":           tx.ToAddress().Hex(),
			"value":        tx.Amount().String(),
			"extendedMode": extendedMode,
			"mvmIdUsed":    vmP.mvmId.Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	mvmSub := mvm.GetOrCreateMVMApi(vmP.mvmId, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), extendedMode)
	mvmSub.SetRelatedAddresses(tx.RelatedAddresses())

	rs, status := vmP.onlyCall(subCtx, tx, mvmSub) // Truyền subCtx

	if span != nil { // GUARD
		span.AddEvent("SubCallResult", map[string]interface{}{
			"resultStatus": rs.ReceiptStatus().String(),
			"resultGas":    rs.GasUsed(),
			"stateChanged": status,
		})
	}

	return rs, status, nil
}

func (vmP *VmProcessor) onlyCall(
	ctx context.Context,
	tx types.Transaction, mvmE *mvm.MVMApi,
) (types.ExecuteSCResult, bool) {
	var span *trace.Span = nil // Khởi tạo nil
	// var onlyCallCtx context.Context = ctx // Không cần nếu không truyền xuống
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.onlyCall", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"to":          tx.ToAddress().Hex(),
			"value":       tx.Amount().String(),
			"inputLen":    len(tx.CallData().Input()),
			"inputHex":    hex.EncodeToString(tx.CallData().Input()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmCallSub", map[string]interface{}{
			"commit":      false,
			"isDebug":     tx.GetIsDebug(),
			"nonceSource": "LastHash",
		})
	}
	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.Call( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.ToAddress().Bytes(), tx.CallData().Input(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), false, tx.Hash().Bytes(), tx.GetIsDebug(),
	)

	if span != nil { // GUARD
		span.AddEvent("MvmCallSubFinished", map[string]interface{}{
			"status":           mvmResult.Status.String(),
			"exception":        mvmResult.Exception.String(),
			"gasUsed":          mvmResult.GasUsed,
			"returnLen":        len(mvmResult.Return),
			"returnHex":        hex.EncodeToString(mvmResult.Return),
			"numLogs":          len(mvmResult.JEventLogs.Logs),
			"potBalanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"potNonceChange":   len(mvmResult.MapNonce) > 0,
			"potCodeChange":    len(mvmResult.MapCodeChange) > 0,
			"potStorageChange": len(mvmResult.MapStorageChange) > 0,
			"potFullDbChange":  len(mvmResult.MapFullDbHash) > 0,
		})
	}

	// checkStateDBStatus không trace, dùng ctx gốc
	stateChanged := vmP.checkStateDBStatus(ctx, tx, mvmResult)
	if span != nil { // GUARD
		span.SetAttribute("potentialStateChangeReported", stateChanged)
	}

	rs := smart_contract.NewExecuteSCResult(tx.Hash(), mvmResult.Status, mvmResult.Exception, mvmResult.Return, mvmResult.GasUsed, common.Hash{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if span != nil { // GUARD
		span.SetAttribute("resultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("resultException", rs.Exception().String())
		span.SetAttribute("resultGasUsed", rs.GasUsed())
	}

	return rs, stateChanged
}

// checkStateDBStatus checks MVM result maps to see if state *would* have changed.
// Currently does not perform tracing itself.
func (vmP *VmProcessor) checkStateDBStatus(
	ctx context.Context, // Context is available if tracing needed later
	transaction types.Transaction,
	mvmRs *mvm.MVMExecuteResult,
) bool {
	if mvmRs.Status == pb.RECEIPT_STATUS_THREW || mvmRs.Status == pb.RECEIPT_STATUS_HALTED {
		return false
	}
	if len(mvmRs.MapAddBalance) > 0 {
		return true
	}
	if len(mvmRs.MapSubBalance) > 0 {
		return true
	}
	if len(mvmRs.JEventLogs.Logs) > 0 {
		return true
	}
	if len(mvmRs.MapCodeHash) > 0 {
		return true
	}
	if len(mvmRs.MapCodeChange) > 0 {
		return true
	}
	if len(mvmRs.MapStorageChange) > 0 {
		return true
	}
	if len(mvmRs.MapNonce) > 0 {
		return true
	}
	if len(mvmRs.MapFullDbHash) > 0 {
		return true
	}
	return false
}

func (vmP *VmProcessor) ExecuteTransactionWithMvmIdSubDeploy(
	ctx context.Context,
	tx types.Transaction, mvmId common.Address, extendedMode bool,
) (types.ExecuteSCResult, bool, error) {
	var span *trace.Span = nil             // Khởi tạo nil
	var subDeployCtx context.Context = ctx // Mặc định dùng context vào

	logger.Warn("ExecuteTransactionWithMvmIdSubDeploy using potentially shared MVM ID", "mvmId", mvmId.Hex())

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		subDeployCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.ExecuteTransactionWithMvmIdSubDeploy", map[string]interface{}{
			"txHash":       tx.Hash().Hex(),
			"from":         tx.FromAddress().Hex(),
			"value":        tx.Amount().String(),
			"extendedMode": extendedMode,
			"mvmIdUsed":    mvmId.Hex(),
			"codeSize":     len(tx.DeployData().Code()),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	mvmSubDeploy := mvm.GetOrCreateMVMApi(mvmId, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), extendedMode)
	mvmSubDeploy.SetRelatedAddresses(tx.RelatedAddresses())

	rs, status := vmP.onlyDeploy(subDeployCtx, tx, mvmSubDeploy) // Truyền subDeployCtx

	if span != nil { // GUARD
		span.AddEvent("SubDeployResult", map[string]interface{}{
			"resultStatus": rs.ReceiptStatus().String(),
			"resultGas":    rs.GasUsed(),
			"stateChanged": status,
		})
	}

	return rs, status, nil
}

func (vmP *VmProcessor) onlyDeploy(
	ctx context.Context,
	tx types.Transaction, mvmE *mvm.MVMApi,
) (types.ExecuteSCResult, bool) {
	var span *trace.Span = nil // Khởi tạo nil
	// var onlyDeployCtx context.Context = ctx // Không cần nếu không truyền xuống
	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	if vmP.tracingEnabled { // Chỉ tạo span nếu flag processor bật
		var actualSpan *trace.Span
		_, actualSpan = trace.StartSpan(ctx, "VmProcessor.onlyDeploy", map[string]interface{}{
			"mvmId":       mvmE.GetKey().Hex(),
			"from":        tx.FromAddress().Hex(),
			"value":       tx.Amount().String(),
			"nonce":       tx.GetNonce(),
			"codeSize":    len(tx.DeployData().Code()),
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
		span = actualSpan
		defer func() {
			if span != nil {
				span.End()
			}
		}() // Defer có điều kiện
	}

	if span != nil { // GUARD
		span.AddEvent("CallingMvmDeploySub", map[string]interface{}{
			"commit":  false,
			"isDebug": tx.GetIsDebug(),
		})
	}

	_, isFree := vmP.chainState.GetFreeFeeAddress()[tx.ToAddress()]
	maxGas := tx.MaxGas()
	if isFree {
		maxGas = uint64(mt_common.MAX_GASS_FEE)
	}
	mvmResult := mvmE.Deploy( // Luôn gọi MVM
		tx.FromAddress().Bytes(), tx.DeployData().Code(), tx.Amount(), tx.MaxGasPrice(), maxGas,
		lastBlockHeader.TimeStamp(), mt_common.BLOCK_GAS_LIMIT, vmP.blockTime, mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1, lastBlockHeader.LeaderAddress(), mvmE.GetKey(), tx.Hash().Bytes(), tx.GetIsDebug(), false,
	)

	if span != nil { // GUARD
		span.AddEvent("MvmDeploySubFinished", map[string]interface{}{
			"status":           mvmResult.Status.String(),
			"exception":        mvmResult.Exception.String(),
			"gasUsed":          mvmResult.GasUsed,
			"returnLen":        len(mvmResult.Return),
			"returnHex":        hex.EncodeToString(mvmResult.Return),
			"numLogs":          len(mvmResult.JEventLogs.Logs),
			"potBalanceChange": len(mvmResult.MapAddBalance)+len(mvmResult.MapSubBalance) > 0,
			"potNonceChange":   len(mvmResult.MapNonce) > 0,
			"potCodeChange":    len(mvmResult.MapCodeChange) > 0,
			"potStorageChange": len(mvmResult.MapStorageChange) > 0,
			"potFullDbChange":  len(mvmResult.MapFullDbHash) > 0,
		})
	}

	// checkStateDBStatus không trace, dùng ctx gốc
	stateChanged := vmP.checkStateDBStatus(ctx, tx, mvmResult)
	if span != nil { // GUARD
		span.SetAttribute("potentialStateChangeReported", stateChanged)
	}

	rs := smart_contract.NewExecuteSCResult(tx.Hash(), mvmResult.Status, mvmResult.Exception, mvmResult.Return, mvmResult.GasUsed, common.Hash{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if span != nil { // GUARD
		span.SetAttribute("resultStatus", rs.ReceiptStatus().String())
		span.SetAttribute("resultException", rs.Exception().String())
		span.SetAttribute("resultGasUsed", rs.GasUsed())
	}

	return rs, stateChanged
}

// executeNonceOnly xử lý một giao dịch chỉ để tăng nonce mà không thực thi EVM.
// Điều này hữu ích cho các giao dịch hệ thống cần cập nhật nonce một cách rõ ràng.
func (vmP *VmProcessor) ExecuteNonceOnly(
	ctx context.Context,
	tx types.Transaction,
	isCache bool,
) (types.ExecuteSCResult, error) {
	var execCtx context.Context = ctx
	var span *trace.Span = nil

	// Bắt đầu trace span nếu tracing được bật
	if vmP.tracingEnabled {
		var actualSpan *trace.Span
		execCtx, actualSpan = trace.StartSpan(ctx, "VmProcessor.executeNonceOnly", map[string]interface{}{
			"txHash":   tx.Hash().Hex(),
			"from":     tx.FromAddress().Hex(),
			"gasLimit": tx.MaxGas(),
			"gasPrice": tx.MaxGasPrice(),
			"nonce":    tx.GetNonce(),
			"isCache":  isCache,
			"mvmId":    vmP.mvmId.Hex(),
		})
		span = actualSpan
		defer span.End() // Kết thúc span khi hàm này thoát
	}

	if span != nil {
		span.AddEvent("HandlingNonceOnlyTransaction", nil)
	}

	lastBlockHeader := *vmP.chainState.GetcurrentBlockHeader()

	// Lấy hoặc tạo MVM API instance
	mvmE := mvm.GetOrCreateMVMApi(vmP.mvmId, vmP.chainState.GetSmartContractDB(), vmP.chainState.GetAccountStateDB(), false)
	mvmE.SetRelatedAddresses(tx.RelatedAddresses()) // Đặt các địa chỉ liên quan cho tính nhất quán

	if span != nil {
		span.AddEvent("CallingMvmNoncePlusOne", map[string]interface{}{
			"blockNumber": lastBlockHeader.BlockNumber() + 1,
			"blockTs":     vmP.blockTime,
			"leader":      lastBlockHeader.LeaderAddress().Hex(),
		})
	}

	// Kiểm tra xem địa chỉ gửi có được miễn phí gas không
	_, isFreeSender := vmP.chainState.GetFreeFeeAddress()[tx.FromAddress()]

	// Gọi hàm NoncePlusOne từ MVM
	mvmResult := mvmE.NoncePlusOne(
		tx.FromAddress().Bytes(),
		tx.MaxGasPrice(),
		tx.MaxGas(),
		lastBlockHeader.TimeStamp(), // Sử dụng blockPrevrandao nhất quán với các cuộc gọi MVM khác
		mt_common.BLOCK_GAS_LIMIT,
		vmP.blockTime,
		mt_common.MINIMUM_BASE_FEE,
		lastBlockHeader.BlockNumber()+1,
		lastBlockHeader.LeaderAddress(),
		vmP.mvmId,
	)

	if span != nil {
		span.AddEvent("MvmNoncePlusOneFinished", map[string]interface{}{
			"status":      mvmResult.Status.String(),
			"exception":   mvmResult.Exception.String(),
			"gasUsed":     mvmResult.GasUsed,
			"nonceChange": len(mvmResult.MapNonce) > 0,
		})
		span.AddEvent("UpdatingStateDBAfterNoncePlusOne", nil)
	}

	// Cập nhật trạng thái DB dựa trên kết quả từ MVM
	_, err := vmP.updateStateDB(execCtx, tx, mvmResult, vmP.mvmId, isFreeSender)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to update state DB after NoncePlusOne: %w", err)
		if span != nil {
			span.SetError(wrappedErr)
			span.AddEvent("StateDBUpdateAfterNoncePlusOneFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		// Vẫn chuyển đổi kết quả MVM để trả về một ExecuteSCResult có ý nghĩa
		rs, _ := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult)
		return rs, wrappedErr
	}
	if span != nil {
		span.AddEvent("StateDBUpdateAfterNoncePlusOneFinished", nil)
	}

	// Chuyển đổi kết quả MVM sang ExecuteSCResult
	rs, errConvert := vmP.MvmResultToExecuteResult(execCtx, tx, mvmResult)
	if errConvert != nil {
		wrappedErr := fmt.Errorf("failed to convert MVM result to execute result after NoncePlusOne: %w", errConvert)
		if span != nil {
			span.SetError(wrappedErr)
			span.AddEvent("MvmResultToExecuteResultConversionFailed", map[string]interface{}{"error": wrappedErr.Error()})
		}
		// Vẫn trả về rs đã chuyển đổi, ngay cả khi có lỗi chuyển đổi
	}

	// Xóa MVM API instance nếu không ở chế độ cache, vì đây là một hoạt động thay đổi trạng thái
	if !isCache {
		mvm.ClearMVMApi(vmP.mvmId)
	}
	return rs, nil
}

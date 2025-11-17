#include <iostream>
#include <tuple>
#include "mvm_linker.hpp"
#include <string>
#include "mvm/opcode.h"
#include "mvm/processor.h"
#include "my_global_state.h"

#include "my_logger.h"
#include "my_extension/my_extension.h"
#include "xapian/xapian_manager.h"

#include "xapian/xapian_registry.h"
#include "my_extension/utils.h"

#include <stdlib.h>
#include <cassert>
#include <fmt/format_header_only.h>
#include <fstream>
#include <iostream>
#include <nlohmann/json.hpp>
#include <random>
#include <sstream>
#include <vector>
#include <fstream>
#include <mutex>
#include <unordered_map>
#include <chrono>
#include "state.h"
#include <stdbool.h> // Thêm include này

ExecuteResult *pendingResult;

nlohmann::json vectorLogsHandlerToJson(mvm::VectorLogHandler logHandler)
{
    auto json_logs = nlohmann::json::array();
    for (const auto &log : logHandler.logs)
    {
        nlohmann::json json_log;
        mvm::to_json(json_log, log);
        json_logs.push_back(json_log);
    }

    return json_logs;
}

// Hàm helper để xóa mảng con trỏ, mỗi phần tử cũng là mảng
template <typename T>
void safeDeleteArrayOfArrays(T **&ptrArray, int count)
{
    if (ptrArray)
    {
        for (int i = 0; i < count; ++i)
        {
            if (ptrArray[i])
            {
                delete[] ptrArray[i];
                ptrArray[i] = nullptr; 
            }
        }
        delete[] ptrArray;
        ptrArray = nullptr; 
    }
}


void cleanupProcessResultMemoryOnError(
    char *&b_output,
    char *&b_exmsg,
    char *&b_logs,
    uint8_t **&b_add_balance_change, int length_add_balance_change,
    uint8_t **&b_sub_balance_change, int length_sub_balance_change,
    uint8_t **&b_nonce_change, int length_nonce_change,
    uint8_t **&b_code_change, int length_code_change, int *&length_codes,
    uint8_t **&b_storage_change, int length_storage_change, int *&length_storages,
    char **&b_full_db_hash, int length_full_db_hash, int *&length_full_db_hashes)
{
    std::cerr << "[CLEANUP] Cleaning up allocated memory due to error..." << std::endl;
    delete[] b_output;
    b_output = nullptr;
    delete[] b_exmsg;
    b_exmsg = nullptr;
    delete[] b_logs;
    b_logs = nullptr;

    safeDeleteArrayOfArrays(b_add_balance_change, length_add_balance_change);
    safeDeleteArrayOfArrays(b_sub_balance_change, length_sub_balance_change);
    safeDeleteArrayOfArrays(b_nonce_change, length_nonce_change);

    safeDeleteArrayOfArrays(b_code_change, length_code_change);
    delete[] length_codes;
    length_codes = nullptr;

    safeDeleteArrayOfArrays(b_storage_change, length_storage_change);
    delete[] length_storages;
    length_storages = nullptr;

    safeDeleteArrayOfArrays(b_full_db_hash, length_full_db_hash);
    delete[] length_full_db_hashes;
    length_full_db_hashes = nullptr;

    std::cerr << "[CLEANUP] Cleanup finished." << std::endl;
}

void append_argument(std::vector<uint8_t> &code, const uint256_t &arg)
{
    const auto pre_size = code.size();
    code.resize(pre_size + 32u);
    mvm::to_big_endian(arg, code.data() + pre_size);
}
// Run input as an EVM transaction, check the result and return the output
mvm::ExecResult run(
    mvm::MyGlobalState &gs,
    bool deploy,
    const mvm::Address &from,
    const mvm::Address &to,
    const uint256_t &amount,
    uint64_t gas_price,
    uint64_t gas_limit,
    mvm::VectorLogHandler &log_handler,
    const mvm::Code &input,
    unsigned char *mvmId,
    bool readOnly,
    const uint256_t &tx_hash,
    bool is_debug)
{
    mvm::Transaction tx(
        from,
        amount,
        gas_price,
        gas_limit,
        tx_hash,
        is_debug);
    MyLogger logger = MyLogger();
    MyExtension extension = MyExtension(mvmId);
    mvm::Processor p(gs, log_handler, extension, logger);


    try
    {

        auto fromAc = gs.get(from, nullptr);
        auto toAc = gs.get(to, nullptr);
        if (amount > 0)
        {
            fromAc.acc.set_balance(fromAc.acc.get_balance() - amount);
            toAc.acc.set_balance(toAc.acc.get_balance() + amount);
            gs.add_addresses_sub_balance_change(from, amount);
            gs.add_addresses_add_balance_change(to, amount);
        }

        const auto exec_result = p.run(tx, deploy, from, gs.get(to, nullptr), input, amount, nullptr, readOnly);

        fromAc.acc.increment_nonce();
        auto new_nonce = fromAc.acc.get_nonce();
        gs.set_addresses_nonce_change(from, new_nonce);

        return exec_result;
    }
    catch (const std::exception &e)
    {
        mvm::ExecResult error_result;
        auto fromAc = gs.get(from, nullptr);

        error_result.er = mvm::ExitReason::threw; 
        
        error_result.ex = mvm::Exception::Type::addressNotInRelated;
        error_result.exmsg = e.what(); 
        fromAc.acc.increment_nonce();
        auto new_nonce = fromAc.acc.get_nonce();
        gs.set_addresses_nonce_change(from, new_nonce);
        return error_result;
    }
}

mvm::BlockContext CreateBlockContext(
    unsigned char *mvmId,
    uint64_t prevrandao,
    uint64_t gas_limit,
    uint64_t time,
    uint64_t base_fee,
    uint256_t number,
    uint256_t coinbase)
{
    mvm::BlockContext block_context;
    block_context.mvmId = mvmId;
    block_context.prevrandao = prevrandao;
    block_context.gas_limit = gas_limit;
    block_context.time = time;
    block_context.base_fee = base_fee;
    block_context.number = number;
    block_context.coinbase = coinbase;
    return block_context;
}

// --- Hàm processResult ---
ExecuteResult *processResult(mvm::ExecResult result, mvm::MyGlobalState &gs, mvm::VectorLogHandler &log_handler)
{
    // --- Khởi tạo tất cả các con trỏ là nullptr ban đầu ---
    char *b_output = nullptr;
    int length_output = 0;
    char *b_exmsg = nullptr;
    int length_exmsg = 0;
    char *b_logs = nullptr;
    int length_logs = 0;
    uint8_t **b_add_balance_change = nullptr;
    int length_add_balance_change = 0;
    uint8_t **b_sub_balance_change = nullptr;
    int length_sub_balance_change = 0;
    uint8_t **b_nonce_change = nullptr;
    int length_nonce_change = 0;
    uint8_t **b_code_change = nullptr;
    int length_code_change = 0;
    int *length_codes = nullptr;
    uint8_t **b_storage_change = nullptr;
    int length_storage_change = 0;
    int *length_storages = nullptr;
    char **b_full_db_hash = nullptr;
    int length_full_db_hash = 0;
    int *length_full_db_hashes = nullptr;
    char **b_full_db_logs = nullptr;
    int length_full_db_logs = 0;
    int *length_full_db_logs_data = nullptr;

    ExecuteResult *pendingResult = nullptr; // Khởi tạo con trỏ kết quả là nullptr

    try
    { 
        // --- Xử lý Output ---
        if (!result.output.empty())
        {
            length_output = static_cast<int>(result.output.size());
            b_output = new char[length_output];
            memcpy(b_output, result.output.data(), length_output);
        }

        // --- Xử lý Exception Message ---
        if (!result.exmsg.empty())
        {
            length_exmsg = static_cast<int>(result.exmsg.size());
            b_exmsg = new char[length_exmsg + 1];
            memcpy(b_exmsg, result.exmsg.c_str(), length_exmsg);
            b_exmsg[length_exmsg] = '\0';
        }

        // --- Xử lý Logs ---
        try
        { 
            auto json_logs = vectorLogsHandlerToJson(log_handler);
            std::string str_logs = json_logs.dump();
            if (!str_logs.empty() && str_logs != "[]")
            {
                length_logs = static_cast<int>(str_logs.size());
                b_logs = new char[length_logs + 1];
                memcpy(b_logs, str_logs.c_str(), length_logs);
                b_logs[length_logs] = '\0';
            }
        }
        catch (const nlohmann::json::exception &e)
        {
             std::cerr << "[ERROR] JSON dumping failed for logs: " << e.what() << std::endl;
        }

        // Add balance
        std::vector<std::vector<uint8_t>> add_balance_change = gs.get_add_balance_change();
        length_add_balance_change = add_balance_change.size();
        if (length_add_balance_change > 0)
        {
            b_add_balance_change = new uint8_t *[length_add_balance_change](); // Initialize with nullptrs
            for (int i = 0; i < length_add_balance_change; ++i)
            {
                size_t current_size = add_balance_change[i].size();
                if (current_size == 0)
                    continue;
                b_add_balance_change[i] = new uint8_t[current_size];
                memcpy(b_add_balance_change[i], add_balance_change[i].data(), current_size);
            }
        }

        // Sub balance
        std::vector<std::vector<uint8_t>> sub_balance_change = gs.get_sub_balance_change();
        length_sub_balance_change = sub_balance_change.size();
        if (length_sub_balance_change > 0)
        {
            b_sub_balance_change = new uint8_t *[length_sub_balance_change]();
            for (int i = 0; i < length_sub_balance_change; ++i)
            {
                size_t current_size = sub_balance_change[i].size();
                if (current_size == 0)
                    continue;
                b_sub_balance_change[i] = new uint8_t[current_size];
                memcpy(b_sub_balance_change[i], sub_balance_change[i].data(), current_size);
            }
        }

        // Nonce changes
        std::vector<std::vector<uint8_t>> nonce_change = gs.get_nonce_change();
        length_nonce_change = nonce_change.size();
        if (length_nonce_change > 0)
        {
            b_nonce_change = new uint8_t *[length_nonce_change]();
            for (int i = 0; i < length_nonce_change; ++i)
            {
                size_t current_size = nonce_change[i].size();
                if (current_size == 0)
                    continue;
                b_nonce_change[i] = new uint8_t[current_size];
                memcpy(b_nonce_change[i], nonce_change[i].data(), current_size);
            }
        }

        // Code changes
        std::vector<std::vector<uint8_t>> code_change = gs.get_newly_deploy();
        length_code_change = code_change.size();
        if (length_code_change > 0)
        {
            length_codes = new int[length_code_change];
            b_code_change = new uint8_t *[length_code_change]();
            for (size_t i = 0; i < length_code_change; ++i)
            {
                length_codes[i] = code_change[i].size();
                if (length_codes[i] == 0)
                {
                    b_code_change[i] = nullptr;
                    continue;
                }
                b_code_change[i] = new uint8_t[length_codes[i]];
                memcpy(b_code_change[i], code_change[i].data(), length_codes[i]);
            }
        }

        // Storage changes
        std::vector<std::vector<uint8_t>> storage_change = gs.get_storage_change();
        length_storage_change = storage_change.size();
        if (length_storage_change > 0)
        {
            length_storages = new int[length_storage_change];
            b_storage_change = new uint8_t *[length_storage_change]();
            for (int i = 0; i < length_storage_change; ++i)
            {
                length_storages[i] = storage_change[i].size();
                if (length_storages[i] == 0)
                {
                    b_storage_change[i] = nullptr;
                    continue;
                }
                b_storage_change[i] = new uint8_t[length_storages[i]];
                memcpy(b_storage_change[i], storage_change[i].data(), length_storages[i]);
            }
        }


        if (result.er == mvm::ExitReason::returned)
        {
            unsigned char *mvmId_to_query = gs.get_block_context().mvmId;
            registry.commitTransaction(mvmId_to_query);
            std::map<mvm::Address, XapianLog::ComprehensiveLog> groupLogs = registry.getGroupChangeLogsForMvmId(mvmId_to_query);
            length_full_db_logs = groupLogs.size();
            if (length_full_db_logs > 0)
            {
                b_full_db_logs = new char *[length_full_db_logs]();
                length_full_db_logs_data = new int[length_full_db_logs];
                int i = 0;
                for (const auto &pair : groupLogs)
                {
                    const mvm::Address &addr = pair.first;
                    const XapianLog::ComprehensiveLog &log = pair.second;
                    std::vector<uint8_t> serialized_log_data;
                    try
                    {
                        serialized_log_data = log.serialize();
                    }
                    catch (const std::exception &e)
                    {
                        b_full_db_logs[i] = nullptr; 
                        length_full_db_logs_data[i] = 0;
                        i++;
                        continue; 
                    }

                    size_t serialized_size = serialized_log_data.size();
                    int element_size = 32 + serialized_size; 

                    b_full_db_logs[i] = new char[element_size];
                    length_full_db_logs_data[i] = element_size; 

                    mvm::to_big_endian(addr, reinterpret_cast<uint8_t *>(b_full_db_logs[i]));
                    memcpy(b_full_db_logs[i] + 32, serialized_log_data.data(), serialized_size);

                    i++; 
                }
            }

            std::map<mvm::Address, std::array<uint8_t, 32u>> groupHashes = registry.getGroupHashForMvmId(mvmId_to_query);

            length_full_db_hash = groupHashes.size();

            if (length_full_db_hash > 0)
            {
                b_full_db_hash = new char *[length_full_db_hash]();
                length_full_db_hashes = new int[length_full_db_hash];

                int i = 0;
                for (const auto &pair : groupHashes)
                {
                    const mvm::Address &addr = pair.first;
                    const std::array<uint8_t, 32u> &hash_array = pair.second;

                    const int element_size = 64;
                    b_full_db_hash[i] = new char[element_size];
                    length_full_db_hashes[i] = element_size;

                    mvm::to_big_endian(addr, reinterpret_cast<uint8_t *>(b_full_db_hash[i]));
                    memcpy(b_full_db_hash[i] + 32, hash_array.data(), 32);
                    i++; 
                }
            }
        }
        else
        {
            unsigned char *mvmId_to_query = gs.get_block_context().mvmId;
            std::map<mvm::Address, XapianLog::ComprehensiveLog> groupLogs = registry.getGroupChangeLogsForMvmId(mvmId_to_query);
            registry.cancelTransaction(mvmId_to_query);
        }

        pendingResult = new ExecuteResult{
            b_exitReason : (char)result.er,
            b_exception : (char)result.ex,
            b_exmsg : b_exmsg,
            length_exmsg : length_exmsg,
            b_output : b_output,
            length_output : length_output,
            full_db_hash : b_full_db_hash,
            length_full_db_hash : length_full_db_hash,
            length_full_db_hashes : length_full_db_hashes,
            full_db_logs : b_full_db_logs,
            length_full_db_logs : length_full_db_logs,
            length_full_db_logs_data : length_full_db_logs_data,
            b_add_balance_change : (char **)b_add_balance_change,
            length_add_balance_change : length_add_balance_change,
            b_nonce_change : (char **)b_nonce_change,
            length_nonce_change : length_nonce_change,
            b_sub_balance_change : (char **)b_sub_balance_change,
            length_sub_balance_change : length_sub_balance_change,
            b_code_change : (char **)b_code_change,
            length_code_change : length_code_change,
            length_codes : length_codes,
            b_storage_change : (char **)b_storage_change,
            length_storage_change : length_storage_change,
            length_storages : length_storages,
            b_logs : b_logs,
            length_logs : length_logs,
            gas_used : result.gas_used
        };

        return pendingResult; 
    }
    catch (const std::bad_alloc &e)
    {
        std::cerr << "[FATAL] Allocation failed during processResult: " << e.what() << std::endl;
        cleanupProcessResultMemoryOnError(
            b_output, b_exmsg, b_logs,
            b_add_balance_change, length_add_balance_change,
            b_sub_balance_change, length_sub_balance_change,
            b_nonce_change, length_nonce_change,
            b_code_change, length_code_change, length_codes,
            b_storage_change, length_storage_change, length_storages,
            b_full_db_hash, length_full_db_hash, length_full_db_hashes);
        throw; 
    }
    catch (const std::exception &e)
    {
        std::cerr << "[ERROR] Standard exception during processResult: " << e.what() << std::endl;
        cleanupProcessResultMemoryOnError(
            b_output, b_exmsg, b_logs,
            b_add_balance_change, length_add_balance_change,
            b_sub_balance_change, length_sub_balance_change,
            b_nonce_change, length_nonce_change,
            b_code_change, length_code_change, length_codes,
            b_storage_change, length_storage_change, length_storages,
            b_full_db_hash, length_full_db_hash, length_full_db_hashes);
        throw; 
    }
    catch (...)
    {
        std::cerr << "[ERROR] Unknown exception during processResult." << std::endl;
        cleanupProcessResultMemoryOnError(
            b_output, b_exmsg, b_logs,
            b_add_balance_change, length_add_balance_change,
            b_sub_balance_change, length_sub_balance_change,
            b_nonce_change, length_nonce_change,
            b_code_change, length_code_change, length_codes,
            b_storage_change, length_storage_change, length_storages,
            b_full_db_hash, length_full_db_hash, length_full_db_hashes);
        throw;
    }
}

ExecuteResult *deploy(
    // transaction data
    unsigned char *b_caller_address,
    unsigned char *b_contract_constructor,
    int contract_constructor_length,
    unsigned char *b_amount,
    unsigned long long gas_price,
    unsigned long long gas_limit,
    // block context data
    unsigned long long block_prevrandao,
    unsigned long long block_gas_limit,
    unsigned long long block_time,
    unsigned long long block_base_fee,
    unsigned char *b_block_number,
    unsigned char *b_block_coinbase,
    unsigned char *mvmId,
    unsigned char *b_tx_hash,
    bool is_debug,
    bool is_cache)
{
    try
    {
        // format argument to right data type
        uint256_t caller_address = mvm::from_big_endian((uint8_t *)b_caller_address, 20u);

        std::vector<uint8_t> contract_constructor((uint8_t *)b_contract_constructor, (uint8_t *)b_contract_constructor + contract_constructor_length);
        uint256_t amount = mvm::from_big_endian((uint8_t *)b_amount, 32u);

        uint256_t block_number = mvm::from_big_endian((uint8_t *)b_block_number, 32u);
        uint256_t block_coinbase = mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u);

        uint256_t tx_hash = mvm::from_big_endian((uint8_t *)b_tx_hash, 32u);

        mvm::BlockContext blockContext = CreateBlockContext(
            mvmId,
            block_prevrandao,
            block_gas_limit,
            block_time,
            block_base_fee,
            block_number,
            block_coinbase);
        mvm::MyGlobalState gs(blockContext, is_cache);
        //  init env
        mvm::VectorLogHandler log_handler;
        auto from = gs.get(caller_address, nullptr);
        const auto contract_address = mvm::generate_address(caller_address, from.acc.get_nonce());
        std::string caller_hex = mvm::to_hex_string(caller_address);
        std::string contract_address_hex = mvm::to_hex_string(contract_address);
        // Set this constructor as the contract's code body
        auto contract = gs.create(contract_address, 0u, contract_constructor, 0);

        auto result = run(
            gs,
            true,
            caller_address,
            contract_address,
            amount,
            gas_price,
            gas_limit,
            log_handler,
            {}, mvmId, false,
            tx_hash, is_debug);
        auto code = result.output;
        contract.acc.set_code(std::move(code));

        gs.add_addresses_newly_deploy(contract_address, code);
        // update output to contract address
        std::vector<uint8_t> deployed_address(32);
        mvm::to_big_endian(contract_address, deployed_address.data());
        std::vector<uint8_t> truncated_address(20);
        memcpy(truncated_address.data(), deployed_address.data() + 12, 20);

        result.output = truncated_address;
        ExecuteResult *rs = processResult(result, gs, log_handler);

        return rs;
    }
    catch (const mvm::Exception &e)
    {
        // Xử lý exception từ MVM
        mvm::ExecResult error_result;
        error_result.er = mvm::ExitReason::threw;
        error_result.ex = e.type;
        error_result.exmsg = e.what();
        
        // Tạo empty state và log handler để processResult
        mvm::BlockContext blockContext = CreateBlockContext(
            mvmId,
            block_prevrandao,
            block_gas_limit,
            block_time,
            block_base_fee,
            mvm::from_big_endian((uint8_t *)b_block_number, 32u),
            mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u));
        mvm::MyGlobalState gs(blockContext, is_cache);
        mvm::VectorLogHandler log_handler;
        
        ExecuteResult *rs = processResult(error_result, gs, log_handler);
        return rs;
    }
    catch (const std::exception &e)
    {
        // Xử lý các exception khác
        mvm::ExecResult error_result;
        error_result.er = mvm::ExitReason::threw;
        error_result.ex = mvm::Exception::Type::addressNotInRelated;
        error_result.exmsg = e.what();
        
        // Tạo empty state và log handler để processResult
        mvm::BlockContext blockContext = CreateBlockContext(
            mvmId,
            block_prevrandao,
            block_gas_limit,
            block_time,
            block_base_fee,
            mvm::from_big_endian((uint8_t *)b_block_number, 32u),
            mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u));
        mvm::MyGlobalState gs(blockContext, is_cache);
        mvm::VectorLogHandler log_handler;
        
        ExecuteResult *rs = processResult(error_result, gs, log_handler);
        return rs;
    }
    catch (...)
    {
        // Xử lý các exception không xác định
        mvm::ExecResult error_result;
        error_result.er = mvm::ExitReason::threw;
        error_result.ex = mvm::Exception::Type::addressNotInRelated;
        error_result.exmsg = "Unknown exception occurred during deploy";
        
        // Tạo empty state và log handler để processResult
        mvm::BlockContext blockContext = CreateBlockContext(
            mvmId,
            block_prevrandao,
            block_gas_limit,
            block_time,
            block_base_fee,
            mvm::from_big_endian((uint8_t *)b_block_number, 32u),
            mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u));
        mvm::MyGlobalState gs(blockContext, is_cache);
        mvm::VectorLogHandler log_handler;
        
        ExecuteResult *rs = processResult(error_result, gs, log_handler);
        return rs;
    }
}

ExecuteResult *call(
    // transaction data
    unsigned char *b_caller_address,
    unsigned char *b_contract_address,
    unsigned char *b_input,
    int length_input,
    unsigned char *b_amount,
    unsigned long long gas_price,
    unsigned long long gas_limit,
    // block context data
    unsigned long long block_prevrandao,
    unsigned long long block_gas_limit,
    unsigned long long block_time,
    unsigned long long block_base_fee,
    unsigned char *b_block_number,
    unsigned char *b_block_coinbase,
    unsigned char *mvmId,
    bool readOnly,
    unsigned char *b_tx_hash,
    bool is_debug)
{
    // format argument to right data type
    uint256_t caller_address = mvm::from_big_endian((uint8_t *)b_caller_address, 20u);
    uint256_t contract_address = mvm::from_big_endian((uint8_t *)b_contract_address, 20u);

    std::vector<uint8_t> input((uint8_t *)b_input, (uint8_t *)b_input + length_input);
    uint256_t amount = mvm::from_big_endian((uint8_t *)b_amount, 32u);

    uint256_t block_number = mvm::from_big_endian((uint8_t *)b_block_number, 32u);
    uint256_t block_coinbase = mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u);
    uint256_t tx_hash = mvm::from_big_endian((uint8_t *)b_tx_hash, 32u);

    mvm::BlockContext blockContext = CreateBlockContext(
        mvmId,
        block_prevrandao,
        block_gas_limit,
        block_time,
        block_base_fee,
        block_number,
        block_coinbase);

    mvm::MyGlobalState gs(blockContext, false);
    //  init env
    mvm::VectorLogHandler log_handler;

    auto result = run(
        gs,
        false,
        caller_address,
        contract_address,
        amount,
        gas_price,
        gas_limit,
        log_handler,
        input,
        mvmId, readOnly, tx_hash, is_debug);

    ExecuteResult *rs = processResult(result, gs, log_handler);
    
    return rs;
}

int commit_full_db(unsigned char *mvmId)
{

    bool result = registry.commitChangesForMvmId(mvmId);
    return (int)result;
}

int revert_full_db(unsigned char *mvmId)
{

    bool result = registry.revertChangesForMvmId(mvmId);
    return (int)result;
}

ExecuteResult *execute(
    unsigned char *b_caller_address,
    unsigned char *b_contract_address,
    unsigned char *b_input,
    int length_input,
    unsigned char *b_amount,
    unsigned long long gas_price,
    unsigned long long gas_limit,
    unsigned long long block_prevrandao,
    unsigned long long block_gas_limit,
    unsigned long long block_time,
    unsigned long long block_base_fee,
    unsigned char *b_block_number,
    unsigned char *b_block_coinbase,
    unsigned char *mvmId,
    unsigned char *b_tx_hash,
    bool is_debug)
{

    uint256_t caller_address = mvm::from_big_endian((uint8_t *)b_caller_address, 20u);
    uint256_t contract_address = mvm::from_big_endian((uint8_t *)b_contract_address, 20u);
    std::vector<uint8_t> input((uint8_t *)b_input, (uint8_t *)b_input + length_input);
    uint256_t amount = mvm::from_big_endian((uint8_t *)b_amount, 32u);
    uint256_t block_number = mvm::from_big_endian((uint8_t *)b_block_number, 32u);
    uint256_t block_coinbase = mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u);
    uint256_t tx_hash = mvm::from_big_endian((uint8_t *)b_tx_hash, 32u);

    mvm::BlockContext blockContext = CreateBlockContext(
        mvmId,
        block_prevrandao,
        block_gas_limit,
        block_time,
        block_base_fee,
        block_number,
        block_coinbase);

    mvm::MyGlobalState gs(blockContext, true);

    mvm::VectorLogHandler log_handler;

    auto result = run(
        gs,
        false,
        caller_address,
        contract_address,
        amount,
        gas_price,
        gas_limit,
        log_handler,
        input,
        mvmId,
        false,
        tx_hash, is_debug);

    ExecuteResult *rs = processResult(result, gs, log_handler);

    return rs;
}

ExecuteResult *sendNative(
    unsigned char *b_from,
    unsigned char *b_to,
    unsigned char *b_amount,
    unsigned long long gas_price,
    unsigned long long gas_limit,
    unsigned long long block_prevrandao,
    unsigned long long block_gas_limit,
    unsigned long long block_time,
    unsigned long long block_base_fee,
    unsigned char *b_block_number,
    unsigned char *b_block_coinbase,
    unsigned char *mvmId)
{

    uint256_t from = mvm::from_big_endian((uint8_t *)b_from, 20u);
    uint256_t to = mvm::from_big_endian((uint8_t *)b_to, 20u);
    uint256_t amount = mvm::from_big_endian((uint8_t *)b_amount, 32u);
    uint256_t block_number = mvm::from_big_endian((uint8_t *)b_block_number, 32u);
    uint256_t block_coinbase = mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u);

    mvm::BlockContext blockContext = CreateBlockContext(
        mvmId,
        block_prevrandao,
        block_gas_limit,
        block_time,
        block_base_fee,
        block_number,
        block_coinbase);

    mvm::MyGlobalState gs(blockContext, true);

    auto fromAc = gs.get(from, nullptr);
    auto toAc = gs.get(to, nullptr);

    fromAc.acc.set_balance(fromAc.acc.get_balance() - amount);
    toAc.acc.set_balance(toAc.acc.get_balance() + amount);
    gs.add_addresses_sub_balance_change(from, amount);
    gs.add_addresses_add_balance_change(to, amount);

    fromAc.acc.increment_nonce();
    auto new_nonce = fromAc.acc.get_nonce();
    gs.set_addresses_nonce_change(from, new_nonce);

    mvm::VectorLogHandler log_handler;
    mvm::ExecResult result;
    result.er = mvm::ExitReason::returned;
    result.gas_used = gas_price * gas_limit;

    ExecuteResult *rs = processResult(result, gs, log_handler);

    return rs;
}

ExecuteResult *noncePlusOne(
    unsigned char *b_from,
    unsigned long long gas_price,
    unsigned long long gas_limit,
    unsigned long long block_prevrandao,
    unsigned long long block_gas_limit,
    unsigned long long block_time,
    unsigned long long block_base_fee,
    unsigned char *b_block_number,
    unsigned char *b_block_coinbase,
    unsigned char *mvmId)
{

    uint256_t from = mvm::from_big_endian((uint8_t *)b_from, 20u);
    uint256_t block_number = mvm::from_big_endian((uint8_t *)b_block_number, 32u);
    uint256_t block_coinbase = mvm::from_big_endian((uint8_t *)b_block_coinbase, 20u);

    mvm::BlockContext blockContext = CreateBlockContext(
        mvmId,
        block_prevrandao,
        block_gas_limit,
        block_time,
        block_base_fee,
        block_number,
        block_coinbase);

    mvm::MyGlobalState gs(blockContext, true);


    auto fromAc = gs.get(from, nullptr);

    fromAc.acc.increment_nonce();
    auto new_nonce = fromAc.acc.get_nonce();
    gs.set_addresses_nonce_change(from, new_nonce);

    mvm::VectorLogHandler log_handler;
    mvm::ExecResult result;
    result.er = mvm::ExitReason::returned;
    result.gas_used = 50000;

    ExecuteResult *rs = processResult(result, gs, log_handler);

    return rs;
}


void freeResult(ExecuteResult *pendingResult)
{
    if (!pendingResult)
        return;

    delete[] pendingResult->b_exmsg;
    delete[] pendingResult->b_output;

    if (pendingResult->full_db_hash) {
        for (int i = 0; i < pendingResult->length_full_db_hash; ++i) {
            delete[] pendingResult->full_db_hash[i];
        }
        delete[] pendingResult->full_db_hash;
    }
    delete[] pendingResult->length_full_db_hashes;

    if (pendingResult->full_db_logs) {
        for (int i = 0; i < pendingResult->length_full_db_logs; ++i) {
            delete[] pendingResult->full_db_logs[i];
        }
        delete[] pendingResult->full_db_logs;
    }
    delete[] pendingResult->length_full_db_logs_data;

    if (pendingResult->b_add_balance_change) {
        for (int i = 0; i < pendingResult->length_add_balance_change; ++i) {
            delete[] pendingResult->b_add_balance_change[i];
        }
        delete[] pendingResult->b_add_balance_change;
    }

    if (pendingResult->b_sub_balance_change) {
        for (int i = 0; i < pendingResult->length_sub_balance_change; ++i) {
            delete[] pendingResult->b_sub_balance_change[i];
        }
        delete[] pendingResult->b_sub_balance_change;
    }

    if (pendingResult->b_nonce_change) {
        for (int i = 0; i < pendingResult->length_nonce_change; ++i) {
            delete[] pendingResult->b_nonce_change[i];
        }
        delete[] pendingResult->b_nonce_change;
    }

    if (pendingResult->b_code_change) {
        for (int i = 0; i < pendingResult->length_code_change; ++i) {
            delete[] pendingResult->b_code_change[i];
        }
        delete[] pendingResult->b_code_change;
    }
    delete[] pendingResult->length_codes;

    if (pendingResult->b_storage_change) {
        for (int i = 0; i < pendingResult->length_storage_change; ++i) {
            delete[] pendingResult->b_storage_change[i];
        }
        delete[] pendingResult->b_storage_change;
    }
    delete[] pendingResult->length_storages;

    delete[] pendingResult->b_logs;

    delete pendingResult;
}

ExecuteResult *testMemLeak()
{
    char *b_output = reinterpret_cast<char *>(malloc(32 * sizeof(char)));
    int length_output = static_cast<int>(32);

    int length_add_balance_change = 0;
    int length_sub_balance_change = 0;
    int length_code_change = 0;

    mvm::MyGlobalState gs;
    std::vector<std::vector<uint8_t>> storage_change;

    int length_storage_change = 100;
    int *length_storages = new int[length_storage_change];
    uint8_t **b_storage_change = new uint8_t *[length_storage_change];
    for (int i = 0; i < length_storage_change; ++i)
    {
        int allocSize = 64 * 10 + 32;
        length_storages[i] = allocSize;
        b_storage_change[i] = new uint8_t[allocSize];
        for (int u = 0; u < allocSize; u++)
        {
            b_storage_change[i][u] = i;
        }
    }

    std::string str_logs = "{}";

    ExecuteResult *pendingResult = new ExecuteResult{
        b_exitReason : (char)1,
        b_exception : (char)1,
        b_exmsg : new char[0],
        length_exmsg : 0,

        b_output : b_output,
        length_output : length_output,
        length_add_balance_change : length_add_balance_change,
        length_sub_balance_change : length_sub_balance_change,
        length_code_change : length_code_change,
        b_storage_change : (char **)b_storage_change,
        length_storage_change : length_storage_change,
        length_storages : length_storages,

        b_logs : (char *)malloc((int)str_logs.size() * sizeof(char)),
        length_logs : (int)str_logs.size(),

        gas_used : 0
    };

    memcpy(pendingResult->b_logs, str_logs.c_str(), str_logs.size());

    return pendingResult;
}

void testMemLeakGS(
    int total_address,
    unsigned char *b_contract_addresses)
{
    mvm::MyGlobalState gs;
    for (int i = 0; i < total_address; i++)
    {
        uint256_t contract_address = mvm::from_big_endian(b_contract_addresses + (i * 20), 20u);
        gs.get(contract_address);
    }
}

// --- Triển khai hàm C được export ---
int ReplayFullDbLogs(LogReplayEntryC *entries, int num_entries)
{
    if (entries == nullptr || num_entries <= 0)
    {
        return 0;
    }

    int successful_replays_count = 0;
    bool overall_operation_success = true;

    for (int i = 0; i < num_entries; ++i)
    {
        LogReplayEntryC &current_entry = entries[i];

        if (current_entry.address_ptr == nullptr || current_entry.address_len != 20)
        {
            overall_operation_success = false;
            continue;
        }
        if (current_entry.log_data_ptr == nullptr || current_entry.log_data_len <= 0)
        {
            overall_operation_success = false;
            continue;
        }

        mvm::Address address = mvm::from_big_endian(current_entry.address_ptr, current_entry.address_len);
        std::string addr_hex = mvm::address_to_hex_string(address);

        std::vector<uint8_t> serialized_log_data(
            current_entry.log_data_ptr,
            current_entry.log_data_ptr + current_entry.log_data_len);

        std::optional<XapianLog::ComprehensiveLog> deserialized_log_opt =
            XapianLog::ComprehensiveLog::deserialize(serialized_log_data);

        if (!deserialized_log_opt)
        {
            overall_operation_success = false;
            continue;
        }
        XapianLog::ComprehensiveLog &comp_log = *deserialized_log_opt;
        
        if (comp_log.db_name.empty())
        {
            std::cerr << "[C++ Replay Error] DB db_name in log is empty for address " << addr_hex << ". Cannot determine target manager." << std::endl;
            std::cerr << "  - Entry index: " << i << std::endl;
            overall_operation_success = false;
            continue;
        }

        std::shared_ptr<XapianManager> target_manager = nullptr;
        try
        {
            target_manager = XapianManager::getInstance(comp_log.db_name, address, true);
        }
        catch (const std::exception &e)
        {
            std::cerr << "[C++ Replay Error] Exception when calling XapianManager::getInstance (reset=true) for db_name '" << comp_log.db_name << "' and address " << addr_hex << ": " << e.what() << std::endl;
            std::cerr << "  - Entry index: " << i << std::endl;
            overall_operation_success = false;
            continue;
        }
        catch (...)
        {
            std::cerr << "[C++ Replay Error] Unknown exception when calling XapianManager::getInstance (reset=true) for db_name '" << comp_log.db_name << "' and address " << addr_hex << std::endl;
            std::cerr << "  - Entry index: " << i << std::endl;
            overall_operation_success = false;
            continue;
        }

        if (!target_manager)
        {
            std::cerr << "[C++ Replay Error] Failed to get/create XapianManager for db_name '" << comp_log.db_name << "' and address " << addr_hex << " after reset. Skipping replay for this entry." << std::endl;
            std::cerr << "  - Entry index: " << i << std::endl;
            overall_operation_success = false;
            continue;
        }
        
        bool replay_success_for_this_entry = true;

        if (!comp_log.xapian_doc_logs.empty())
        {
            if (!target_manager->replay_log(comp_log.xapian_doc_logs))
            {
                std::cerr << "[C++ Replay Error] Replaying document logs failed." << std::endl;
                std::cerr << "  - Entry index: " << i << std::endl;
                std::cerr << "  - Address: " << addr_hex << std::endl;
                std::cerr << "  - Total operations attempted: " << comp_log.xapian_doc_logs.size() << std::endl;
                replay_success_for_this_entry = false;
            }
        }
        
        if (replay_success_for_this_entry)
        {
            try
            {
                if (target_manager->saveAllAndCommit())
                {
                    successful_replays_count++;
                }
                else
                {
                    std::cerr << "[C++ Replay Error] saveAllAndCommit failed for address " << addr_hex << std::endl;
                    overall_operation_success = false;
                }
            }
            catch (const std::exception &e)
            {
                std::cerr << "[C++ Replay Error] Exception during saveAllAndCommit for address " << addr_hex << ": " << e.what() << std::endl;
                overall_operation_success = false;
            }
            catch (...)
            {
                std::cerr << "[C++ Replay Error] Unknown exception during saveAllAndCommit for address " << addr_hex << std::endl;
                overall_operation_success = false;
            }
        }
        else
        {
            overall_operation_success = false;
        }

    } 
    return overall_operation_success ? 1 : 0;
}
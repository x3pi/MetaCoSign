// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.
#include <mvm/crypto/sha256.hpp>
#include <mvm/crypto/ripemd160.hpp>
#include <mvm/crypto/bn254.hpp>
#include <mvm/crypto/blake2b.hpp>
#include <mvm/crypto/kzg.hpp>
#include <mvm/crypto/secp256k1.hpp>

#include "my_extension/my_extension.h"
#include "mvm_linker.hpp"
#include <mpfr.h>
#include <math.h>
#include "mvm/util.h"
#include <fstream>
#include <vector>
#include <iostream>
#include "my_extension/constants.h"
#include "xapian/xapian_manager.h"
#include "xapian/xapian_search.h"
#include "xapian/xapian_registry.h"

#include "my_extension/utils.h"
#include <xapian.h>
#include <unordered_map>
#include <random>
#include <ctime>
#include <sstream>
#include <arpa/inet.h> // For ntohl
#include <abi_decode.hpp>
#include <abi_encode.hpp>
#include <charconv>
#include <cstring> // Cần cho memcpy
#include <span>
#include <algorithm> // Thêm thư viện này để dùng transform()
#include <filesystem>

#define DBNAME_MAX_LEN 128 // Giới hạn cho dbname
#define TERM_MAX_LEN 64
extern "C"
{
#include <secp256k1.h>
#include <secp256k1_recovery.h> // Cần thiết cho chức năng phục hồi khóa công khai
}

using namespace evmmax::bn254;

template <typename T>
T getJsonValue(const json &j, const std::string &path1, const std::string &path2, T defaultValue)
{
    try
    {
        return j[path1][path2].get<T>();
    }
    catch (...)
    {
        return defaultValue;
    }
}

void printHex(const std::vector<uint8_t> &bytes)
{
    for (uint8_t byte : bytes)
    {
        std::cout << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(byte);
    }
    std::cout << std::dec << std::endl;
}
mvm::Code convertToCode(Extension_return data)
{
    std::vector<uint8_t> vec(data.data_p, data.data_p + data.data_size);
    return vec;
}

std::vector<uint8_t> hexString32ToBytes(const std::string &hex_input_str)
{
    // 1. Check if the input hex string has the correct length (64 characters)
    //    Optionally handle "0x" prefix if your ABI decode includes it.
    std::string hex_to_convert = hex_input_str;
    if (hex_to_convert.rfind("0x", 0) == 0 || hex_to_convert.rfind("0X", 0) == 0)
    {
        hex_to_convert = hex_to_convert.substr(2); // Remove "0x" prefix
    }

    if (hex_to_convert.length() != 64)
    {
        throw std::invalid_argument("Input hex string must represent 32 bytes (be 64 characters long). Actual length after prefix removal: " + std::to_string(hex_to_convert.length()));
    }

    std::vector<uint8_t> result_bytes;
    result_bytes.reserve(32); // Reserve space for 32 bytes

    // 2. Convert hex pairs to bytes
    for (size_t i = 0; i < hex_to_convert.length(); i += 2)
    {
        std::string byte_string = hex_to_convert.substr(i, 2);
        try
        {
            // Use stoul (string to unsigned long) with base 16
            uint8_t byte = static_cast<uint8_t>(std::stoul(byte_string, nullptr, 16));
            result_bytes.push_back(byte);
        }
        catch (const std::invalid_argument &e)
        {
            throw std::invalid_argument("Invalid hex character found in string: " + byte_string);
        }
        catch (const std::out_of_range &e)
        {
            // This shouldn't happen for 2 hex chars, but good practice
            throw std::out_of_range("Hex value out of range for uint8_t: " + byte_string);
        }
    }

    // 3. Final check (should always be 32 if logic above is correct)
    if (result_bytes.size() != 32)
    {
        // This indicates an internal logic error
        throw std::runtime_error("Internal error: Conversion resulted in unexpected byte count: " + std::to_string(result_bytes.size()));
    }

    return result_bytes;
}
std::vector<uint8_t> ecrecover(
    const std::vector<uint8_t> &message_hash,
    uint8_t v, // Should be 27 or 28 usually
    const std::vector<uint8_t> &r,
    const std::vector<uint8_t> &s)
{
    std::cerr << "ecrecover" << std::endl;

    // 1. Validation
    if (message_hash.size() != 32)
    {
        std::cerr << "Error: Message hash must be 32 bytes long." << std::endl;
        return {}; // Return empty vector on failure
    }
    std::cerr << "ecrecover 1" << std::endl;

    if (r.size() != 32)
    {
        std::cerr << "Error: Signature 'r' component must be 32 bytes long." << std::endl;
        return {};
    }
    std::cerr << "ecrecover 2" << std::endl;

    if (s.size() != 32)
    {
        std::cerr << "Error: Signature 's' component must be 32 bytes long." << std::endl;
        return {};
    }
    // Note: v validation allowing 27/28 happens below when calculating recid

    // 2. Prepare for libsecp256k1
    // Use SECP256K1_CONTEXT_RECOVER for recovery functions

    secp256k1_context *ctx = secp256k1_context_create(SECP256K1_CONTEXT_SIGN | SECP256K1_CONTEXT_VERIFY);
    if (ctx == nullptr)
    {
        std::cerr << "Error: secp256k1_context_create returned nullptr." << std::endl;
        return {};
    }
    std::cerr << "ecrecover 4" << std::endl;

    // Calculate recovery ID (recid). Common Ethereum values for v are 27 and 28.
    // recid is typically 0 or 1 for these. libsecp256k1 expects 0, 1, 2, or 3.
    if (v < 27 || v > 34)
    { // Check broader range used by some systems, adjust if needed
        std::cerr << "Error: v value (" << (int)v << ") out of expected range." << std::endl;
        secp256k1_context_destroy(ctx);
        return {};
    }
    std::cerr << "ecrecover 5" << std::endl;

    int recid = v - 27; // Adjust based on your specific 'v' standard (e.g., some might use 0/1 directly)

    // Combine r and s into a single 64-byte array for parse_compact
    std::vector<uint8_t> input64(64);
    std::copy(r.begin(), r.end(), input64.begin());      // Copy r to first 32 bytes
    std::copy(s.begin(), s.end(), input64.begin() + 32); // Copy s to next 32 bytes

    secp256k1_ecdsa_recoverable_signature recoverable_sig;

    // Parse the compact signature (r, s, recid)
    if (!secp256k1_ecdsa_recoverable_signature_parse_compact(ctx, &recoverable_sig, input64.data(), recid))
    {
        std::cerr << "Error: Failed to parse compact signature (invalid r, s, or recid?). recid used: " << recid << std::endl;
        secp256k1_context_destroy(ctx);
        return {}; // Return empty vector on failure
    }
    std::cerr << "ecrecover 6" << std::endl;

    // 3. Recover the public key
    secp256k1_pubkey pubkey;
    if (!secp256k1_ecdsa_recover(ctx, &pubkey, &recoverable_sig, message_hash.data()))
    {
        std::cerr << "Error: Failed to recover public key from signature." << std::endl;
        secp256k1_context_destroy(ctx);
        return {};
    }

    // 4. Serialize the public key to uncompressed format (65 bytes, starting with 0x04)
    std::vector<uint8_t> pubkey_serialized(65);
    size_t output_len = pubkey_serialized.size();
    if (!secp256k1_ec_pubkey_serialize(ctx, pubkey_serialized.data(), &output_len, &pubkey, SECP256K1_EC_UNCOMPRESSED))
    {
        std::cerr << "Error: Failed to serialize public key." << std::endl;
        secp256k1_context_destroy(ctx);
        return {};
    }

    // Check if serialization produced the expected 65 bytes for uncompressed format
    if (output_len != 65 || pubkey_serialized[0] != 0x04)
    {
        std::cerr << "Error: Unexpected serialized public key format or length. Length: " << output_len << std::endl;
        secp256k1_context_destroy(ctx);
        return {};
    }

    // 5. Calculate the Ethereum address
    // Hash the public key bytes (excluding the 0x04 prefix byte) using Keccak-256
    mvm::KeccakHash pubkey_hash = mvm::keccak_256(pubkey_serialized.data() + 1, 64); // Hash the 64 bytes X and Y coordinates

    // The Ethereum address is the last 20 bytes of the Keccak-256 hash
    std::vector<uint8_t> address_bytes;
    address_bytes.reserve(20);
    // Copy bytes from index 12 to the end (total 32 bytes in hash, 32 - 12 = 20 bytes)
    std::copy(pubkey_hash.begin() + 12, pubkey_hash.end(), std::back_inserter(address_bytes));
    std::cerr << "ecrecover 7" << std::endl;

    // 6. Clean up the secp256k1 context
    secp256k1_context_destroy(ctx);
    std::cerr << "ecrecover 8" << std::endl;

    // 7. Return the 20-byte address
    return address_bytes;
}

std::optional<uint8_t> getFirstByteFromString(const std::string &input_str)
{
    if (input_str.empty())
    {
        std::cerr << "Error: Input string is empty." << std::endl;
        return std::nullopt;
    }
    return reinterpret_cast<const uint8_t *>(input_str.data())[0];
}

mvm::Code MyExtension::CallGetApi(mvm::Code input)
{
    return convertToCode(ExtensionCallGetApi(input.data(), input.size()));
}

mvm::Code MyExtension::ExtractJsonField(mvm::Code input)
{
    return convertToCode(ExtensionExtractJsonField(input.data(), input.size()));
}

mvm::Code MyExtension::Blst(mvm::Code input)
{
    return convertToCode(ExtensionBlst(input.data(), input.size()));
}

mvm::Code MyExtension::Math(mvm::Code input)
{
    // Check for valid input size
    if (input.size() < 4)
    {
        return mvm::Code(32, 0); // Return error for invalid input
    }

    // Get operation code from first 4 bytes
    uint32_t opCode = (input[0] << 24) | (input[1] << 16) | (input[2] << 8) | input[3];
    std::vector<uint8_t> remainingBytes(input.begin() + 4, input.end());

    // Initialize MPFR variables with precision
    mpfr_t result, num1, num2;
    mpfr_init2(result, 256);
    mpfr_init2(num1, 256);
    mpfr_init2(num2, 256);

    // Parse first number if available
    if (remainingBytes.size() >= 32)
    {
        std::vector<uint8_t> firstNumber(remainingBytes.begin(), remainingBytes.begin() + 32);
        mvm::hexToSignedInt(num1, firstNumber);

        // Scale down by SCALE_FACTOR (1e18)
        mpfr_t divisor;
        mpfr_init_set_d(divisor, SCALE_FACTOR, MPFR_RNDN);
        mpfr_div(num1, num1, divisor, MPFR_RNDN);
        mpfr_clear(divisor);
    }

    // Parse second number if available
    if (remainingBytes.size() == 64)
    {
        std::vector<uint8_t> secondNumber(remainingBytes.begin() + 32, remainingBytes.begin() + 64);
        mvm::hexToSignedInt(num2, secondNumber);

        // Scale down by SCALE_FACTOR (1e18)
        mpfr_t divisor;
        mpfr_init_set_d(divisor, SCALE_FACTOR, MPFR_RNDN);
        mpfr_div(num2, num2, divisor, MPFR_RNDN);
        mpfr_clear(divisor);

        // Binary operations (two operands)
        if (opCode == mvm::FunctionSelector::ADD)
        {
            mpfr_add(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SUB)
        {
            mpfr_sub(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::MUL)
        {
            mpfr_mul(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::DIV)
        {
            mpfr_div(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::POW)
        {
            mpfr_pow(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ATAN2)
        {
            mpfr_atan2(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::MOD)
        {
            mpfr_fmod(result, num1, num2, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ROOT)
        {
            mpfr_t inverse;
            mpfr_init2(inverse, 256);
            mpfr_ui_div(inverse, 1, num2, MPFR_RNDN);
            mpfr_pow(result, num1, inverse, MPFR_RNDN);
            mpfr_clear(inverse);
        }
        else if (opCode == mvm::FunctionSelector::GCD || opCode == mvm::FunctionSelector::LCM)
        {
            mpz_t int_x, int_y, res;
            mpz_init(int_x);
            mpz_init(int_y);
            mpz_init(res);

            // Convert floats to integers
            mpfr_get_z(int_x, num1, MPFR_RNDN);
            mpfr_get_z(int_y, num2, MPFR_RNDN);

            // Calculate GCD or LCM
            if (opCode == mvm::FunctionSelector::GCD)
            {
                mpz_gcd(res, int_x, int_y);
            }
            else
            {
                mpz_lcm(res, int_x, int_y);
            }

            // Convert result back to float
            mpfr_set_z(result, res, MPFR_RNDN);

            // Free GMP memory
            mpz_clear(int_x);
            mpz_clear(int_y);
            mpz_clear(res);
        }
        else
        {
            mpfr_clear(result);
            mpfr_clear(num1);
            mpfr_clear(num2);
            return mvm::Code(32, 0); // Invalid operation
        }
    }
    else if (remainingBytes.size() == 32)
    {
        // Unary operations (one operand)
        if (opCode == mvm::FunctionSelector::ABS)
        {
            mpfr_abs(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SIN)
        {
            mpfr_sin(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::COS)
        {
            mpfr_cos(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::TAN)
        {
            mpfr_tan(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ASIN)
        {
            mpfr_asin(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ACOS)
        {
            mpfr_acos(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ATAN)
        {
            mpfr_atan(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SINH)
        {
            mpfr_sinh(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::COSH)
        {
            mpfr_cosh(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::TANH)
        {
            mpfr_tanh(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::EXP)
        {
            mpfr_exp(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::LOG)
        {
            mpfr_log(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::LOG10)
        {
            mpfr_log10(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::LOG2)
        {
            mpfr_log2(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SQRT)
        {
            mpfr_sqrt(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::CEIL)
        {
            mpfr_ceil(result, num1);
        }
        else if (opCode == mvm::FunctionSelector::FLOOR)
        {
            mpfr_floor(result, num1);
        }
        else if (opCode == mvm::FunctionSelector::ROUND)
        {
            mpfr_round(result, num1);
        }
        else if (opCode == mvm::FunctionSelector::COT)
        {
            mpfr_cot(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::CSC)
        {
            mpfr_csc(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SEC)
        {
            mpfr_sec(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::EXP2)
        {
            mpfr_exp2(result, num1, MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::SIGN)
        {
            mpfr_set_si(result, mpfr_sgn(num1), MPFR_RNDN);
        }
        else if (opCode == mvm::FunctionSelector::ENCODE_MPFR)
        {
            std::vector<uint8_t> encodedResult = mvm::evm_encode_mpfr(num1);
            mpfr_clear(result);
            mpfr_clear(num1);
            mpfr_clear(num2);
            return encodedResult;
        }
        else
        {
            mpfr_clear(result);
            mpfr_clear(num1);
            mpfr_clear(num2);
            return mvm::Code(32, 0); // Invalid operation
        }
    }
    else if (remainingBytes.empty())
    {
        // Constants
        if (opCode == mvm::FunctionSelector::PI)
        {
            mpfr_const_pi(result, MPFR_RNDN);
        }
        else
        {
            mpfr_clear(result);
            mpfr_clear(num1);
            mpfr_clear(num2);
            return mvm::Code(32, 0); // Invalid operation
        }
    }
    else
    {
        // Invalid input size
        mpfr_clear(result);
        mpfr_clear(num1);
        mpfr_clear(num2);
        return mvm::Code(32, 0);
    }

    // Scale result back by SCALE_FACTOR
    mpfr_t scaleFactor;
    mpfr_init_set_str(scaleFactor, "1e18", 10, MPFR_RNDN);
    mpfr_mul(result, result, scaleFactor, MPFR_RNDN);
    mpfr_clear(scaleFactor);

    // Convert result to bytes
    std::vector<uint8_t> resultBytes;
    mvm::signedIntToHex(resultBytes, result);

    // Clean up MPFR variables
    mpfr_clear(result);
    mpfr_clear(num1);
    mpfr_clear(num2);

    return resultBytes;
}

// Hàm mã hóa offset thành 32-byte (big-endian)
std::vector<uint8_t> encode_offset(uint32_t offset)
{
    std::vector<uint8_t> encoded(WORD_SIZE, 0);
    encoded[31] = offset & 0xFF; // Chỉ lưu 1 byte cuối
    return encoded;
}

// Hàm mã hóa số nguyên thành 32-byte (big-endian)
std::vector<uint8_t> encode_length(uint32_t length)
{
    std::vector<uint8_t> encoded(WORD_SIZE, 0);
    encoded[31] = length & 0xFF; // Chỉ lưu 1 byte cuối
    return encoded;
}

// Hàm mã hóa chuỗi theo EVM ABI
std::vector<uint8_t> evm_encode_string(const std::string &input)
{
    uint32_t len = input.length();
    uint32_t padded_len = ((len + WORD_SIZE - 1) / WORD_SIZE) * WORD_SIZE; // Căn chỉnh bội số của 32

    std::vector<uint8_t> encoded;

    // Thêm offset (luôn là 32)
    std::vector<uint8_t> offset_bytes = encode_offset(WORD_SIZE);
    encoded.insert(encoded.end(), offset_bytes.begin(), offset_bytes.end());

    // Thêm độ dài chuỗi (32 byte)
    std::vector<uint8_t> length_bytes = encode_length(len);
    encoded.insert(encoded.end(), length_bytes.begin(), length_bytes.end());

    // Thêm nội dung chuỗi (UTF-8)
    encoded.insert(encoded.end(), input.begin(), input.end());

    // Thêm padding 0x00 nếu cần
    encoded.resize(encoded.size() + (padded_len - len), 0);

    return encoded;
}

mvm::Code MyExtension::SimpleDatabase(mvm::Code input, mvm::Address address)
{
    // Check for valid input size
    if (input.size() < 4)
    {
        return mvm::Code(32, 0); // Return error for invalid input
    }

    // Get operation code from first 4 bytes
    uint32_t opCode = (input[0] << 24) | (input[1] << 16) | (input[2] << 8) | input[3];
    // Address là uint256_t
    uint256_t addr = address;

    // Chuyển đổi uint256_t thành mảng byte
    std::vector<uint8_t> addressBytes(20); // 256 bits = 32 bytes

    // Sao chép dữ liệu từ uint256_t vào addressBytes
    for (size_t i = 0; i < 20; ++i)
    {
        addressBytes[19 - i] = static_cast<uint8_t>(addr >> (i * 8));
    }

    Extension_return data = ExtensionGetOrCreateSimpleDb(input.data(), input.size(), addressBytes.data(), this->mvmId);

    if (opCode == mvm::FunctionSelector::GET_OR_CREATE_SIMPLE_DB || opCode == mvm::FunctionSelector::SET || opCode == mvm::FunctionSelector::GET || opCode == mvm::FunctionSelector::GET_ALL || opCode == mvm::FunctionSelector::SEARCH_BY_VALUE || opCode == mvm::FunctionSelector::SINPLE_DB_DELETE || opCode == mvm::FunctionSelector::SINPLE_GET_NEXT_KEYS)
    {
        return convertToCode(data);
    }
    return mvm::Code(32, 0);
}

// Hàm chuyển hex string thành vector<uint8_t>
std::vector<uint8_t> hexToBytes(const std::string &hex)
{
    std::vector<uint8_t> bytes;
    for (size_t i = 0; i < hex.length(); i += 2)
    {
        std::string byteString = hex.substr(i, 2);
        uint8_t byte = (uint8_t)strtol(byteString.c_str(), nullptr, 16);
        bytes.push_back(byte);
    }
    return bytes;
}

std::string uint32ToHexString(uint32_t value)
{
    std::stringstream ss;
    ss << std::hex << std::setfill('0') << std::setw(8) << value; // 8 hex digits for uint32_t
    return ss.str();
}

// Hàm kiểm tra ABI hợp lệ

bool parseABI(const std::vector<uint8_t> &data, std::string &selector_out, std::string &extracted_string)
{
    if (data.size() < 68)
    {
        std::cerr << "ABI Error: Data too short (less than 68 bytes)." << std::endl;
        return false;
    }

    std::stringstream selector_ss;
    for (int i = 0; i < 4; i++)
    {
        selector_ss << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(data[i]);
    }
    selector_out = selector_ss.str();

    uint32_t offset;
    std::memcpy(&offset, &data[32], sizeof(offset));
    offset = ntohl(offset);

    if (offset != 32)
    {
        std::cerr << "ABI Error: Invalid string offset (" << offset << ")." << std::endl;
        return false;
    }

    uint32_t str_length;
    std::memcpy(&str_length, &data[64], sizeof(str_length));
    str_length = ntohl(str_length);

    if (str_length == 0)
    {
        std::cerr << "ABI Error: String length is zero." << std::endl;
        return false;
    }

    if (offset + str_length > data.size())
    {
        std::cerr << "ABI Error: String data exceeds data size." << std::endl;
        return false;
    }

    // Kiểm tra padding (nếu cần)
    // ...

    extracted_string.resize(str_length);
    std::copy(data.begin() + 68, data.begin() + 68 + str_length, extracted_string.begin());

    return true;
}
std::string decimalToHex(int decimal)
{
    std::stringstream ss;
    ss << std::hex << decimal;
    return ss.str();
}

vector<uint8_t> encodeStringArray(const vector<string> &docInfo)
{
    vector<uint8_t> result;

    // 1. Đầu tiên encode độ dài của mảng (offset 0x20)
    result.insert(result.end(), 31, 0x00);
    result.push_back(0x20);

    // 2. Encode số lượng phần tử trong mảng
    uint32_t length = docInfo.size();
    for (int i = 0; i < 28; i++)
    {
        result.push_back(0x00);
    }
    result.push_back((length >> 24) & 0xFF);
    result.push_back((length >> 16) & 0xFF);
    result.push_back((length >> 8) & 0xFF);
    result.push_back(length & 0xFF);

    // 3. Tính và thêm các offset cho từng string
    uint32_t currentOffset = 32 * (docInfo.size() + 1); // offset bắt đầu sau phần header
    for (size_t i = 0; i < docInfo.size(); i++)
    {
        for (int j = 0; j < 28; j++)
        {
            result.push_back(0x00);
        }
        result.push_back((currentOffset >> 24) & 0xFF);
        result.push_back((currentOffset >> 16) & 0xFF);
        result.push_back((currentOffset >> 8) & 0xFF);
        result.push_back(currentOffset & 0xFF);

        currentOffset += 32 + ((docInfo[i].length() + 31) / 32) * 32;
    }

    // 4. Encode từng string
    for (const string &str : docInfo)
    {
        // Encode độ dài của string
        uint32_t strLength = str.length();
        for (int i = 0; i < 28; i++)
        {
            result.push_back(0x00);
        }
        result.push_back((strLength >> 24) & 0xFF);
        result.push_back((strLength >> 16) & 0xFF);
        result.push_back((strLength >> 8) & 0xFF);
        result.push_back(strLength & 0xFF);

        // Encode nội dung string
        result.insert(result.end(), str.begin(), str.end());

        // Padding cho đủ 32 bytes
        size_t padding = (32 - (str.length() % 32)) % 32;
        result.insert(result.end(), padding, 0x00);
    }

    return result;
}
// Hoặc phiên bản ngắn gọn hơn:
void printDocInfo(const std::vector<std::string> &docInfo)
{
    std::cout << "[\n";
    for (const auto &str : docInfo)
    {
        std::cout << "  \"" << str << "\",\n";
    }
    std::cout << "]" << std::endl;
}
// Helper functions
std::vector<uint8_t> getInputWithoutOpcode(const mvm::Code &input)
{
    std::vector<uint8_t> result;
    result.reserve(input.size() - 4);
    for (size_t i = 4; i < input.size(); ++i)
    {
        result.push_back(input[i]);
    }
    return result;
}

// Function to format hex for display
void printHexInput(const mvm::Code &input)
{
    std::cout << "FullDatabase Opcode 2 (hex): 0x";
    for (uint8_t byte : input)
    {
        std::cout << std::hex << std::setw(2) << std::setfill('0') << (int)byte;
    }
    std::cout << std::endl;
}

// Hàm chuyển đổi một string thành std::vector<uint8_t> theo chuẩn ABI
std::vector<uint8_t> toABI(const std::string &str)
{
    // Tạo vector chứa các byte của chuỗi
    std::vector<uint8_t> result(32, 0); // Đảm bảo kích thước là 32 bytes (32 * 8 = 256 bits)

    // Chuyển chuỗi thành vector<uint8_t> và chèn vào đầu vector (đảm bảo dữ liệu bắt đầu từ đầu)
    for (size_t i = 0; i < str.size() && i < 32; ++i)
    {
        result[i] = static_cast<uint8_t>(str[i]);
    }

    return result;
}

// Hàm chuyển đổi 2 string thành vector<uint8_t> thỏa mãn chuẩn ABI
std::vector<uint8_t> convertStringsToABI(const std::string &str1, const std::string &str2)
{
    std::vector<uint8_t> abiData;

    // Chuyển đổi từng chuỗi thành vector<uint8_t> theo chuẩn ABI
    std::vector<uint8_t> abi1 = toABI(str1);
    std::vector<uint8_t> abi2 = toABI(str2);

    // Thêm vào vector kết quả theo chuẩn ABI (mỗi chuỗi 32 bytes)
    abiData.insert(abiData.end(), abi1.begin(), abi1.end());
    abiData.insert(abiData.end(), abi2.begin(), abi2.end());

    return abiData;
}

// Hàm tách chuỗi tại dấu ":"
std::pair<std::string, std::string> splitString(const std::string &input)
{
    size_t pos = input.find(":");

    if (pos != std::string::npos)
    {
        // Trả về một std::pair chứa hai phần chuỗi
        return {input.substr(0, pos), input.substr(pos + 1)};
    }
    else
    {
        // Nếu không tìm thấy ":", trả về hai chuỗi rỗng
        return {"", ""};
    }
}

uint64_t hex_to_uint64(const std::string &hex_str)
{
    uint64_t result = 0;
    std::istringstream(hex_str) >> std::hex >> result;
    return result;
}
std::optional<int64_t> hex_to_int64(const std::string &hex_str)
{
    // Kiểm tra chuỗi rỗng
    if (hex_str.empty())
    {
        return std::nullopt;
    }

    // Con trỏ bắt đầu và kết thúc của chuỗi gốc
    const char *start_parse_ptr = hex_str.data();
    const char *const end_ptr_str = start_parse_ptr + hex_str.length();

    // Xử lý tiền tố "0x" hoặc "0X"
    if (hex_str.length() >= 2 && hex_str[0] == '0' && (hex_str[1] == 'x' || hex_str[1] == 'X'))
    {
        start_parse_ptr += 2; // Di chuyển con trỏ qua tiền tố
    }

    // Kiểm tra nếu không còn gì sau tiền tố (vd: chuỗi là "0x")
    if (start_parse_ptr == end_ptr_str)
    {
        return std::nullopt;
    }

    // Tính số lượng ký tự hex còn lại
    size_t num_hex_digits = end_ptr_str - start_parse_ptr;

    // *** Logic cắt bớt ***
    // Nếu số ký tự nhiều hơn 16, điều chỉnh con trỏ bắt đầu để chỉ lấy 16 ký tự cuối
    if (num_hex_digits > 16)
    {
        start_parse_ptr = end_ptr_str - 16; // Đặt con trỏ vào đầu của 16 ký tự cuối
        num_hex_digits = 16;                // Chỉ xử lý 16 ký tự
    }
    // *** Kết thúc logic cắt bớt ***

    // Kiểm tra nếu sau khi điều chỉnh không còn ký tự nào (trường hợp hy hữu)
    if (num_hex_digits == 0)
    {
        return std::nullopt;
    }

    // Phân tích phần hex (tối đa 16 ký tự) thành uint64_t
    uint64_t unsigned_val = 0;
    const char *end_parse_ptr = start_parse_ptr + num_hex_digits;                    // Con trỏ kết thúc phần cần phân tích
    auto result = std::from_chars(start_parse_ptr, end_parse_ptr, unsigned_val, 16); // Cơ số 16

    // Kiểm tra lỗi từ std::from_chars
    if (result.ec != std::errc())
    {
        // Có lỗi (ký tự không hợp lệ hoặc ngoài phạm vi uint64_t)
        return std::nullopt;
    }

    // Kiểm tra xem std::from_chars có xử lý hết các ký tự dự kiến không
    if (result.ptr != end_parse_ptr)
    {
        // Không xử lý hết (ví dụ: "123G" trong 16 ký tự cuối)
        return std::nullopt;
    }

    // Sử dụng memcpy để diễn giải lại các bit từ unsigned_val sang signed_val
    int64_t signed_val;
    // Đảm bảo kích thước khớp nhau tại thời điểm biên dịch - rất quan trọng cho memcpy
    static_assert(sizeof(signed_val) == sizeof(unsigned_val), "Size mismatch between int64_t and uint64_t");
    std::memcpy(&signed_val, &unsigned_val, sizeof(signed_val));

    // Trả về giá trị đã diễn giải
    return signed_val;
}

// Hàm chuyển đổi chuỗi hex string thành vector<uint8_t>
std::vector<uint8_t> hexStringToByteVector(const std::string &hexString)
{
    std::vector<uint8_t> byteVector;
    // Duyệt qua từng cặp ký tự hex trong chuỗi
    for (size_t i = 0; i < hexString.length(); i += 2)
    {
        // Chuyển cặp ký tự hex thành giá trị uint8_t
        std::string byteStr = hexString.substr(i, 2);
        uint8_t byte = static_cast<uint8_t>(std::stoul(byteStr, nullptr, 16));
        byteVector.push_back(byte);
    }
    return byteVector;
}

// Main function
mvm::Code MyExtension::FullDatabase(mvm::Code input, mvm::Address address, bool isReset, uint256_t blockNumber)
{
    // Kiểm tra kích thước input hợp lệ
    if (input.size() < 4)
    {
        std::cerr << "Error: Input size too small!" << std::endl;
        return mvm::Code(32, 0);
    }

    // Lấy opcode từ 4 byte đầu tiên
    uint32_t opCode = (input[0] << 24) | (input[1] << 16) | (input[2] << 8) | input[3];

    // Chuyển đổi địa chỉ thành dạng byte array
    uint256_t addr = address;
    std::vector<uint8_t> addressBytes(20);
    for (size_t i = 0; i < 20; ++i)
    {
        addressBytes[19 - i] = static_cast<uint8_t>(addr >> (i * 8));
    }

    // Xử lý các operation dựa vào opCode
    try
    {
        // GET_OR_CREATE_DB
        if (opCode == mvm::FunctionSelector::XAPIAN_GET_OR_CREATE_DB)
        {
            std::string selector, extracted_str;
            bool result = parseABI(input, selector, extracted_str);
            if (extracted_str.empty())
            {
                std::cerr << "Error: Extracted database name is empty!" << std::endl;
                return mvm::Code(32, 0);
            }

            std::filesystem::path fullPath = mvm::createFullPath(address, extracted_str);

            if (!std::filesystem::exists(fullPath))
            {
                std::filesystem::create_directories(fullPath);
            }

            auto manager = XapianManager::getInstance(extracted_str, address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << fullPath.string() << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            json uint256Abi = {{"type", "uint256"}};
            std::string hexNumber = decimalToHex(true);
            return encodeArgument(uint256Abi, hexNumber);
        }

        // NEW_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_NEW_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "string", "name": "data", "type": "string"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (!manager)
            {
                std::cerr << "Failed to create XapianManager" << std::endl;
                return mvm::Code(32, 0);
            }

            auto newDocID = manager->new_document(input_argument["data"].get<std::string>(), blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string hexNumber = decimalToHex(newDocID);
            return encodeArgument(uint256Abi, hexNumber);
        }

        // GET_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_GET_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->get_document(static_cast<int>(number), blockNumber);

            return mvm::Code(32, 0);
        }

        // DELETE_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_DELETE_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);
            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->delete_document(static_cast<int>(number), blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string hexNumber = decimalToHex(docInfo);
            auto encodedData = encodeArgument(uint256Abi, hexNumber);
            printHex(encodedData);

            return encodedData;
        }

        // ADD_TERM_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_ADD_TERM_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"},
                {"internalType": "string", "name": "term", "type": "string"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();
            std::string term = input_argument["term"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            // *** Kiểm tra dbname ***
            if (term.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (term.length() >= TERM_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (TERM_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->add_term(
                static_cast<int>(number), input_argument["term"], blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string value = std::to_string(docInfo);
            std::string hexNumber = decimalToHex(docInfo);
            auto encodedData = encodeArgument(uint256Abi, hexNumber);

            return encodedData;
        }

        // ADD_TERM_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_INDEX_TEXT_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"},
                {"internalType": "string", "name": "text", "type": "string"},
                {"internalType": "uint8", "name": "weight", "type": "uint8"},
                {"internalType": "string", "name": "prefix", "type": "string"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 docId = intx::from_string<intx::uint256>(hex_str);
            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->index_text(
                static_cast<int>(docId), input_argument["text"], hex_to_uint64(input_argument["weight"]), input_argument["prefix"], blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string value = std::to_string(docInfo);
            std::string hexNumber = decimalToHex(docInfo);
            auto encodedData = encodeArgument(uint256Abi, hexNumber);
            return encodedData;
        }

        // SET_DATA_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_SET_DATA_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"},
                {"internalType": "string", "name": "data", "type": "string"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);
            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->set_data(
                static_cast<int>(number), input_argument["data"], blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string hexNumber = decimalToHex(docInfo);
            return encodeArgument(uint256Abi, hexNumber);
        }

        // ADD_VALUE_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_ADD_VALUE_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"},
                {"internalType": "uint256", "name": "slot", "type": "uint256"},
                {"internalType": "string", "name": "data", "type": "string"},
                {"internalType": "bool", "name": "isSerialise", "type": "bool"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->add_value(
                hex_to_uint64(input_argument["docId"]), hex_to_uint64(input_argument["slot"]), input_argument["data"], input_argument["isSerialise"].get<bool>(), blockNumber);

            json uint256Abi = {{"type", "uint256"}};
            std::string value = decimalToHex(docInfo);

            auto encodedData = encodeArgument(uint256Abi, value);
            printHex(encodedData);

            return encodedData;
        }

        // GET_DATA_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_GET_DATA_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);
            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->get_data(static_cast<int>(number), blockNumber);

            json stringAbi = {{"type", "string"}};

            auto encodedData = encodeArgument(stringAbi, docInfo);

            return addOffsetPrefix(encodedData);
        }

        // GET_TERMS_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_GET_TERMS_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            std::string hex_str = "0x" + input_argument["docId"].get<std::string>();
            intx::uint256 number = intx::from_string<intx::uint256>(hex_str);
            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->get_terms(static_cast<int>(number), blockNumber);
            printDocInfo(docInfo);

            json stringArrayAbi = {{"type", "string[]"}};
            auto encodedData = encodeArgument(stringArrayAbi, joinStringArgument(docInfo));

            printHex(encodedData);
            return addOffsetPrefix(encodedData);
        }

        // GET_VALUE_DOCUMENT
        if (opCode == mvm::FunctionSelector::XAPIAN_GET_VALUE_DOCUMENT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"},
                {"internalType": "uint256", "name": "docId", "type": "uint256"},
                {"internalType": "uint256", "name": "slot", "type": "uint256"},
                {"internalType": "bool", "name": "isSerialise", "type": "bool"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto docInfo = manager->get_value(
                hex_to_uint64(input_argument["docId"]), hex_to_uint64(input_argument["slot"]), input_argument["isSerialise"].get<bool>(), blockNumber);

            json stringAbi = {{"type", "string"}};
            auto encodedData = encodeArgument(stringAbi, docInfo);
            printHex(encodedData);
            return addOffsetPrefix(encodedData);
        }

        if (opCode == mvm::FunctionSelector::XAPIAN_QUERY_SEARCH)
        {
            string abi_string = R"([
      {
        "name": "dbName",
        "type": "string"
      },
      {
        "name": "options",
        "type": "tuple",
        "components": [
          {
            "name": "queries",
            "type": "string"
          },
          {
            "name": "prefixMap",
            "type": "tuple[]",
            "components": [
              {
                "name": "key",
                "type": "string"
              },
              {
                "name": "value",
                "type": "string"
              }
            ]
          },
          {
            "name": "stopWords",
            "type": "string[]"
          },
          {
            "name": "offset",
            "type": "uint64"
          },
          {
            "name": "limit",
            "type": "uint64"
          },
          {
            "name": "sortByValueSlot",
            "type": "int64"
          },
          {
            "name": "sortAscending",
            "type": "bool"
          },
          {
            "name": "rangeFilters",
            "type": "tuple[]",
            "components": [
              {
                "name": "slot",
                "type": "uint256"
              },
              {
                "name": "begin",
                "type": "string"
              },
              {
                "name": "end",
                "type": "string"
              }
            ]
          }
        ]
      }
    ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            std::string dbName = getDbNameFromABI(input_without_opcode);
            // Giải mã dữ liệu
            json decodedData = decode(input_without_opcode, abi_string);

            // *** Kiểm tra dbname ***
            if (dbName.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbName.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbName.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }
            std::filesystem::path fullPath = mvm::createFullPath(address, dbName);

            XapianSearcher searcher(fullPath);
            std::vector<std::string> queries1 = {decodedData["options"]["queries"]};

            std::map<std::string, std::string> product_prefix_map = convertJsonToMap(decodedData["options"]["prefixMap"]);
            std::optional<std::vector<std::string>> stop_words_list = convertJsonToStopWordsList(decodedData["options"]["stopWords"]);

            std::optional<std::string> stem_lang = std::nullopt;
            Xapian::doccount offset = 0;
            try
            {
                offset = hex_to_uint64(decodedData["options"]["offset"]);
            }
            catch (...)
            {
                // Giữ giá trị mặc định nếu có lỗi
            }

            // Gán limit với giá trị mặc định là 10
            Xapian::doccount limit = 10;
            try
            {
                limit = hex_to_uint64(decodedData["options"]["limit"]);
            }
            catch (...)
            {
                // Giữ giá trị mặc định nếu có lỗi
            }

            // Gán sort_by_value_slot với giá trị mặc định là 0
            std::optional<Xapian::valueno> sort_by_value_slot = std::nullopt;
            try
            {

                auto sort_slot = hex_to_int64(decodedData["options"]["sortByValueSlot"]);

                if (sort_slot.has_value())
                {
                    if (sort_slot >= 0)
                        sort_by_value_slot = sort_slot;
                }
            }
            catch (...)
            {
                // Giữ giá trị mặc định nếu có lỗi
            }

            bool sort_ascending = true;

            try
            {
                sort_ascending = decodedData["options"]["sortAscending"].get<bool>();
            }
            catch (...)
            {
                // Giữ giá trị mặc định nếu có lỗi
            }

            std::vector<RangeFilter> range_filters = convertJsonToRangeFilters(decodedData["options"]);
            std::cerr << "[searcher] dumpIndex" << std::endl;

            searcher.dumpIndex();
            std::cerr << "[searcher] dumpIndex: " << blockNumber << std::endl;

            auto [results1, total1] = searcher.search(
                queries1, Xapian::Query::OP_AND, Xapian::Query::OP_AND, product_prefix_map, stem_lang, stop_words_list,
                offset, limit, sort_by_value_slot, sort_ascending, range_filters, blockNumber);

            auto dataReturn = searcher.encodeSearchResultsPage(total1, results1);
            std::cerr << "[searcher] results1 size: " << results1.size() << std::endl;

            return addOffsetPrefix(dataReturn);
        }

        if (opCode == mvm::FunctionSelector::XAPIAN_COMMIT)
        {
            std::string inputABI = R"([
                {"internalType": "string", "name": "dbname", "type": "string"}
            ])";

            auto input_without_opcode = getInputWithoutOpcode(input);
            nlohmann::json input_argument = decode(input_without_opcode, inputABI);

            std::string dbname = input_argument["dbname"].get<std::string>();

            // *** Kiểm tra dbname ***
            if (dbname.empty())
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname cannot be empty." << std::endl;
                return mvm::Code(32, 0);
            }
            if (dbname.length() >= DBNAME_MAX_LEN)
            {
                std::cerr << "[Error] FullDatabase (XAPIAN_NEW_DOCUMENT): dbname length (" << dbname.length()
                          << ") exceeds maximum of " << (DBNAME_MAX_LEN - 1) << " characters." << std::endl;
                return mvm::Code(32, 0);
            }

            auto manager = XapianManager::getInstance(input_argument["dbname"], address, isReset);
            if (manager)
            {
                registry.registerManager(this->mvmId, manager);
            }
            else
            {
                std::cerr << "Lỗi: Không thể lấy/tạo XapianManager cho " << input_argument["dbname"] << std::endl;
                return mvm::Code(32, 0); // Trả về lỗi
            }
            auto hash = manager->getChangeHash();
            auto log = manager->getChangeLogs();
            auto status = registry.commitChangesForMvmId(this->mvmId);
            json stringAbi = {{"type", "uint256"}};
            std::string hexNumber = decimalToHex(status);
            auto encodedData = encodeArgument(stringAbi, hexNumber);
            printHex(encodedData);
            return encodedData;
        }
    }
    catch (const std::exception &e)
    {
        std::cerr << "Error in operation: " << e.what() << std::endl;
    }
    catch (...)
    {
        std::cerr << "Unknown error" << std::endl;
    }

    return mvm::Code(32, 0);
}

mvm::Code MyExtension::Ecrecover(mvm::Code input)
{
    // Kiểm tra kích thước input hợp lệ
    if (input.size() < 4)
    {
        std::cerr << "Error: Input size too small!" << std::endl;
        return mvm::Code(32, 0);
    }

    // Lấy opcode từ 4 byte đầu tiên
    uint32_t opCode = (input[0] << 24) | (input[1] << 16) | (input[2] << 8) | input[3];

    std::string inputABI = R"([
                {"internalType": "bytes32", "name": "hash", "type": "bytes32"},
                {"internalType": "uint256", "name": "v", "type": "uint8"},
                {"internalType": "uint256", "name": "r", "type": "bytes32"},
                {"internalType": "bool", "name": "s", "type": "bytes32"}
            ])";

    nlohmann::json input_argument = decode(input, inputABI);

    std::string hash = input_argument["hash"];
    std::string v = input_argument["v"];
    std::string r = input_argument["r"];
    std::string s = input_argument["s"];

    uint64_t v_uint64 = 0;

    // Assuming decode returns v as a hex string representation of uint8
    v_uint64 = std::stoull(input_argument.at("v").get<std::string>(), nullptr, 16);

    if (v_uint64 != 27 && v_uint64 != 28)
    { // Or other valid range if needed
        std::cerr << "Error: Invalid v value decoded from ABI: " << v_uint64 << ". Expected 27 or 28." << std::endl;
        return mvm::Code(32, 0); // Return error immediately
    }
    uint8_t v_val = static_cast<uint8_t>(v_uint64); // This holds the correct v (e.g., 27 or 28)

    auto hbyte = hexString32ToBytes(hash);

    auto rbyte = hexString32ToBytes(r);
    auto sbyte = hexString32ToBytes(s);

    auto address = ecrecover(hbyte, v_val, rbyte, sbyte);
    mvm::Code result_code(32, 0);
    if (!address.empty() && address.size() == 20)
    {
        std::copy(address.begin(), address.end(), result_code.begin() + 12);
    }
    else if (address.empty())
    {
        std::cerr << "ecrecover returned empty address, check logs." << std::endl;
    }
    else
    {
        std::cerr << "ecrecover returned address of unexpected size: " << address.size() << std::endl;
    }

    return result_code;
}

mvm::Code MyExtension::Sha256(mvm::Code input)
{

    printHex(input);

    // Chuẩn bị bộ đệm cho kết quả hash (32 bytes)
    std::vector<std::byte> hash_result_buffer(mvm::crypto::SHA256_HASH_SIZE);

    // Gọi hàm sha256
    mvm::crypto::sha256(
        hash_result_buffer.data(),                         // Con trỏ đến bộ đệm kết quả
        reinterpret_cast<const std::byte *>(input.data()), // Con trỏ đến dữ liệu đầu vào
        input.size()                                       // Kích thước dữ liệu đầu vào
    );

    // Chuyển đổi kết quả std::byte thành mvm::Code (std::vector<uint8_t>)
    mvm::Code final_hash_result;
    final_hash_result.resize(mvm::crypto::SHA256_HASH_SIZE);
    for (size_t i = 0; i < mvm::crypto::SHA256_HASH_SIZE; ++i)
    {
        final_hash_result[i] = static_cast<uint8_t>(hash_result_buffer[i]);
    }

    return final_hash_result; // Trả về hash 32 byte
}

mvm::Code MyExtension::Ripemd160(mvm::Code input)
{

    // Chuẩn bị bộ đệm cho kết quả hash (20 bytes)
    std::vector<std::byte> hash_result_buffer(mvm::crypto::RIPEMD160_HASH_SIZE);

    // Gọi hàm ripemd160
    mvm::crypto::ripemd160(
        hash_result_buffer.data(),                         // Con trỏ đến bộ đệm kết quả
        reinterpret_cast<const std::byte *>(input.data()), // Con trỏ đến dữ liệu đầu vào
        input.size()                                       // Kích thước dữ liệu đầu vào
    );                                                     // Thêm noexcept nếu hàm thực sự là noexcept

    // Chuyển đổi kết quả std::byte thành mvm::Code (std::vector<uint8_t>)
    // Kết quả của RIPEMD160 là 20 byte, nhưng precompile Ethereum thường trả về 32 byte
    // với 12 byte đầu là 0.
    mvm::Code final_hash_result(32, 0); // Tạo vector 32 byte chứa giá trị 0
    // Sao chép 20 byte hash vào 20 byte cuối của vector kết quả
    for (size_t i = 0; i < mvm::crypto::RIPEMD160_HASH_SIZE; ++i)
    {
        final_hash_result[12 + i] = static_cast<uint8_t>(hash_result_buffer[i]);
    }

    return final_hash_result; // Trả về hash 32 byte (đã đệm)
}

// --- Hàm trợ giúp để in vector byte dưới dạng hex ---
std::string bytes_to_hex_string(const std::vector<uint8_t> &bytes)
{
    std::ostringstream oss;
    oss << "0x";
    for (uint8_t b : bytes)
    {
        oss << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(b);
    }
    return oss.str();
}

// Hàm helper mới để đọc 32 bytes và chuyển thành uint32_t
// Trả về giá trị hoặc ném lỗi nếu giá trị quá lớn cho uint32_t
uint32_t read_uint256_as_uint32_be(const std::vector<uint8_t> &data, size_t offset)
{
    if (offset + 32 > data.size())
    {
        std::cerr << "[MODEXP LOG][ERROR] read_uint256_as_uint32_be out of range at offset " << offset << std::endl;
        // Ném lỗi hoặc trả về giá trị đặc biệt tùy theo cách xử lý lỗi của bạn
        throw std::out_of_range("read_uint256_as_uint32_be out of range");
    }

    // Sử dụng hàm từ thư viện intx (giả sử mvm::from_big_endian tương tự)
    uint256_t value256 = mvm::from_big_endian(data.data() + offset, 32);

    // Kiểm tra xem giá trị có nằm trong giới hạn của uint32_t không
    if (value256 > std::numeric_limits<uint32_t>::max())
    {
        std::cerr << "[MODEXP LOG][ERROR] Size value read from offset " << offset << " exceeds uint32_t max." << std::endl;
        throw std::overflow_error("Size value exceeds uint32_t max");
    }

    return static_cast<uint32_t>(value256); // Ép kiểu an toàn sau khi kiểm tra
}

// Hàm Modexp với logging
mvm::Code MyExtension::Modexp(mvm::Code input)
{
    // --- Log đầu vào ---
    std::cerr << "[MODEXP LOG] Function Entry. Input data (" << input.size() << " bytes): "
              << bytes_to_hex_string(input) << std::endl;

    const size_t header_size = 3 * 32;
    if (input.size() < header_size)
    {
        std::cerr << "[MODEXP LOG][ERROR] Input size " << input.size() << " is less than header size " << header_size << std::endl;
        return {}; // Input quá ngắn
    }

    // --- Log các size đọc được ---
    uint32_t b_size = read_uint256_as_uint32_be(input, 0);
    uint32_t e_size = read_uint256_as_uint32_be(input, 32);
    uint32_t m_size = read_uint256_as_uint32_be(input, 64);
    std::cerr << "[MODEXP LOG] Parsed Sizes: Bsize=" << b_size << ", Esize=" << e_size << ", Msize=" << m_size << std::endl;

    uint64_t data_size = static_cast<uint64_t>(b_size) + e_size + m_size;
    uint64_t expected_min_size = header_size;
    if (std::numeric_limits<uint64_t>::max() - header_size < data_size)
    {
        std::cerr << "[MODEXP LOG][ERROR] Integer overflow calculating total data size." << std::endl;
        return {}; // Tràn số
    }
    expected_min_size += data_size;

    if (input.size() < expected_min_size)
    {
        std::cerr << "[MODEXP LOG][ERROR] Input size " << input.size() << " is less than expected minimum size " << expected_min_size << std::endl;
        return {}; // Kích thước input không đủ
    }

    // --- Log các offset tính toán được ---
    const size_t b_offset = header_size;
    const size_t e_offset = b_offset + b_size;
    const size_t m_offset = e_offset + e_size;
    std::cerr << "[MODEXP LOG] Calculated Offsets: B=" << b_offset << ", E=" << e_offset << ", M=" << m_offset << std::endl;

    // --- Sử dụng GMP ---
    mpz_t gmp_base, gmp_exp, gmp_mod, gmp_result;
    mpz_inits(gmp_base, gmp_exp, gmp_mod, gmp_result, nullptr);
    std::cerr << "[MODEXP LOG] GMP variables initialized." << std::endl;

    std::vector<uint8_t> mutable_input = input; // Tạo bản sao không const

    // Import và Log Base
    if (b_size > 0)
    {
        mpz_import(gmp_base, b_size, 1, sizeof(uint8_t), 1, 0, mutable_input.data() + b_offset);
        char *b_str = mpz_get_str(nullptr, 16, gmp_base); // Lấy chuỗi hex
        std::cerr << "[MODEXP LOG] Imported Base (B): 0x" << (b_str ? b_str : "null") << std::endl;
        if (b_str)
            free(b_str); // Giải phóng bộ nhớ cấp phát bởi mpz_get_str
    }
    else
    {
        std::cerr << "[MODEXP LOG] Imported Base (B): 0 (size was 0)" << std::endl;
    }

    // Import và Log Exponent
    if (e_size > 0)
    {
        mpz_import(gmp_exp, e_size, 1, sizeof(uint8_t), 1, 0, mutable_input.data() + e_offset);
        char *e_str = mpz_get_str(nullptr, 16, gmp_exp);
        std::cerr << "[MODEXP LOG] Imported Exponent (E): 0x" << (e_str ? e_str : "null") << std::endl;
        if (e_str)
            free(e_str);
    }
    else
    {
        std::cerr << "[MODEXP LOG] Imported Exponent (E): 0 (size was 0)" << std::endl;
    }

    // Import và Log Modulus
    if (m_size > 0)
    {
        mpz_import(gmp_mod, m_size, 1, sizeof(uint8_t), 1, 0, mutable_input.data() + m_offset);
        char *m_str = mpz_get_str(nullptr, 16, gmp_mod);
        std::cerr << "[MODEXP LOG] Imported Modulus (M): 0x" << (m_str ? m_str : "null") << std::endl;
        if (m_str)
            free(m_str);
    }
    else
    {
        std::cerr << "[MODEXP LOG] Imported Modulus (M): 0 (size was 0)" << std::endl;
    }

    // Xử lý M = 0
    if (m_size == 0 || mpz_sgn(gmp_mod) == 0)
    {
        std::cerr << "[MODEXP LOG] Modulus is zero. Returning empty result." << std::endl;
        mpz_clears(gmp_base, gmp_exp, gmp_mod, gmp_result, nullptr);
        return {};
    }

    // --- Log trước khi tính toán ---
    char *b_str_pre = mpz_get_str(nullptr, 16, gmp_base);
    char *e_str_pre = mpz_get_str(nullptr, 16, gmp_exp);
    char *m_str_pre = mpz_get_str(nullptr, 16, gmp_mod);
    std::cerr << "[MODEXP LOG] Calling mpz_powm with B=0x" << (b_str_pre ? b_str_pre : "null")
              << ", E=0x" << (e_str_pre ? e_str_pre : "null")
              << ", M=0x" << (m_str_pre ? m_str_pre : "null") << std::endl;
    if (b_str_pre)
        free(b_str_pre);
    if (e_str_pre)
        free(e_str_pre);
    if (m_str_pre)
        free(m_str_pre);

    // Tính B^E mod M bằng hàm của GMP
    mpz_powm(gmp_result, gmp_base, gmp_exp, gmp_mod);

    // --- Log kết quả tính toán (dạng số nguyên lớn) ---
    char *res_str = mpz_get_str(nullptr, 16, gmp_result);
    std::cerr << "[MODEXP LOG] mpz_powm result: 0x" << (res_str ? res_str : "null") << std::endl;
    if (res_str)
        free(res_str);

    // Export kết quả về dạng byte array, có kích thước bằng m_size
    mvm::Code result_bytes(m_size, 0); // Vector kết quả có kích thước m_size, đầy số 0
    size_t bytes_written = 0;
    // Tính toán offset để căn lề phải
    size_t result_num_bytes = (mpz_sizeinbase(gmp_result, 2) + 7) / 8; // Số byte thực tế của kết quả
    size_t export_offset = (m_size > result_num_bytes) ? (m_size - result_num_bytes) : 0;

    std::cerr << "[MODEXP LOG] Exporting result. Target size=" << m_size << ", Actual result bytes=" << result_num_bytes << ", Export offset=" << export_offset << std::endl;

    mpz_export(result_bytes.data() + export_offset, // Ghi vào vị trí đã tính toán
               &bytes_written, 1, sizeof(uint8_t), 1, 0, gmp_result);

    std::cerr << "[MODEXP LOG] Bytes written by mpz_export: " << bytes_written << std::endl;
    std::cerr << "[MODEXP LOG] Final result bytes before return (" << result_bytes.size() << " bytes): "
              << bytes_to_hex_string(result_bytes) << std::endl;

    // Dọn dẹp bộ nhớ GMP
    mpz_clears(gmp_base, gmp_exp, gmp_mod, gmp_result, nullptr);
    std::cerr << "[MODEXP LOG] GMP variables cleared. Function exiting." << std::endl;

    return result_bytes;
}
/**
 * @brief Implements ECADD precompile (alt_bn128 addition)
 * Mimics the style of the Sha256 example function.
 * Handles input parsing, validation, point addition, and output serialization.
 * Returns empty Code on error.
 * NOTE: Gas handling is NOT included here and must be managed by the caller.
 *
 * @param input Input data (128 bytes: x1, y1, x2, y2)
 * @return mvm::Code Output data (64 bytes: x, y) or empty Code on error.
 */
mvm::Code MyExtension::EcAdd(mvm::Code input)
{
    // Use the namespace definitions from the header provided by the user
    // Using directive inside the function scope for brevity

    constexpr size_t EXPECTED_INPUT_SIZE = 128;
    constexpr size_t COORD_SIZE = 32;
    constexpr size_t OUTPUT_SIZE = 64;

    if (input.size() != EXPECTED_INPUT_SIZE)
    {
        std::cerr << "ECADD Error: Invalid input size. Expected " << EXPECTED_INPUT_SIZE
                  << ", got " << input.size() << std::endl;
        return {}; // Return empty Code on error
    }

    try
    {
        // 1. Parse Input Coordinates using confirmed function from util.h
        const uint8_t *data = input.data();
        uint256_t x1_u = mvm::from_big_endian(data + 0 * COORD_SIZE);
        uint256_t y1_u = mvm::from_big_endian(data + 1 * COORD_SIZE);
        uint256_t x2_u = mvm::from_big_endian(data + 2 * COORD_SIZE);
        uint256_t y2_u = mvm::from_big_endian(data + 3 * COORD_SIZE);

        // 2. Validate Coordinates are valid Field Elements (< Modulus)
        //    Use the FieldPrime constant from the user-provided header
        if (x1_u >= FieldPrime || y1_u >= FieldPrime || // <-- Use identifier directly due to 'using namespace'
            x2_u >= FieldPrime || y2_u >= FieldPrime)
        {
            std::cerr << "ECADD Error: Coordinate out of field range." << std::endl;
            return {}; // Return empty Code on error
        }

        // 3. Create Point objects
        //    Use the Point type from the user-provided header
        //    Assuming Fp is essentially uint256_t based on Point definition.
        Point p1 = {uint256_t(x1_u), uint256_t(y1_u)};
        Point p2 = {uint256_t(x2_u), uint256_t(y2_u)};

        // Define infinity using the standard (0,0) encoding
        const Point infinity_point = {uint256_t(0), uint256_t(0)};

        // 4. Validate Points are on the curve
        //    Use the validate function from the user-provided header
        if (p1 != infinity_point && !validate(p1))
        { // <-- Use identifier directly
            std::cerr << "ECADD Error: Point P1 is not on the curve." << std::endl;
            return {};
        }
        if (p2 != infinity_point && !validate(p2))
        { // <-- Use identifier directly
            std::cerr << "ECADD Error: Point P2 is not on the curve." << std::endl;
            return {};
        }

        // 5. Perform Point Addition
        //    Use the add function from the user-provided header
        Point result_p = add(p1, p2); // <-- Use identifier directly

        // 6. Serialize Output Coordinates using confirmed function from util.h
        mvm::Code output(OUTPUT_SIZE);
        mvm::to_big_endian(uint256_t(result_p.x), output.data() + 0 * COORD_SIZE);
        mvm::to_big_endian(uint256_t(result_p.y), output.data() + 1 * COORD_SIZE);

        return output; // Return the 64-byte result
    }
    catch (const std::exception &e)
    {
        std::cerr << "ECADD Error: Exception during execution: " << e.what() << std::endl;
        return {};
    }
    catch (...)
    {
        std::cerr << "ECADD Error: Unknown exception during execution." << std::endl;
        return {};
    }
}

/**
 * @brief Implements ECMUL precompile (alt_bn128 scalar multiplication)
 * Mimics the style of the EcAdd example function.
 * Handles input parsing, validation, scalar multiplication, and output serialization.
 * Returns empty Code on error.
 * NOTE: Gas handling is NOT included here and must be managed by the caller.
 *
 * @param input Input data (96 bytes: x1, y1, s)
 * @return mvm::Code Output data (64 bytes: x, y) or empty Code on error.
 */
mvm::Code MyExtension::EcMul(mvm::Code input)
{
    // Use the namespace definitions from the header provided by the user

    constexpr size_t EXPECTED_INPUT_SIZE = 96; // ECMUL input is 96 bytes (Point + Scalar)
    constexpr size_t COORD_SIZE = 32;
    constexpr size_t SCALAR_OFFSET = 2 * COORD_SIZE; // Scalar starts after x1, y1
    constexpr size_t OUTPUT_SIZE = 64;

    if (input.size() != EXPECTED_INPUT_SIZE)
    {
        std::cerr << "ECMUL Error: Invalid input size. Expected " << EXPECTED_INPUT_SIZE
                  << ", got " << input.size() << std::endl;
        return {}; // Return empty Code on error
    }

    try
    {
        // 1. Parse Input Coordinates and Scalar
        const uint8_t *data = input.data();
        uint256_t x1_u = mvm::from_big_endian(data + 0 * COORD_SIZE); //
        uint256_t y1_u = mvm::from_big_endian(data + 1 * COORD_SIZE); //
        uint256_t s_u = mvm::from_big_endian(data + SCALAR_OFFSET);   //

        // 2. Validate Coordinates are valid Field Elements (< Modulus)
        //    Use the FieldPrime constant
        if (x1_u >= FieldPrime || y1_u >= FieldPrime)
        {
            std::cerr << "ECMUL Error: Coordinate out of field range." << std::endl;
            return {}; // Return empty Code on error
        }
        // Scalar 's' does not need to be < FieldPrime, it's interpreted mod curve order usually,
        // but the precompile spec doesn't mention failing for large scalars.
        // The underlying 'mul' function should handle the scalar correctly.

        // 3. Create Point object
        //    Use the Point type
        Point p1 = {uint256_t(x1_u), uint256_t(y1_u)};

        // Define infinity using the standard (0,0) encoding
        const Point infinity_point = {uint256_t(0), uint256_t(0)};

        // 4. Validate Point is on the curve
        //    Use the validate function
        if (p1 != infinity_point && !validate(p1))
        {
            std::cerr << "ECMUL Error: Input point is not on the curve." << std::endl;
            return {};
        }

        // 5. Perform Scalar Multiplication
        //    Use the mul function from the header
        Point result_p = mul(p1, s_u);

        // 6. Serialize Output Coordinates
        mvm::Code output(OUTPUT_SIZE);
        mvm::to_big_endian(uint256_t(result_p.x), output.data() + 0 * COORD_SIZE); //
        mvm::to_big_endian(uint256_t(result_p.y), output.data() + 1 * COORD_SIZE); //

        return output; // Return the 64-byte result
    }
    catch (const std::exception &e)
    {
        std::cerr << "ECMUL Error: Exception during execution: " << e.what() << std::endl;
        return {};
    }
    catch (...)
    {
        std::cerr << "ECMUL Error: Unknown exception during execution." << std::endl;
        return {};
    }
}

/**
 * @brief Implements ECPAIRING precompile (alt_bn128 pairing check)
 * Mimics the style of the EcAdd/EcMul example functions.
 * Handles input parsing, pairing check, and output serialization.
 * Assumes the underlying pairing_check function handles point validation.
 * Returns empty Code on error.
 * NOTE: Gas handling (base + per-pair cost) is NOT included here and must be managed by the caller.
 *
 * @param input Input data (multiple of 192 bytes: [x1_g1, y1_g1, x1_re_g2, x1_im_g2, y1_re_g2, y1_im_g2]...)
 * @return mvm::Code Output data (32 bytes: 1 for success, 0 for failure) or empty Code on error.
 */
mvm::Code MyExtension::EcPairing(mvm::Code input)
{
    // using namespace evmmax::bn254; // Adjust if needed

    constexpr size_t G1_POINT_SIZE = 64;                        // 2 * 32 bytes
    constexpr size_t G2_POINT_SIZE = 128;                       // 4 * 32 bytes
    constexpr size_t PAIR_SIZE = G1_POINT_SIZE + G2_POINT_SIZE; // 192 bytes
    constexpr size_t COORD_SIZE = 32;                           // Size of one coordinate field element
    constexpr size_t OUTPUT_SIZE = 32;                          // Standard EVM word size

    if (input.size() % PAIR_SIZE != 0)
    {
        std::cerr << "[EcPairing] Error: Invalid input size. Must be multiple of " << PAIR_SIZE
                  << ", got " << input.size() << std::endl;
        return {}; // Return empty Code on error
    }

    const size_t num_pairs = input.size() / PAIR_SIZE;

    // If input is empty (0 pairs), the result is success (1)
    if (num_pairs == 0)
    {
        mvm::Code output(OUTPUT_SIZE, 0);
        mvm::to_big_endian(uint256_t(1), output.data()); // Success
        return output;
    }

    std::vector<std::pair<Point, ExtPoint>> pairs;
    pairs.reserve(num_pairs);

    try
    {
        const uint8_t *data = input.data();

        for (size_t i = 0; i < num_pairs; ++i)
        {
            const uint8_t *current_pair_data = data + i * PAIR_SIZE;

            // 1. Parse G1 Point (P) - No changes needed here
            const uint8_t *p1_data = current_pair_data;
            uint256_t p1_x = mvm::from_big_endian(p1_data + 0 * COORD_SIZE);
            uint256_t p1_y = mvm::from_big_endian(p1_data + 1 * COORD_SIZE);

            // 2. Parse G2 Point (Q) - **MODIFIED SECTION**
            // EVM ABI Input format: [x_im, x_re, y_im, y_re]
            const uint8_t *p2_data = current_pair_data + G1_POINT_SIZE;
            uint256_t p2_x_im_in = mvm::from_big_endian(p2_data + 0 * COORD_SIZE); // Read Imaginary X from input[0]
            uint256_t p2_x_re_in = mvm::from_big_endian(p2_data + 1 * COORD_SIZE); // Read Real X from input[1]
            uint256_t p2_y_im_in = mvm::from_big_endian(p2_data + 2 * COORD_SIZE); // Read Imaginary Y from input[2]
            uint256_t p2_y_re_in = mvm::from_big_endian(p2_data + 3 * COORD_SIZE); // Read Real Y from input[3]

            // 3. Validate Coordinates (< FieldPrime) - Basic check using parsed values
            // Check against the actual FieldPrime constant
            if (p1_x >= FieldPrime || p1_y >= FieldPrime ||
                p2_x_re_in >= FieldPrime || p2_x_im_in >= FieldPrime || // Check both real and imaginary parts
                p2_y_re_in >= FieldPrime || p2_y_im_in >= FieldPrime)
            {
                std::cerr << "[EcPairing] Error: Coordinate out of field range in pair " << i << std::endl;
                // Print specific error details if needed for debugging
                if (p1_x >= FieldPrime)
                    std::cerr << "  P.x >= FieldPrime: " << mvm::to_hex_string(p1_x) << std::endl;
                if (p1_y >= FieldPrime)
                    std::cerr << "  P.y >= FieldPrime: " << mvm::to_hex_string(p1_y) << std::endl;
                if (p2_x_re_in >= FieldPrime)
                    std::cerr << "  Q.x_re >= FieldPrime: " << mvm::to_hex_string(p2_x_re_in) << std::endl;
                if (p2_x_im_in >= FieldPrime)
                    std::cerr << "  Q.x_im >= FieldPrime: " << mvm::to_hex_string(p2_x_im_in) << std::endl;
                if (p2_y_re_in >= FieldPrime)
                    std::cerr << "  Q.y_re >= FieldPrime: " << mvm::to_hex_string(p2_y_re_in) << std::endl;
                if (p2_y_im_in >= FieldPrime)
                    std::cerr << "  Q.y_im >= FieldPrime: " << mvm::to_hex_string(p2_y_im_in) << std::endl;
                return {}; // Return empty Code on error
            }

            // 4. Create Point objects
            // G1 point is straightforward
            Point p1 = {p1_x, p1_y};
            // G2 point: Assign parsed values to ExtPoint structure, assuming it expects {real, imaginary}
            ExtPoint p2 = {
                {p2_x_re_in, p2_x_im_in}, // Store as {real, imaginary}
                {p2_y_re_in, p2_y_im_in}  // Store as {real, imaginary}
            };

            // 5. Add pair to list
            pairs.emplace_back(p1, p2);
        }

        // *** Logging section (remains the same, uses the constructed pairs) ***
        int log_pair_idx = 0;
        for (const auto &pair_to_log : pairs)
        {
            const auto &p_log = pair_to_log.first;
            const auto &q_log = pair_to_log.second;
            // Add actual logging here if needed, e.g.:
            // std::cout << "[EcPairing] Debug Pair " << log_pair_idx << ":" << std::endl;
            // std::cout << "  P: x=" << mvm::to_hex_string(p_log.x) << " y=" << mvm::to_hex_string(p_log.y) << std::endl;
            // std::cout << "  Q: x={re=" << mvm::to_hex_string(q_log.x.first) << ", im=" << mvm::to_hex_string(q_log.x.second) << "}" << std::endl;
            // std::cout << "     y={re=" << mvm::to_hex_string(q_log.y.first) << ", im=" << mvm::to_hex_string(q_log.y.second) << "}" << std::endl;
            log_pair_idx++;
        }
        // *** End Logging section ***

        // 6. Perform Pairing Check
        std::optional<bool> check_result = pairing_check(std::span{pairs}); // Pass the vector of pairs

        // 7. Process Pairing Result and Serialize Output
        mvm::Code output(OUTPUT_SIZE, 0);

        if (check_result.has_value())
        {
            if (check_result.value())
            {
                mvm::to_big_endian(uint256_t(1), output.data()); // Success
            }
            else
            {
                // Pairing check evaluated to false (valid points, but pairing equation != 1)
                mvm::to_big_endian(uint256_t(0), output.data()); // Failure
            }
            return output;
        }
        else
        {
            // pairing_check returned std::nullopt, indicating an error during the check
            // This usually implies invalid points (e.g., not on curve, subgroup checks)
            // if the underlying library performs these checks.
            std::cerr << "[EcPairing] Error: pairing_check function indicated an error (e.g., invalid point)." << std::endl;
            // Return 0 as per EIP-197 failure modes (invalid input points lead to failure, not revert)
            // However, the prompt asks for empty Code on error, so we stick to that. If EVM compatibility
            // requires returning 0 on point validation failure within pairing_check, adjust here.
            return {}; // Return empty Code on error as per original function spec
            // Alternative (closer to EIP-197 failure):
            // mvm::to_big_endian(uint256_t(0), output.data()); // Indicate failure (0)
            // return output;
        }
    }
    catch (const std::exception &e)
    {
        std::cerr << "[EcPairing] Error: Exception during execution: " << e.what() << std::endl;
        return {}; // Return empty Code on error
    }
    catch (...)
    {
        std::cerr << "[EcPairing] Error: Unknown exception during execution." << std::endl;
        return {}; // Return empty Code on error
    }
}

// --- Helper function to read little-endian uint64_t ---
inline uint64_t read_le64(const uint8_t *ptr)
{
    uint64_t value = 0;
    std::memcpy(&value, ptr, sizeof(uint64_t));
    // Assuming the system is little-endian. If not, byte swap is needed.
    // On x86/x64, this memcpy is usually sufficient.
    // For cross-platform safety, check endianness or use explicit byte manipulation.
#if __BYTE_ORDER__ == __ORDER_BIG_ENDIAN__
    value = __builtin_bswap64(value);
#endif
    return value;
}

// --- Helper function to write little-endian uint64_t ---
inline void write_le64(uint8_t *ptr, uint64_t value)
{
#if __BYTE_ORDER__ == __ORDER_BIG_ENDIAN__
    value = __builtin_bswap64(value);
#endif
    std::memcpy(ptr, &value, sizeof(uint64_t));
}

// --- Helper function to read big-endian uint32_t ---
inline uint32_t read_be32(const uint8_t *ptr)
{
    uint32_t value = 0;
    std::memcpy(&value, ptr, sizeof(uint32_t));
#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
    value = __builtin_bswap32(value); // Use GCC/Clang intrinsic for byte swap
#endif
    // For MSVC use _byteswap_ulong
    // #elif defined(_MSC_VER)
    //     value = _byteswap_ulong(value);
    // #endif
    return value;
}

// Helper function to read a little-endian uint32_t from a byte array
inline uint32_t read_le32(const uint8_t *ptr)
{
    uint32_t value = 0;
    for (int i = 0; i < 4; ++i)
    {
        value |= static_cast<uint32_t>(ptr[i]) << (i * 8);
    }
    return value;
}

// --- Implementation for BLAKE2f Precompile ---
mvm::Code MyExtension::Blake2f(mvm::Code input)
{

    // --- Constants for BLAKE2f input structure ---
    constexpr size_t EXPECTED_INPUT_SIZE = 213;
    constexpr size_t ROUNDS_SIZE = 4;
    constexpr size_t H_SIZE = 64;  // 8 * 8 bytes
    constexpr size_t M_SIZE = 128; // 16 * 8 bytes
    constexpr size_t T_SIZE = 16;  // 2 * 8 bytes
    constexpr size_t F_SIZE = 1;
    constexpr size_t OUTPUT_SIZE = 64; // Output is the new state vector h

    // Offsets
    constexpr size_t H_OFFSET = ROUNDS_SIZE;
    constexpr size_t M_OFFSET = H_OFFSET + H_SIZE;
    constexpr size_t T_OFFSET = M_OFFSET + M_SIZE;
    constexpr size_t F_OFFSET = T_OFFSET + T_SIZE;

    // --- Input Validation ---
    if (input.size() != EXPECTED_INPUT_SIZE)
    {
        std::cerr << "BLAKE2f Error: Invalid input size. Expected " << EXPECTED_INPUT_SIZE
                  << ", got " << input.size() << std::endl;
        return {}; // Return empty code to signal error
    }

    const uint8_t *data = input.data();

    try
    {
        // --- Parse Input ---
        // 1. Rounds (4 bytes, big-endian unsigned integer)
        uint32_t rounds = read_be32(data); // Reads as uint32_t directly

        // 2. State vector h (64 bytes, 8 x 8-byte little-endian unsigned integers)
        //    Use C-style array matching the blake2b_compress signature
        uint64_t h_state[8];
        for (int i = 0; i < 8; ++i)
        {
            h_state[i] = read_le64(data + H_OFFSET + i * 8); //
        }

        // 3. Message block vector m (128 bytes, 16 x 8-byte little-endian unsigned integers)
        //    Use const C-style array matching the blake2b_compress signature
        uint64_t m_block[16]; // Non-const needed for read_le64, but function takes const
        for (int i = 0; i < 16; ++i)
        {
            m_block[i] = read_le64(data + M_OFFSET + i * 8); //
        }

        // 4. Offset counters t (16 bytes, 2 x 8-byte little-endian integers)
        //    Use const C-style array matching the blake2b_compress signature
        uint64_t t_counters[2];                         // Non-const needed for read_le64, but function takes const
        t_counters[0] = read_le64(data + T_OFFSET);     //
        t_counters[1] = read_le64(data + T_OFFSET + 8); //

        // 5. Final block indicator flag f (1 byte, 0 or 1)
        uint8_t f_byte = data[F_OFFSET]; //
        if (f_byte > 1)
        {
            std::cerr << "BLAKE2f Error: Invalid f flag value. Expected 0 or 1, got "
                      << static_cast<int>(f_byte) << std::endl;
            return {}; // Invalid flag
        }
        bool final_flag = (f_byte == 1); //

        // --- Call Core BLAKE2f Compression Function ---
        // The signature expects: uint32_t rounds, uint64_t h[8], const uint64_t m[16], const uint64_t t[2], bool last
        mvm::crypto::blake2b_compress(
            rounds,     // Already uint32_t
            h_state,    // Pass C-style array (will be modified in place)
            m_block,    // Pass C-style array
            t_counters, // Pass C-style array
            final_flag  // Pass bool flag
        );

        // --- Serialize Output ---
        // Output is the resulting state vector h (modified h_state)
        mvm::Code output(OUTPUT_SIZE);
        for (int i = 0; i < 8; ++i)
        {
            // Write the modified h_state back to the output buffer
            write_le64(output.data() + i * 8, h_state[i]); //
        }

        return output;
    }
    catch (const std::out_of_range &oor)
    {
        std::cerr << "BLAKE2f Error: Out of range during input parsing: " << oor.what() << std::endl;
        return {}; // Return empty code on parsing error
    }
    catch (const std::exception &e)
    {
        std::cerr << "BLAKE2f Error: Exception during execution: " << e.what() << std::endl;
        return {}; // Return empty code on other errors
    }
    catch (...)
    {
        std::cerr << "BLAKE2f Error: Unknown exception during execution." << std::endl;
        return {};
    }
}

// --- Implementation for Point Evaluation Precompile (0x0A) ---
mvm::Code MyExtension::PointEvaluationVerify(mvm::Code input)
{

    // --- Constants for Point Evaluation input structure ---
    constexpr size_t VERSIONED_HASH_OFFSET = 0;
    constexpr size_t VERSIONED_HASH_SIZE_BYTES = mvm::crypto::VERSIONED_HASH_SIZE; // 32
    constexpr size_t Z_OFFSET = VERSIONED_HASH_OFFSET + VERSIONED_HASH_SIZE_BYTES; // 32
    constexpr size_t Z_SIZE = 32;
    constexpr size_t Y_OFFSET = Z_OFFSET + Z_SIZE; // 64
    constexpr size_t Y_SIZE = 32;
    constexpr size_t COMMITMENT_OFFSET = Y_OFFSET + Y_SIZE; // 96
    constexpr size_t COMMITMENT_SIZE = 48;
    constexpr size_t PROOF_OFFSET = COMMITMENT_OFFSET + COMMITMENT_SIZE; // 144
    constexpr size_t PROOF_SIZE = 48;
    constexpr size_t EXPECTED_INPUT_SIZE = PROOF_OFFSET + PROOF_SIZE; // 192
    constexpr size_t OUTPUT_SIZE = 64;

    // --- Input Validation ---
    if (input.size() != EXPECTED_INPUT_SIZE)
    {
        std::cerr << "PointEvaluation Error: Invalid input size. Expected " << EXPECTED_INPUT_SIZE
                  << ", got " << input.size() << std::endl;
        // Return 64 bytes of 0 on failure as per EIP-4844 spec for failed verification
        return mvm::Code(OUTPUT_SIZE, 0);
    }

    const uint8_t *data_ptr = input.data();

    // --- Prepare pointers for kzg_verify_proof (casting to std::byte*) ---
    // Ensure alignment is not an issue; direct casting should be fine if data is byte-aligned.
    const std::byte *versioned_hash_ptr = reinterpret_cast<const std::byte *>(data_ptr + VERSIONED_HASH_OFFSET);
    const std::byte *z_ptr = reinterpret_cast<const std::byte *>(data_ptr + Z_OFFSET);
    const std::byte *y_ptr = reinterpret_cast<const std::byte *>(data_ptr + Y_OFFSET);
    const std::byte *commitment_ptr = reinterpret_cast<const std::byte *>(data_ptr + COMMITMENT_OFFSET);
    const std::byte *proof_ptr = reinterpret_cast<const std::byte *>(data_ptr + PROOF_OFFSET);

    // --- Call Core KZG Verification Function ---
    bool success = false;
    try
    {
        // Directly use the function from the provided header/namespace
        success = mvm::crypto::kzg_verify_proof(
            versioned_hash_ptr, z_ptr, y_ptr, commitment_ptr, proof_ptr);
    }
    catch (const std::exception &e)
    {
        // Catch potential exceptions from underlying crypto library if they exist
        std::cerr << "PointEvaluation Error: Exception during kzg_verify_proof: " << e.what() << std::endl;
        success = false; // Treat exceptions as verification failure
    }
    catch (...)
    {
        std::cerr << "PointEvaluation Error: Unknown exception during kzg_verify_proof." << std::endl;
        success = false; // Treat exceptions as verification failure
    }

    // --- Prepare Output ---
    mvm::Code output(OUTPUT_SIZE, 0); // Initialize with zeros

    if (success)
    {
        // On success, return (FIELD_ELEMENTS_PER_BLOB, BLS_MODULUS)
        try
        {
            // Use mvm::to_big_endian (or implement it if needed)
            mvm::to_big_endian(mvm::crypto::FIELD_ELEMENTS_PER_BLOB, output.data()); // First 32 bytes
            mvm::to_big_endian(mvm::crypto::BLS_MODULUS, output.data() + 32);        // Next 32 bytes
        }
        catch (const std::exception &e)
        {
            std::cerr << "PointEvaluation Error: Exception during output serialization: " << e.what() << std::endl;
            // If serialization fails unexpectedly, revert to failure output
            std::fill(output.begin(), output.end(), 0);
        }
    }
    else
    {
        // On failure, the output is already initialized to 64 zeros.
    }

    return output;
}
// --- Implementation for Point Evaluation Precompile (0x0A) ---
mvm::Code MyExtension::PublicKeyFromPrivateKey(mvm::Code input)
{
    mvm::Code output(32, 0);

    if (input.size() < 4)
    {
        return output;
    }

    // Lấy opcode từ 4 byte đầu tiên
    uint32_t opCode = (input[0] << 24) | (input[1] << 16) | (input[2] << 8) | input[3];

    auto input_without_opcode = getInputWithoutOpcode(input);

    if (opCode == mvm::FunctionSelector::ESCP_PFP)
    {
        std::vector<uint8_t> public_key_bytes;

        // Private key mẫu (32 bytes)
        auto private_key_hex = mvm::vector_to_string_format(input_without_opcode, true, false);

        intx::uint256 private_key = intx::from_string<intx::uint256>(private_key_hex);

        Point public_key = evmmax::secp256k1::private_key_to_public_key(private_key);
        if (public_key.is_inf())
        {
            return output;
        }
        else
        {
            // Sử dụng hàm helper mới nhất
            std::vector<uint8_t> x_bytes = mvm::uint256_to_vector(public_key.x); // Sẽ có 32 bytes
            std::vector<uint8_t> y_bytes = mvm::uint256_to_vector(public_key.y); // Sẽ có 32 bytes

            printHex(x_bytes);
            // Construct the uncompressed public key string
            // Đặt trước dung lượng cho vector cuối cùng: 1 byte (prefix) + 32 bytes (X) + 32 bytes (Y) = 65 bytes
            public_key_bytes.reserve(1 + x_bytes.size() + y_bytes.size());

            // Thêm byte tiền tố định dạng uncompressed (0x04)
            public_key_bytes.push_back(0x04);

            // Nối các byte của tọa độ X vào sau
            public_key_bytes.insert(public_key_bytes.end(), x_bytes.begin(), x_bytes.end());

            // Nối các byte của tọa độ Y vào sau
            public_key_bytes.insert(public_key_bytes.end(), y_bytes.begin(), y_bytes.end());

            return mvm::encode_abi_bytes(public_key_bytes);
        }
        return output;
    }

    return output;
}
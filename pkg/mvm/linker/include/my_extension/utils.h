#pragma once

#include <cstdint> // Thêm dòng này để định nghĩa uint8_t
#include <vector>
#include <mpfr.h>
#include <filesystem> // Cho std::filesystem::path
#include "mvm/util.h"

namespace mvm
{
    void hexToSignedInt(mpfr_t result, const std::vector<uint8_t> &bytes);
    void signedIntToHex(std::vector<uint8_t> &result_bytes, const mpfr_t number);
    std::vector<uint8_t> evm_encode_mpfr(const mpfr_t &value);
    std::filesystem::path createFullPath(const mvm::Address &address, const std::string &dbname);
}
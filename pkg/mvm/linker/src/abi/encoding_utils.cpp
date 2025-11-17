// encoding_utils.cpp
#include "abi/encoding_utils.h"
#include <stdexcept>
#include <algorithm> // for std::reverse
#include <limits>    // for std::numeric_limits
#include <iostream>

#include <iomanip>  

namespace encoding
{
void printHex2(const std::vector<uint8_t> &bytes)
{
    for (uint8_t byte : bytes)
    {
        std::cout << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(byte);
    }
    std::cout << std::dec << std::endl;
}
    void appendUint16Padded(std::vector<uint8_t> &buffer, uint16_t value)
    {
        size_t initial_size = buffer.size();
        buffer.resize(initial_size + 32, 0);             // Thêm 32 byte 0
        buffer[initial_size + 30] = (value >> 8) & 0xFF; // Byte cao
        buffer[initial_size + 31] = value & 0xFF;        // Byte thấp
    }

    void appendUint8Padded(std::vector<uint8_t> &buffer, uint8_t value)
    {
        size_t initial_size = buffer.size();
        buffer.resize(initial_size + 32, 0); // Thêm 32 byte 0
        buffer[initial_size + 31] = value;   // Ghi vào byte cuối
    }

    void appendBytesPadded(std::vector<uint8_t> &buffer, const uint8_t *data, size_t len)
    {
        size_t initial_size = buffer.size();
        size_t padded_len = static_cast<size_t>(std::ceil(static_cast<double>(len) / 32.0)) * 32;
        buffer.resize(initial_size + padded_len, 0); // Thêm dung lượng đã đệm

        if (data && len > 0)
        {
            std::memcpy(buffer.data() + initial_size, data, len); // Sao chép dữ liệu gốc
        }
        // Phần còn lại đã được khởi tạo là 0 khi resize
    }

    // Thêm các hàm tiện ích
    void appendString(std::vector<uint8_t> &buffer, const std::string &str)
    {
        // Encode độ dài string
        appendUint256(buffer, str.length());

        // Encode nội dung string với padding
        appendBytesPadded(buffer,
                          reinterpret_cast<const uint8_t *>(str.data()),
                          str.length());
    }

    uint64_t readUint256(const std::vector<uint8_t> &buffer, size_t offset)
    {
        if (offset + 32 > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading uint256");
        }

        uint64_t value = 0;
        for (int i = 0; i < 8; ++i)
        {
            value = (value << 8) | buffer[offset + 24 + i];
        }
        return value;
    }

    std::string readString(const std::vector<uint8_t> &buffer, size_t offset)
    {
        // Đọc độ dài string
        uint64_t length = readUint256(buffer, offset);
        offset += 32;

        if (offset + length > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading string");
        }

        return std::string(
            reinterpret_cast<const char *>(buffer.data() + offset),
            length);
    }

    size_t getPaddedSize(size_t len)
    {
        return ((len + 31) / 32) * 32; // Làm tròn lên đến bội số gần nhất của 32
    }

    // --- Helper function for endianness ---
    inline bool is_little_endian()
    {
        int num = 1;
        return (*reinterpret_cast<char *>(&num) == 1);
    }

    // --- Append functions ---

    // Encode uint64 vào 32 byte, Big Endian (chuẩn ABI)
    void appendUint256FromUint64(std::vector<uint8_t> &buffer, uint64_t value)
    {
        size_t initial_size = buffer.size();
        buffer.resize(initial_size + 32, 0); // Thêm 32 byte 0
        // Ghi 8 byte của value vào 8 byte cuối cùng của 32 byte (Big Endian)
        for (int i = 0; i < 8; ++i)
        {
            buffer[initial_size + 31 - i] = static_cast<uint8_t>((value >> (i * 8)) & 0xFF);
        }
    }
    // Giữ hàm cũ nếu cần encode uint64_t như là uint256
    void appendUint256(std::vector<uint8_t> &buffer, uint64_t value)
    {
        appendUint256FromUint64(buffer, value);
    }

    // Encode uint64 vào 1 slot 32 byte (Big Endian)
    void appendUint64Padded(std::vector<uint8_t> &buffer, uint64_t value)
    {
        appendUint256FromUint64(buffer, value); // uint64 cũng chiếm 32 byte slot
    }

    void appendBoolPadded(std::vector<uint8_t> &buffer, bool value)
    {
        size_t initial_size = buffer.size();
        buffer.resize(initial_size + 32, 0);                             // Thêm 32 byte 0
        buffer[initial_size + 31] = static_cast<uint8_t>(value ? 1 : 0); // Ghi 0 hoặc 1 vào byte cuối
    }

    // --- Read functions ---

    // Đọc 32 byte như uint256 nhưng chỉ lấy 8 byte cuối (Big Endian) thành uint64
    uint64_t readUint64Padded(const std::vector<uint8_t> &buffer, size_t offset)
    {
        if (offset + 32 > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading uint64 padded");
        }
        uint64_t value = 0;
        // Chỉ đọc 8 byte cuối cùng
        for (int i = 0; i < 8; ++i)
        {
            value = (value << 8) | buffer[offset + 24 + i];
        }
        return value;
    }

    // Đọc 32 byte như uint256 (Big Endian), trả về uint64. Cần cẩn thận tràn số.
    // Hàm này nên được dùng khi đọc offset hoặc length của kiểu động.
    uint64_t readUint256AsUint64(const std::vector<uint8_t> &buffer, size_t offset)
    {
        if (offset + 32 > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading uint256 as uint64");
        }
        // Kiểm tra các byte cao hơn có phải là 0 không để tránh tràn số uint64
        for (size_t i = 0; i < 24; ++i)
        {
            if (buffer[offset + i] != 0)
            {
                // Hoặc throw lỗi, hoặc trả về giá trị max(), hoặc xử lý khác
                // Ở đây tạm trả về max để biểu thị lỗi/tràn số tiềm ẩn
                // Hoặc throw: throw std::overflow_error("Value too large for uint64");
                return std::numeric_limits<uint64_t>::max();
            }
        }

        uint64_t value = 0;
        for (int i = 0; i < 8; ++i)
        {
            value = (value << 8) | buffer[offset + 24 + i];
        }
        return value;
    }

    bool readBoolPadded(const std::vector<uint8_t> &buffer, size_t offset)
    {
        if (offset + 32 > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading bool padded");
        }
        // Kiểm tra byte cuối cùng, các byte khác phải là 0
        for (size_t i = 0; i < 31; ++i)
        {
            if (buffer[offset + i] != 0)
            {
                // Hoặc throw lỗi về padding không hợp lệ
            }
        }
        return buffer[offset + 31] != 0;
    }

    // Đọc bytes từ vị trí data_offset với độ dài len (không có padding độ dài ở đầu)
    std::vector<uint8_t> readBytesPadded(const std::vector<uint8_t> &buffer, size_t data_offset, size_t len)
    {
        if (data_offset + len > buffer.size())
        { // Kiểm tra len thôi, padding có thể vượt quá
            throw std::out_of_range("Buffer overflow when reading bytes data");
        }
        // Kích thước thực tế cần đọc (bao gồm padding)
        size_t padded_len_to_read = getPaddedSize(len);
        if (data_offset + padded_len_to_read > buffer.size())
        {
            // Nếu không đủ padding, có thể là lỗi encode hoặc cuối buffer
            // Tạm thời chấp nhận đọc hết phần còn lại nếu len hợp lệ
            padded_len_to_read = buffer.size() - data_offset;
            if (padded_len_to_read < len) // Vẫn không đủ chỗ cho dữ liệu gốc
                throw std::out_of_range("Buffer overflow reading bytes data even without padding");
        }

        std::vector<uint8_t> result(buffer.begin() + data_offset, buffer.begin() + data_offset + len);
        return result;
    }

    // Đọc string từ vị trí data_offset và độ dài len đã biết
    std::string readStringFromData(const std::vector<uint8_t> &buffer, size_t data_offset, size_t len)
    {
        if (data_offset + len > buffer.size())
        {
            throw std::out_of_range("Buffer overflow when reading string data");
        }
        size_t padded_len_to_read = getPaddedSize(len);
        if (data_offset + padded_len_to_read > buffer.size())
        {
            padded_len_to_read = buffer.size() - data_offset;
            if (padded_len_to_read < len)
                throw std::out_of_range("Buffer overflow reading string data even without padding");
        }
        return std::string(
            reinterpret_cast<const char *>(buffer.data() + data_offset),
            len);
    }

    // Hàm đọc string động từ vị trí chứa con trỏ offset
     std::string readStringDynamic(const std::vector<uint8_t>& buffer, size_t offset_ptr) {
         if (offset_ptr + 32 > buffer.size()) throw std::out_of_range("readStringDynamic: Offset pointer out of bounds");
         uint64_t data_offset = readUint256AsUint64(buffer, offset_ptr);

         if (data_offset + 32 > buffer.size()) throw std::out_of_range("readStringDynamic: Data offset out of bounds (for length)");
         uint64_t length_u64 = readUint256AsUint64(buffer, data_offset);

         // Kiểm tra tràn số size_t
         if (length_u64 > std::numeric_limits<size_t>::max()) {
             throw std::overflow_error("readStringDynamic: String length exceeds size_t limit");
         }
         size_t length = static_cast<size_t>(length_u64);
         size_t actual_data_offset = data_offset + 32;

         return readStringFromData(buffer, actual_data_offset, length);
     }

     void appendInt256(std::vector<uint8_t>& buffer, int64_t value) {
    size_t initial_size = buffer.size();
    buffer.resize(initial_size + 32); // Thêm 32 byte (chưa khởi tạo)

    uint8_t bytes[32];
    bool is_negative = value < 0;

    // Nếu âm, lấy giá trị tuyệt đối (dưới dạng unsigned) rồi tính two's complement
    uint64_t abs_value;
    if (is_negative) {
        // Cẩn thận với INT64_MIN
        if (value == std::numeric_limits<int64_t>::min()) {
            abs_value = static_cast<uint64_t>(std::numeric_limits<int64_t>::max()) + 1;
        } else {
            abs_value = static_cast<uint64_t>(-value);
        }
    } else {
        abs_value = static_cast<uint64_t>(value);
    }

    // Ghi 8 byte của abs_value vào 8 byte cuối (Big Endian)
    for (int i = 0; i < 8; ++i) {
        bytes[31 - i] = static_cast<uint8_t>((abs_value >> (i * 8)) & 0xFF);
    }

    // Khởi tạo các byte cao
    uint8_t fill_byte = is_negative ? 0xFF : 0x00;
    for(int i = 0; i < 24; ++i) {
        bytes[i] = fill_byte;
    }

    // Nếu âm, thực hiện two's complement trên toàn bộ 32 byte
    if (is_negative) {
        // 1. Invert bits
        for (int i = 0; i < 32; ++i) {
            bytes[i] = ~bytes[i];
        }
        // 2. Add 1
        int carry = 1;
        for (int i = 31; i >= 0 && carry > 0; --i) {
            uint16_t sum = bytes[i] + carry;
            bytes[i] = static_cast<uint8_t>(sum & 0xFF);
            carry = (sum >> 8);
        }
    }

    // Sao chép 32 byte đã chuẩn bị vào buffer chính
    std::memcpy(buffer.data() + initial_size, bytes, 32);
}



}
package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto" // Để tính Keccak256
	"github.com/ethereum/go-ethereum/rlp"    // Để mã hóa RLP
)

// MinimalTxData đại diện cho các trường giao dịch cần thiết để tính hash theo định dạng EIP-155 legacy.
type MinimalTxData struct {
	Nonce    uint64
	GasPrice *big.Int
	GasLimit uint64
	To       *common.Address // Con trỏ để có thể nil (tạo hợp đồng)
	Value    *big.Int
	Data     []byte
	ChainID  *big.Int // ChainID cần thiết cho EIP-155
}

// calculateEIP155LegacyHash tính toán hash để ký cho một giao dịch Legacy (Type 0)
// theo định dạng EIP-155 cụ thể như trong code bạn cung cấp.
func calculateEIP155LegacyHash(data MinimalTxData) (common.Hash, error) {
	if data.ChainID == nil {
		return common.Hash{}, fmt.Errorf("ChainID cannot be nil for EIP-155 Legacy transaction")
	}
	if data.GasPrice == nil {
		return common.Hash{}, fmt.Errorf("GasPrice cannot be nil for Legacy transaction")
	}

	// Tạo slice chứa các interface{} theo đúng thứ tự RLP cho EIP-155 legacy
	// Lưu ý: Các giá trị v, r, s cho mục đích hashing trước khi ký sẽ là 0 hoặc chuỗi byte rỗng.
	// Trong trường hợp này, chúng ta dùng uint(0) như bạn đã thấy trong code Go gốc.
	rlpData := []interface{}{
		data.Nonce,
		data.GasPrice,
		data.GasLimit,
		data.To, // RLP encoder của go-ethereum sẽ xử lý *common.Address nil thành empty bytes
		data.Value,
		data.Data,
		data.ChainID,
		uint(0), // v (cho hashing)
		uint(0), // r (cho hashing)
	}

	// Mã hóa các trường này thành RLP bytes
	encodedBytes, err := rlp.EncodeToBytes(rlpData)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to RLP encode transaction data: %w", err)
	}

	// Tính toán Keccak-256 hash của RLP bytes
	return crypto.Keccak256Hash(encodedBytes), nil
}

func main() {
	fmt.Println("--- Tính Hash giao dịch Legacy (EIP-155) theo định dạng cụ thể ---")

	// Địa chỉ nhận (ví dụ)
	toAddress := common.HexToAddress("0x00000000000000000000000000000000D844bb55")

	// Dữ liệu giao dịch

	// Khắc phục lỗi: sử dụng biến `_` để bỏ qua giá trị boolean trả về thứ hai.
	// Bạn có thể kiểm tra biến 'ok' nếu muốn xử lý lỗi chuyển đổi chuỗi.
	gasPrice, ok := new(big.Int).SetString("100000", 10)
	if !ok {
		fmt.Println("Lỗi: Không thể chuyển đổi '100000' thành big.Int")
		return
	}

	value := new(big.Int).SetInt64(0) // SetInt64 trả về *big.Int đơn lẻ, không cần 'ok'

	chainID, ok := new(big.Int).SetString("991", 10)
	if !ok {
		fmt.Println("Lỗi: Không thể chuyển đổi '991' thành big.Int")
		return
	}

	txData := MinimalTxData{
		Nonce:    0,
		GasPrice: gasPrice,
		GasLimit: 1000000000,
		To:       &toAddress,
		Value:    value,
		Data:     common.Hex2Bytes("dd9e42220000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003086d5de6f7c9c13cc0d959a553cc0e4853ba5faae45a28da9bddc8ef8e104eb5d3dece8dfaa24f11b4243ec27537e318400000000000000000000000000000000"),
		ChainID:  chainID,
	}

	// Tính toán hash
	hash, err := calculateEIP155LegacyHash(txData)
	if err != nil {
		fmt.Printf("Lỗi khi tính hash: %v\n", err)
	} else {
		fmt.Printf("Hash theo định dạng yêu cầu: %s\n", hash.Hex())
	}
}

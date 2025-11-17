package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EIP1559TxJSON represents the JSON structure for an EIP-1559 (0x2) transaction
type EIP1559TxJSON struct {
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"` // Hex string or decimal string
	GasLimit          uint64 `json:"gasLimit"`
	Nonce             uint64 `json:"nonce"`
	Data              string `json:"data"`
	ChainID           string `json:"chainId"`           // Hex string or decimal string
	MaxPriorityFeeGas string `json:"maxPriorityFeeGas"` // Hex string or decimal string
	MaxFeePerGas      string `json:"maxFeePerGas"`      // Hex string or decimal string
	PrivateKey        string `json:"privateKey"`        // Private key for signing (for example purposes only, handle securely in production)
}

func main() {
	// Thay thế bằng URL RPC của bạn (ví dụ: Infura, Alchemy, hoặc node Geth cục bộ)
	// Đảm bảo node của bạn hỗ trợ EIP-1559 (London hard fork trở lên)
	client, err := ethclient.Dial("YOUR_ETHEREUM_RPC_URL")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	// Chuỗi JSON đại diện cho giao dịch EIP-1559 (0x2)
	// LƯU Ý: PrivateKey chỉ được đưa vào đây cho mục đích ví dụ.
	// Trong ứng dụng thực tế, KHÔNG BAO GIỜ xử lý private key dưới dạng chuỗi JSON như vậy.
	// Luôn xử lý private key một cách an toàn (ví dụ: từ biến môi trường, KMS, v.v.).
	txJSONString := `{
		"from": "0xYourSenderAddress",
		"to": "0xYourRecipientAddress",
		"value": "100000000000000000",       // 0.1 ETH in Wei
		"gasLimit": 21000,
		"nonce": 0,
		"data": "0x",
		"chainId": "11155111",            // Sepolia Testnet Chain ID
		"maxPriorityFeeGas": "1000000000", // 1 Gwei
		"maxFeePerGas": "50000000000",     // 50 Gwei
		"privateKey": "YOUR_PRIVATE_KEY_HEX_WITHOUT_0X_PREFIX"
	}`

	// 1. Phân tích cú pháp chuỗi JSON thành cấu trúc Go
	var txData EIP1559TxJSON
	if err := json.Unmarshal([]byte(txJSONString), &txData); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Chuyển đổi các giá trị từ string sang *big.Int
	value, ok := new(big.Int).SetString(txData.Value, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid value: %s", txData.Value)
	}

	chainID, ok := new(big.Int).SetString(txData.ChainID, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid chainId: %s", txData.ChainID)
	}

	maxPriorityFeeGas, ok := new(big.Int).SetString(txData.MaxPriorityFeeGas, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid maxPriorityFeeGas: %s", txData.MaxPriorityFeeGas)
	}

	maxFeePerGas, ok := new(big.Int).SetString(txData.MaxFeePerGas, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid maxFeePerGas: %s", txData.MaxFeePerGas)
	}

	// Chuyển đổi địa chỉ và dữ liệu
	fromAddress := common.HexToAddress(txData.From)
	toAddress := common.HexToAddress(txData.To)
	data := common.Hex2Bytes(strings.TrimPrefix(txData.Data, "0x"))

	// 2. Tạo đối tượng giao dịch EIP-1559 (0x2)
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    chainID,
		Nonce:      txData.Nonce,
		GasTipCap:  maxPriorityFeeGas, // maxPriorityFeePerGas
		GasFeeCap:  maxFeePerGas,      // maxFeePerGas
		Gas:        txData.GasLimit,
		To:         &toAddress, // To có thể là nil nếu là tạo hợp đồng
		Value:      value,
		Data:       data,
		AccessList: nil, // EIP-1559 có thể có AccessList, nhưng ví dụ này không sử dụng
	})

	// 3. Ký giao dịch
	privateKey, err := crypto.HexToECDSA(txData.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}
	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	if senderAddress != fromAddress {
		log.Fatalf("Sender address in JSON (%s) does not match address derived from private key (%s)", fromAddress.Hex(), senderAddress.Hex())
	}

	// Ký giao dịch EIP-1559
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), privateKey)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	fmt.Printf("Transaction signed successfully!\n")
	fmt.Printf("Transaction Hash: %s\n", signedTx.Hash().Hex())

	// 4. Gửi giao dịch
	fmt.Printf("Sending transaction...\n")
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatalf("Failed to send transaction: %v", err)
	}

	fmt.Printf("Transaction sent! Tx Hash: %s\n", signedTx.Hash().Hex())
	fmt.Printf("View on Etherscan/Block explorer: https://sepolia.etherscan.io/tx/%s\n", signedTx.Hash().Hex()) // Thay đổi URL nếu dùng mạng khác
}

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

// LegacyTxJSON represents the JSON structure for a Legacy (0x0) transaction
type LegacyTxJSON struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Value      string `json:"value"`    // Hex string or decimal string
	GasPrice   string `json:"gasPrice"` // Hex string or decimal string
	GasLimit   uint64 `json:"gasLimit"`
	Nonce      uint64 `json:"nonce"`
	Data       string `json:"data"`
	ChainID    string `json:"chainId"`    // Hex string or decimal string
	PrivateKey string `json:"privateKey"` // Private key for signing (for example purposes only, handle securely in production)
}

func main() {
	// Thay thế bằng URL RPC của bạn (ví dụ: Infura, Alchemy, hoặc node Geth cục bộ)
	client, err := ethclient.Dial("YOUR_ETHEREUM_RPC_URL")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	// Chuỗi JSON đại diện cho giao dịch Legacy (0x0)
	// LƯU Ý: PrivateKey chỉ được đưa vào đây cho mục đích ví dụ.
	// Trong ứng dụng thực tế, KHÔNG BAO GIỜ xử lý private key dưới dạng chuỗi JSON như vậy.
	// Luôn xử lý private key một cách an toàn (ví dụ: từ biến môi trường, KMS, v.v.).
	txJSONString := `{
		"from": "0xYourSenderAddress",
		"to": "0xYourRecipientAddress",
		"value": "100000000000000000",       // 0.1 ETH in Wei
		"gasPrice": "20000000000",        // 20 Gwei
		"gasLimit": 21000,
		"nonce": 0,
		"data": "0x",
		"chainId": "11155111",            // Sepolia Testnet Chain ID
		"privateKey": "YOUR_PRIVATE_KEY_HEX_WITHOUT_0X_PREFIX"
	}`

	// 1. Phân tích cú pháp chuỗi JSON thành cấu trúc Go
	var txData LegacyTxJSON
	if err := json.Unmarshal([]byte(txJSONString), &txData); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Chuyển đổi các giá trị từ string sang *big.Int
	value, ok := new(big.Int).SetString(txData.Value, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid value: %s", txData.Value)
	}

	gasPrice, ok := new(big.Int).SetString(txData.GasPrice, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid gasPrice: %s", txData.GasPrice)
	}

	chainID, ok := new(big.Int).SetString(txData.ChainID, 10) // Base 10 for decimal string
	if !ok {
		log.Fatalf("Invalid chainId: %s", txData.ChainID)
	}

	// Chuyển đổi địa chỉ và dữ liệu
	fromAddress := common.HexToAddress(txData.From)
	toAddress := common.HexToAddress(txData.To)
	data := common.Hex2Bytes(strings.TrimPrefix(txData.Data, "0x"))

	// 2. Tạo đối tượng giao dịch Legacy (0x0)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    txData.Nonce,
		GasPrice: gasPrice,
		Gas:      txData.GasLimit,
		To:       &toAddress, // To có thể là nil nếu là tạo hợp đồng
		Value:    value,
		Data:     data,
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

	// Ký giao dịch Legacy với EIP155Signer (để bảo vệ replay attack)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
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

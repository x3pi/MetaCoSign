package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// AccessListItem represents an item in the EIP-2930 AccessList
type AccessListItem struct {
	Address     string   `json:"address"`
	StorageKeys []string `json:"storageKeys"`
}

// EIP2930TxJSON represents the JSON structure for an EIP-2930 transaction
type EIP2930TxJSON struct {
	From       string           `json:"from"`
	To         string           `json:"to"`
	Value      string           `json:"value"`    // Hex string or decimal string
	GasPrice   string           `json:"gasPrice"` // Hex string or decimal string
	GasLimit   uint64           `json:"gasLimit"`
	Nonce      uint64           `json:"nonce"`
	Data       string           `json:"data"`
	ChainID    string           `json:"chainId"` // Hex string or decimal string
	AccessList []AccessListItem `json:"accessList"`
	PrivateKey string           `json:"privateKey"` // Private key for signing (for example purposes only, handle securely in production)
}

func main() {
	// Thay thế bằng URL RPC của bạn (ví dụ: Infura, Alchemy, hoặc node Geth cục bộ)
	// Đảm bảo node của bạn hỗ trợ EIP-2930 (Berlin hard fork trở lên)
	// client, err := ethclient.Dial("YOUR_ETHEREUM_RPC_URL")
	// if err != nil {
	// 	log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	// }

	// Chuỗi JSON đại diện cho giao dịch EIP-2930
	// LƯU Ý: PrivateKey chỉ được đưa vào đây cho mục đích ví dụ.
	// Trong ứng dụng thực tế, KHÔNG BAO GIỜ xử lý private key dưới dạng chuỗi JSON như vậy.
	// Luôn xử lý private key một cách an toàn (ví dụ: từ biến môi trường, KMS, v.v.).
	txJSONString := `{
		"from": "0x5AE1e723973577AcaB776ebC4be66231fc57b370",
		"to": "0x5AE1e723973577AcaB776ebC4be66231fc57b370",
		"value": "100000000000000000",    
		"gasPrice": "20000000000",    
		"gasLimit": 21000,
		"nonce": 0,
		"data": "0x",
		"chainId": "11155111",           
		"accessList": [
			{
				"address": "0xYourContractAddress",
				"storageKeys": [
					"0x0000000000000000000000000000000000000000000000000000000000000000",
					"0x0000000000000000000000000000000000000000000000000000000000000001"
				]
			}
		],
		"privateKey": "cee4c644f964bb3ce7a322db844c50708745e1941990574d358af282c25144fc"
	}`

	// 1. Phân tích cú pháp chuỗi JSON thành cấu trúc Go
	var txData EIP2930TxJSON
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

	// Chuyển đổi AccessList từ JSON sang types.AccessList
	var accessList types.AccessList
	for _, item := range txData.AccessList {
		var storageKeys []common.Hash
		for _, key := range item.StorageKeys {
			storageKeys = append(storageKeys, common.HexToHash(key))
		}
		accessList = append(accessList, types.AccessTuple{
			Address:     common.HexToAddress(item.Address),
			StorageKeys: storageKeys,
		})
	}

	// 2. Tạo đối tượng giao dịch EIP-2930
	// EIP-2930 sử dụng types.AccessListTx
	tx := types.NewTx(&types.AccessListTx{
		ChainID:    chainID,
		Nonce:      txData.Nonce,
		GasPrice:   gasPrice,
		Gas:        txData.GasLimit,
		To:         &toAddress, // To có thể là nil nếu là tạo hợp đồng
		Value:      value,
		Data:       data,
		AccessList: accessList,
	})

	// 3. Ký giao dịch
	// Lấy private key từ chuỗi hex
	privateKey, err := crypto.HexToECDSA(txData.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	// Lấy địa chỉ công khai từ private key để xác minh
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}
	senderAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Kiểm tra xem địa chỉ người gửi trong JSON có khớp với địa chỉ từ private key không
	if senderAddress != fromAddress {
		log.Fatalf("Sender address in JSON (%s) does not match address derived from private key (%s)", fromAddress.Hex(), senderAddress.Hex())
	}

	// Ký giao dịch
	signedTx, err := types.SignTx(tx, types.NewEIP2930Signer(chainID), privateKey)
	if err != nil {
		log.Fatalf("Failed to sign transaction: %v", err)
	}

	txJson, err := signedTx.MarshalJSON()
	fmt.Printf("%s\n", err)

	fmt.Printf("%s\n", txJson)

	fmt.Printf("Transaction signed successfully!\n")
	fmt.Printf("Transaction Hash: %s\n", signedTx.Hash().Hex())

	// 4. Gửi giao dịch
	// fmt.Printf("Sending transaction...\n")
	// err = client.SendTransaction(context.Background(), signedTx)
	// if err != nil {
	// 	log.Fatalf("Failed to send transaction: %v", err)
	// }

	// fmt.Printf("Transaction sent! Tx Hash: %s\n", signedTx.Hash().Hex())
	// fmt.Printf("View on Etherscan/Block explorer: https://sepolia.etherscan.io/tx/%s\n", signedTx.Hash().Hex()) // Thay đổi URL nếu dùng mạng khác
}

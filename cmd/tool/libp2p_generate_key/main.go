package main

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func main() {
	// Tạo một private key mới (ví dụ: Ed25519).
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		log.Fatalf("failed to generate key pair: %s", err)
	}

	// Marshal private key sang định dạng protobuf.
	privBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		log.Fatalf("failed to marshal private key: %s", err)
	}

	// Mã hóa protobuf bytes sang base64.
	privBase64 := base64.StdEncoding.EncodeToString(privBytes)

	fmt.Printf("Private Key (Base64 - Protobuf Format): %s\n", privBase64)
	fmt.Println("\n** Hãy sử dụng chuỗi này cho trường 'private_key' trong file cấu hình của bạn. **")
}

package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func main() {
	// Tạo private key Ed25519 mới
	priv, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		log.Fatalf("Lỗi khi tạo private key: %v", err)
	}

	// Marshal private key thành bytes
	privBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		log.Fatalf("Lỗi khi marshal private key: %v", err)
	}

	// Encode key sang base64
	privKeyStr := base64.StdEncoding.EncodeToString(privBytes)

	// Lấy timestamp hiện tại và tạo tên file
	timestamp := time.Now().Format("20060102_150405") // Định dạng: YYYYMMDD_HHMMSS
	fileName := fmt.Sprintf("config_%s.txt", timestamp)

	// Tạo hoặc mở file cấu hình
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Lỗi khi tạo file cấu hình: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Ghi private key vào file
	configLine := fmt.Sprintf("PRIVATE_KEY=%s\n", privKeyStr)
	_, err = writer.WriteString(configLine)
	if err != nil {
		log.Fatalf("Lỗi khi ghi vào file: %v", err)
	}

	writer.Flush() // Đảm bảo dữ liệu được ghi vào file

	fmt.Printf("Đã tạo private key và lưu vào '%s'.\n", fileName)
	fmt.Printf("Private key (Base64): %s\n", privKeyStr)
	fmt.Println("Hãy lưu private key này một cách an toàn!")
}

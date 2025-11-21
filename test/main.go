package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/meta-node-blockchain/meta-node/test/config"
	"github.com/meta-node-blockchain/meta-node/test/contract"
	"github.com/meta-node-blockchain/meta-node/test/listener"
)

// Hàm mới để chạy trong nền, duy trì kết nối
func startKeepAlive(ctx context.Context, client *ethclient.Client) {
	ticker := time.NewTicker(20 * time.Second)
	// Đảm bảo ticker được dừng khi hàm kết thúc để giải phóng tài nguyên
	defer ticker.Stop()
	log.Println("🔧 Bắt đầu goroutine duy trì kết nối (keep-alive)...")
	for {
		select {
		case <-ticker.C: // Mỗi khi ticker tick
			// Gọi một phương thức nhẹ nhàng như ChainID để gửi dữ liệu qua socket
			_, err := client.ChainID(ctx)
			if err != nil {
				// Nếu có lỗi, có thể kết nối đã bị mất
				log.Printf("⚠️ Lỗi trong quá trình keep-alive (ChainID): %v. Đang cố gắng kết nối lại...", err)
			}
		case <-ctx.Done(): // Nếu context nhận được tín hiệu hủy
			log.Println("🛑 Dừng goroutine duy trì kết nối.")
			return // Thoát khỏi vòng lặp và kết thúc goroutine
		}
	}
}
func main() {
	// --- 1. Kết nối đến client Ethereum ---
	envFile := flag.String("envfile", ".env.1", "Path to .env file")
	flag.Parse()
	config.Load(*envFile)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel() // Đảm bảo hàm cancel được gọi khi main thoát
	client, err := ethclient.Dial(config.RpcUrl)
	if err != nil {
		log.Fatalf("Lỗi kết nối đến Ethereum client: %v", err)
	}
	defer client.Close()
	fmt.Println("✅ Đã kết nối đến Ethereum client.")
	// go startKeepAlive(ctx, client)
	// --- 2. Tải tài khoản từ khóa riêng ---
	privateKey, err := crypto.HexToECDSA(config.PrivateKeyHex)
	if err != nil {
		log.Fatalf("Lỗi tải khóa riêng: %v", err)
	}
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Lỗi: không thể chuyển đổi publicKey sang *ecdsa.PublicKey")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Printf("Sử dụng tài khoản: %s\n", fromAddress.Hex())

	// --- 3. Tải hoặc khởi tạo contract ---
	contractAddress := common.HexToAddress(config.ContractAddressHex)
	instanceWS, err := contract.NewFileContract(contractAddress, client)
	if err != nil {
		log.Fatalf("Lỗi khởi tạo contract: %v", err)
	}
	ls := listener.NewEventListener(instanceWS)
	ls.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("--- Nhận được tín hiệu dừng.. ---")
}

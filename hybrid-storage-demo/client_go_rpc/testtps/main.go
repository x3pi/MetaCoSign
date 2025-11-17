package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	// ⚙️ Cấu hình RPC và số lượng request
	rpcURL := "http://192.168.1.234:8545" // Thay bằng RPC bạn muốn test
	numRequests := 120000                 // tổng số request gửi
	concurrency := 10000                  // số request song song tối đa
	method := "eth_blockNumber"           // RPC method bạn muốn test tốc độ

	fmt.Printf("🚀 Testing RPC: %s\n", rpcURL)
	fmt.Printf("🔧 Total requests: %d | concurrency: %d | method: %s\n", numRequests, concurrency, method)

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to RPC: %v", err)
	}
	defer client.Close()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	successCount := 0
	failCount := 0

	startTime := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// 📦 Gửi yêu cầu thực (eth_blockNumber)
			_, err := client.BlockNumber(ctx)
			if err != nil {
				log.Printf("❌ Request %d failed: %v", i, err)
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}

			mu.Lock()
			successCount++
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime).Seconds()

	tps := float64(successCount) / elapsed
	fmt.Printf("\n✅ Done in %.2fs\n", elapsed)
	fmt.Printf("📊 Success: %d | Fail: %d\n", successCount, failCount)
	fmt.Printf("⚡ TPS: %.2f req/s\n", tps)
}

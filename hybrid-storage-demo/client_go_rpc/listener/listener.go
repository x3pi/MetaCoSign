package listener

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/config"
	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/contract"
	processor "github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/proccessor"
	"github.com/meta-node-blockchain/meta-node/pkg/loggerfile"
	"github.com/quic-go/quic-go"
)

type EventListener struct {
	fileContract *contract.FileContract
}

func NewEventListener(fileContract *contract.FileContract) *EventListener {
	return &EventListener{
		fileContract: fileContract,
	}

}
func (e *EventListener) Start() {
	go e.ListenForAdmin1Events() // Tên cũ của bạn
}
func (e *EventListener) subscribeWithRetry(logs chan<- *contract.FileContractFileActivated) event.Subscription {
	for {
		sub, err := e.fileContract.WatchFileActivated(&bind.WatchOpts{}, logs)
		if err != nil {
			log.Printf("Lỗi đăng ký sự kiện: %v. Sẽ thử lại sau %v.", err, 1*time.Second)
			time.Sleep(1 * time.Second)
			continue // Thử lại vòng lặp
		}
		log.Println(">>> Đăng ký sự kiện thành công! Đang chờ thông báo...")
		return sub // Trả về subscription thành công
	}
}
func (e *EventListener) ListenForAdmin1Events() {
	logs := make(chan *contract.FileContractFileActivated)
	for {
		sub := e.subscribeWithRetry(logs)
	eventLoop:
		for {
			select {
			case err := <-sub.Err():
				if err != nil {
					log.Printf("Lỗi trong subscription: %v. Đang cố gắng đăng ký lại...", err)
					sub.Unsubscribe()
				}
				break eventLoop // Thoát vòng lặp nếu có lỗi
			case eventLog := <-logs:
				if eventLog.User != common.HexToAddress(config.Address) {
					continue
				}
				log.Printf("))))___🎉🎉🎉🎉🎉🎉🎉🎉🎉Nhận được sự kiện FileActivated: FileHash=%x 🎉🎉🎉🎉🎉🎉", eventLog.FileKey)
				//🚀 Bắt đầu gửi giao dịch UploadChunk
				startChunk := time.Now()
				log.Printf("🚀 Download lúc: %v", startChunk.Format("15:04:05.000"))
				downloadFile(e.fileContract, eventLog.FileKey)
				sentTime := time.Now()
				log.Printf("📤 dowloadChunk gửi xong lúc: %v (mất %s để gửi)", sentTime.Format("15:04:05.000"), sentTime.Sub(startChunk))

			}
		}
	}

}

func downloadFile(instance *contract.FileContract, fileKey [32]byte) {
	fileKeyHex := hex.EncodeToString(fileKey[:])
	fmt.Printf("\nBắt đầu quá trình tải tệp với FileKey: %s\n", fileKeyHex)

	// --- Bước 1: Lấy thông tin tệp từ Blockchain ---
	fmt.Println("\nBước 1: Đang lấy thông tin tệp (GetFileInfo)...")
	fileInfo, err := instance.GetFileInfo(&bind.CallOpts{}, fileKey)
	if err != nil {
		log.Fatalf("Lỗi lấy thông tin tệp: %v", err)
	}
	if fileInfo.ContentLen == 0 {
		log.Fatalf("Không tìm thấy thông tin cho FileKey này hoặc tệp không có nội dung.")
	}

	fmt.Printf("--- Thông tin tệp từ Blockchain ---\n  Tên: %s\n  Kích thước: %d bytes\n  Hash (SHA256): %x\n  Tổng số chunk: %d\n",
		fileInfo.Name, fileInfo.ContentLen, fileInfo.MerkleRoot, fileInfo.TotalChunks)
	fmt.Println("\nBước 2: Đang tải xuống các chunk từ Rust servers...")
	var wg sync.WaitGroup
	var mu sync.Mutex
	downloadedChunks := make(map[uint64][]byte, fileInfo.TotalChunks)
	privateKeyHex := config.PrivateKeyHex
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Lỗi convert private key: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(config.ChainId))
	if err != nil {
		log.Fatalf("Failed to create transactor: %v", err)
	}
	client, err := ethclient.Dial(config.RpcUrl)
	if err != nil {
		log.Fatalf("Lỗi kết nối đến Ethereum client: %v", err)
	}
	requiredPayment, err := instance.CalculatePrice(&bind.CallOpts{}, big.NewInt(int64(fileInfo.TotalChunks)))
	if err != nil {
		log.Fatalf("Failed to calculate price: %v", err)
	}
	fmt.Printf("Required payment-dowload: %s wei (%.6f ETH)\n", requiredPayment.String(), float64(requiredPayment.Int64())/1e18)
	defer client.Close()
	auth.GasLimit = uint64(3_000_000)
	auth.GasPrice, _ = client.SuggestGasPrice(context.Background())
	auth.Value = requiredPayment // Gửi kèm thanh toán
	tx, err := instance.PayForDownload(auth, fileKey, big.NewInt(1))
	if err != nil {
		log.Printf("Lỗi thanh toán cho việc tải tệp: %v", err)
		return
	}
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Fatalf("Lỗi chờ giao dịch được khai thác: %v", err)
	}
	if receipt.Status != 1 {
		log.Fatalf("Giao dịch thanh toán thất bại. Trạng thái: %d", receipt.Status)
	}
	var downloadKey [32]byte
	var foundEvent bool
	for _, vLog := range receipt.Logs {
		parsedLog, err := instance.ParseDownloadKeyGenerated(*vLog)
		if err == nil { // Nếu parse thành công, đây chính là sự kiện chúng ta cần
			downloadKey = parsedLog.DownloadKey
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		log.Fatalf("Không thể tìm thấy sự kiện DownloadKeyGenerated trong biên lai giao dịch.")
	}
	downloadKeyHex := hex.EncodeToString(downloadKey[:])
	log.Printf("Thanh toán thành công! DownloadKey nhận được: %s", downloadKeyHex)

	start := time.Now()
	// --- CẬP NHẬT: Khởi tạo Connection Pool ---
	const DOWNLOAD_CONNECTION_POOL_SIZE = 10
	connPool1 := make([]quic.Connection, DOWNLOAD_CONNECTION_POOL_SIZE)
	connPool2 := make([]quic.Connection, DOWNLOAD_CONNECTION_POOL_SIZE)

	fmt.Println("Khởi tạo connection pool cho Server 1...")
	for i := 0; i < DOWNLOAD_CONNECTION_POOL_SIZE; i++ {
		conn, err := processor.CreateQuicConnection(processor.RUST_SERVER_1_ADDR_QUIC)
		if err != nil {
			log.Printf("Lỗi tạo kết nối QUIC %d đến server 1: %v", i, err)
		}
		connPool1[i] = conn
		// Lên lịch đóng kết nối khi hàm downloadFile kết thúc
		if conn != nil {
			defer conn.CloseWithError(0, "Download complete")
		}
	}

	fmt.Println("Khởi tạo connection pool cho Server 2...")
	for i := 0; i < DOWNLOAD_CONNECTION_POOL_SIZE; i++ {
		conn, err := processor.CreateQuicConnection(processor.RUST_SERVER_2_ADDR_QUIC)
		if err != nil {
			log.Printf("Lỗi tạo kết nối QUIC %d đến server 2: %v", i, err)
		}
		connPool2[i] = conn
		// Lên lịch đóng kết nối khi hàm downloadFile kết thúc
		defer conn.CloseWithError(0, "Download complete")
	}
	fmt.Println("Tất cả các connection pool đã sẵn sàng.")
	// --- Hết phần cập nhật pool ---
	upLogger, _ := loggerfile.NewFileLogger("UploadFile.log")
	upLogger.Info("___Khởi tạo connection pool mất: %s", time.Since(start))
	sem := make(chan struct{}, 100)
	for i := uint64(0); i < fileInfo.TotalChunks; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(chunkIndex uint64) {
			defer wg.Done()
			defer func() { <-sem }()
			log.Printf("Chunk %d -k %s đang gửi ....\n", chunkIndex, fileKeyHex)
			sign, err := SignMessage(privateKey, downloadKeyHex)
			if err != nil {
				log.Printf("Lỗi ký chunk %d: %v", chunkIndex, err)
				return // Thoát goroutine này
			}

			var conn quic.Connection
			poolIndex := (int(chunkIndex) / 2) % DOWNLOAD_CONNECTION_POOL_SIZE

			if int(chunkIndex)%2 == 0 {
				conn = connPool1[poolIndex]
			} else {
				conn = connPool2[poolIndex]
			}
			// --- Hết phần cập nhật ---

			chunkData, err := processor.RequestChunkFromRustServerQuic(conn, fileKeyHex, downloadKeyHex, int(chunkIndex), sign)
			if err != nil {
				// Sửa: Không dùng log.Fatalf trong goroutine
				log.Printf("Lỗi tải chunk %d (sử dụng pool index %d): %v", chunkIndex, poolIndex, err)
				return // Thoát goroutine này
			}
			mu.Lock() // Khóa mutex để ghi vào map một cách an toàn
			downloadedChunks[chunkIndex] = chunkData
			mu.Unlock() // Mở khóa.
			log.Printf("✅>>> Đã tải Chunk %d -k %s\n", chunkIndex, fileKeyHex)
		}(i)
	}
	wg.Wait()
	if uint64(len(downloadedChunks)) != fileInfo.TotalChunks {
		log.Fatalf("Tải tệp thất bại. Chỉ tải được %d/%d chunks.", len(downloadedChunks), fileInfo.TotalChunks)
	}
	fmt.Println("\nBước 3: Đang ghép các chunk và xác minh tệp...")
	var downloadedData []byte
	for i := uint64(0); i < fileInfo.TotalChunks; i++ {
		downloadedData = append(downloadedData, downloadedChunks[i]...)
	}
	outputFileName := config.OutputFile
	outputDir := filepath.Dir(outputFileName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Lỗi tạo thư mục đầu ra: %v", err)
		return
	}
	err = os.WriteFile(outputFileName, downloadedData, 0644)
	if err != nil {
		log.Fatalf("Lỗi ghi tệp đã tải xuống: %v", err)
		return
	}
	fmt.Printf("\n🎉 Tệp đã được lưu thành công với tên '%s'\n", outputFileName)
}

// Sign message với private key
func SignMessage(privateKey *ecdsa.PrivateKey, message string) (string, error) {
	// Tạo message hash theo chuẩn Ethereum
	messageBytes := []byte(message)
	hash := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageBytes), message)),
	)
	// Sign
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %v", err)
	}

	return hex.EncodeToString(signature), nil
}

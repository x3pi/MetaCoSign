package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/config"
	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/contract"
	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/listener"
	processor "github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/proccessor"
	"github.com/meta-node-blockchain/meta-node/pkg/loggerfile"
	"github.com/quic-go/quic-go"
)

func calculateNextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	return int(math.Pow(2, math.Ceil(math.Log2(float64(n)))))
}

// buildMerkleTreePadded xây dựng cây tương thích với logic xác minh `index >> level`
// Nó trả về các "lá" đã được đệm và gốc cây
func buildMerkleTreePadded(chunks [][]byte) ([][]byte, []byte) {
	numLeaves := len(chunks)
	if numLeaves == 0 {
		emptyHash := sha256.Sum256(nil)
		return nil, emptyHash[:]
	}
	// 1. Đệm (pad) số lá lên lũy thừa của 2
	nextPowerOfTwo := calculateNextPowerOfTwo(numLeaves)
	leaves := make([][]byte, nextPowerOfTwo)
	// Hash các chunk thật
	for i := 0; i < numLeaves; i++ {
		hash := sha256.Sum256(chunks[i])
		leaves[i] = hash[:]
	}
	// Đệm phần còn lại bằng hash rỗng
	// (Lưu ý: Bạn có thể dùng một giá trị hash cố định, nhưng hash rỗng là phổ biến)
	emptyHash := sha256.Sum256([]byte{})
	for i := numLeaves; i < nextPowerOfTwo; i++ {
		leaves[i] = emptyHash[:]
	}

	// 2. Xây dựng cây
	treeLevel := leaves
	for len(treeLevel) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(treeLevel); i += 2 {
			h := sha256.New()
			h.Write(treeLevel[i])   // Nút trái
			h.Write(treeLevel[i+1]) // Nút phải
			nextLevel = append(nextLevel, h.Sum(nil))
		}
		treeLevel = nextLevel
	}

	return leaves, treeLevel[0] // Trả về các lá đã hash và gốc (root)
}

// getMerkleProofPadded lấy bằng chứng cho một lá trong cây đã đệm
func getMerkleProofPadded(paddedLeaves [][]byte, chunkIndex int) [][32]byte {
	var proof [][32]byte
	treeLevel := paddedLeaves
	currentIndex := chunkIndex

	for len(treeLevel) > 1 {
		var nextLevel [][]byte
		for i := 0; i < len(treeLevel); i += 2 {
			var siblingIndex int
			if currentIndex == i {
				siblingIndex = i + 1 // Sibling bên phải
			} else if currentIndex == i+1 {
				siblingIndex = i // Sibling bên trái
			} else {
				// Không liên quan, chỉ cần hash và đi tiếp
				h := sha256.New()
				h.Write(treeLevel[i])
				h.Write(treeLevel[i+1])
				nextLevel = append(nextLevel, h.Sum(nil))
				continue
			}

			// Đây là cấp độ chứa node của chúng ta, lấy sibling
			var sibling32 [32]byte
			copy(sibling32[:], treeLevel[siblingIndex])
			proof = append(proof, sibling32)

			// Tính hash cha
			h := sha256.New()
			h.Write(treeLevel[i])   // trái
			h.Write(treeLevel[i+1]) // phải
			nextLevel = append(nextLevel, h.Sum(nil))
		}
		treeLevel = nextLevel
		currentIndex = currentIndex / 2 // Di chuyển lên cấp độ cha
	}

	return proof
}

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Đảm bảo hàm cancel được gọi khi main thoát
	client, err := ethclient.Dial(config.RpcUrl)
	if err != nil {
		log.Fatalf("Lỗi kết nối đến Ethereum client: %v", err)
	}
	defer client.Close()
	fmt.Println("✅ Đã kết nối đến Ethereum client.")
	go startKeepAlive(ctx, client)
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
	clientHttp, err := ethclient.Dial(config.HttpUrl)
	if err != nil {
		log.Fatalf("Lỗi kết nối đến Ethereum client: %v", err)
	}
	defer clientHttp.Close()
	instanceHttp, err := contract.NewFileContract(contractAddress, clientHttp)
	if err != nil {
		log.Fatalf("Lỗi khởi tạo contract: %v", err)
	}

	// go func(client *ethclient.Client, privateKey *ecdsa.PrivateKey, instanceHttp *contract.FileContract, fromAddress common.Address) {
	// 	for {
	time.Sleep(1 * time.Second)
	startChunk := time.Now()
	log.Printf("🚀  Bắt đầu UploadChunk lúc: %v", startChunk.Format("15:04:05.000"))
	fileKey, _ := uploadFile(client, privateKey, instanceHttp, fromAddress)
	sentTime := time.Now()
	upLogger, _ := loggerfile.NewFileLogger("UploadFile.log")
	upLogger.Info("📤  UploadChunk gửi xong lúc: %v (mất %s để gửi)",
		sentTime.Format("15:04:05.000"), sentTime.Sub(startChunk))
	fmt.Println("\n🎉🎉🎉 QUÁ TRÌNH TẢI TỆP LÊN HOÀN TẤT! 🎉🎉🎉")
	fmt.Printf("🔑 FileKey để tải xuống: %x\n", fileKey)
	// 	}
	// }(client, privateKey, instanceHttp, fromAddress)
	// --- 4. Thực thi chức năng tải tệp lên ---
	// hexStr := "074904e2b0a96db7661cd52736dcc63772fa8f3ccedc84dd96872b33a4ef6fe0"
	// bytes, err := hex.DecodeString(hexStr)
	// if err != nil {
	// 	panic(err)
	// }
	// // Đảm bảo đúng 32 byte
	// if len(bytes) != 32 {
	// 	panic("Không đúng 32 byte")
	// }
	// // Chuyển thành [32]byte
	// var fileKey [32]byte
	// copy(fileKey[:], bytes)
	// startChunk := time.Now()
	// log.Printf("🚀 Download lúc: %v", startChunk.Format("15:04:05.000"))
	// if err := DownloadFile(client, privateKey, instanceHttp, fileKey); err != nil {
	// 	log.Fatalf("Lỗi trong quá trình tải xuống riêng biệt: %v", err)
	// }
	// sentTime := time.Now()
	// log.Printf("📤 dowloadChunk gửi xong lúc: %v (mất %s để gửi)", sentTime.Format("15:04:05.000"), sentTime.Sub(startChunk))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("--- Nhận được tín hiệu dừng.. ---")
}

func uploadFile(client *ethclient.Client, privateKey *ecdsa.PrivateKey, instance *contract.FileContract, fromAddress common.Address) ([32]byte, string) {
	fileData, err := os.ReadFile(config.FilePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	fileName := filepath.Base(config.FilePath)
	fileExt := filepath.Ext(fileName)
	contentLen := uint64(len(fileData))
	totalChunks := (contentLen + config.ChunkSize - 1) / config.ChunkSize
	fmt.Println("Building PADDED Merkle Tree (compatible with server logic)...")
	var chunks [][]byte
	for i := uint64(0); i < totalChunks; i++ {
		start := i * config.ChunkSize
		end := start + config.ChunkSize
		if end > contentLen {
			end = contentLen
		}
		chunkData := fileData[start:end]
		chunks = append(chunks, chunkData)
		// merkleLeaves = append(merkleLeaves, MerkleContent{data: chunkData}) // BỎ DÒNG NÀY
	}
	// 3. Lấy Merkle Root
	paddedLeaves, merkleRoot := buildMerkleTreePadded(chunks)
	var merkleRoot32 [32]byte
	copy(merkleRoot32[:], merkleRoot)

	fmt.Printf("Uploading File:\n  Name: %s\n  Size: %d bytes\n  Total Chunks: %d\n  Merkle Root: %x\n",
		fileName, contentLen, totalChunks, merkleRoot32)

	info := contract.Info{
		Owner:              fromAddress,
		MerkleRoot:         merkleRoot32,
		ContentLen:         contentLen,
		TotalChunks:        totalChunks,
		ExpireTime:         uint64(time.Now().Add(365 * 24 * time.Hour).Unix()),
		Name:               fileName,
		Ext:                fileExt,
		ContentDisposition: "inline",
		ContentID:          fmt.Sprintf("%x", merkleRoot32),
	}
	log.Printf("File info prepared: %+v\n", info)
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	requiredPayment, err := instance.CalculatePrice(&bind.CallOpts{}, big.NewInt(int64(totalChunks)))
	if err != nil {
		log.Fatalf("Failed to calculate price: %v", err)
	}
	fmt.Printf("Required payment: %s wei (%.6f ETH)\n", requiredPayment.String(), float64(requiredPayment.Int64())/1e18)

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("Failed to create transactor: %v", err)
	}
	auth.GasLimit = uint64(3_000_000_0000000)
	auth.GasPrice, _ = client.SuggestGasPrice(context.Background())
	auth.Value = requiredPayment // Gửi kèm thanh toán

	tx, err := instance.PushFileInfo(auth, info)
	if err != nil {
		log.Fatalf("Failed to call PushFileInfo: %v", err)
	}

	fmt.Println("Waiting for PushFileInfo tx to be mined...")
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Fatalf("Failed to wait for tx: %v", err)
	}
	log.Printf("PushFileInfo tx %v mined in block %d with status %d", tx.Hash(), receipt.BlockNumber.Uint64(), receipt.Status)
	var fileKey [32]byte
	for _, v := range receipt.Logs {
		if parsed, err := instance.ParseFileAdded(*v); err == nil {
			// Chỉ cần truy cập vào trường 'FileKey' của struct trả về là được
			fileKey = parsed.FileKey
			fmt.Printf("FileKey successfully retrieved: %x\n", fileKey)
			break // Thoát khỏi vòng lặp khi đã tìm thấy
		}
	}
	if fileKey == [32]byte{} {
		log.Fatal("FileKey not found in logs")
	}
	var countErr int
	sem := make(chan struct{}, 1000)
	var wg sync.WaitGroup
	auth.Value = big.NewInt(0) // Chỉ cần thanh toán một lần ban đầu
	fileTimeLogger, _ := loggerfile.NewFileLogger("fileClientLogger.log")
	fileTimeError, _ := loggerfile.NewFileLogger("fileClientLoggerErr.log")

	for i := uint64(0); i < totalChunks; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i uint64) {
			defer wg.Done()
			defer func() { <-sem }() // Đảm bảo semaphore được giải phóng
			// --- Bước 1: Chuẩn bị dữ liệu (chỉ làm 1 lần) ---
			start := i * config.ChunkSize
			end := start + config.ChunkSize
			if end > contentLen {
				end = contentLen
			}
			chunk := fileData[start:end]

			// Tạo Merkle Proof
			proofBytes := getMerkleProofPadded(paddedLeaves, int(i))
			// --- Bước 2: Logic Thử lại (Retry) ---
			const maxRetries = 3               // Thử lại tối đa 3 lần
			const retryDelay = 1 * time.Second // Chờ 3 giây giữa các lần thử
			var err error
			for attempt := 1; attempt <= maxRetries; attempt++ {
				startChunk := time.Now()
				// Tạm thời tôi vẫn dùng code gốc của bạn:
				fileTimeLogger.Info("🚀 [Chunk %d -k %s] up...", i, maxRetries, hex.EncodeToString(fileKey[:]))
				tx, err = instance.UploadChunk(auth, fileKey, chunk, big.NewInt(int64(i)), proofBytes)
				if err == nil {
					// THÀNH CÔNG
					sentTime := time.Now()
					if tx == nil {
						fileTimeError.Info("📤 [Chunk %d -k %s -v %v] ", i, hex.EncodeToString(fileKey[:]), tx)
					}
					fileTimeLogger.Info("📤 [Chunk %d -k %s] up xong: (mất %s để gửi)",
						i, hex.EncodeToString(fileKey[:]), sentTime.Format("15:04:05.000"), sentTime.Sub(startChunk))
					return // Thoát khỏi goroutine (thành công)
				}
				// THẤT BẠI
				if err != nil && strings.Contains(err.Error(), "to store chunk on disk") {
					return
				}
				log.Printf("⚠️ [Chunk %d, Thử %d/%d] Lỗi UploadChunk: %v", i, attempt, maxRetries, err)
				// Nếu đây chưa phải là lần thử cuối, hãy chờ
				if attempt < maxRetries {
					log.Printf("... [Chunk %d] Sẽ thử lại sau %v...", i, retryDelay)
					time.Sleep(retryDelay)
				}
			}

			// Nếu vòng lặp kết thúc mà không `return` (tức là đã hết số lần thử)
			log.Printf("❌ [Chunk %d] TỪ BỎ sau %d lần thử. Lỗi cuối: %v", i, maxRetries, err)
			// Goroutine sẽ tự động kết thúc, defer wg.Done() và defer <-sem sẽ chạy

		}(i)
	}
	wg.Wait()
	fmt.Printf("✅ File upload completed! với %d lỗi", countErr)
	return fileKey, fileName
}

// DownloadFile là một hàm độc lập để tải tệp bằng fileKey của nó.
func DownloadFile(client *ethclient.Client, privateKey *ecdsa.PrivateKey, instance *contract.FileContract, fileKey [32]byte) error {
	fileKeyHex := hex.EncodeToString(fileKey[:])
	fmt.Printf("\nBắt đầu quá trình tải tệp với FileKey: %s\n", fileKeyHex)

	// --- Bước 1: Lấy thông tin tệp từ Blockchain ---
	fmt.Println("\nBước 1: Đang lấy thông tin tệp (GetFileInfo)...")
	fileInfo, err := instance.GetFileInfo(&bind.CallOpts{}, fileKey)
	if err != nil {
		return fmt.Errorf("lỗi lấy thông tin tệp: %v", err)
	}
	if fileInfo.ContentLen == 0 {
		return fmt.Errorf("không tìm thấy thông tin cho FileKey này hoặc tệp không có nội dung")
	}
	fmt.Printf("--- Thông tin tệp từ Blockchain ---\n  Tên: %s\n  Kích thước: %d bytes\n  Hash (SHA256): %x\n  Tổng số chunk: %d\n",
		fileInfo.Name, fileInfo.ContentLen, fileInfo.MerkleRoot, fileInfo.TotalChunks)

	// --- Bước 2: Thanh toán và lấy DownloadKey ---
	fmt.Println("\nBước 2: Đang thanh toán để tải xuống và lấy DownloadKey...")
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("lỗi lấy chain ID: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("lỗi tạo transactor: %v", err)
	}

	requiredPayment, err := instance.CalculatePrice(&bind.CallOpts{}, big.NewInt(int64(fileInfo.TotalChunks)))
	if err != nil {
		return fmt.Errorf("lỗi tính toán giá: %v", err)
	}
	fmt.Printf("Yêu cầu thanh toán để tải xuống: %s wei (%.6f ETH)\n", requiredPayment.String(), float64(requiredPayment.Int64())/1e18)

	auth.GasLimit = uint64(3_000_000)
	auth.GasPrice, _ = client.SuggestGasPrice(context.Background())
	auth.Value = requiredPayment // Gửi kèm thanh toán

	tx, err := instance.PayForDownload(auth, fileKey, big.NewInt(1))
	if err != nil {
		return fmt.Errorf("lỗi thanh toán cho việc tải tệp: %v", err)
	}
	fmt.Printf("Giao dịch thanh toán đã được gửi: %s. Đang chờ khai thác...\n", tx.Hash().Hex())

	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return fmt.Errorf("lỗi chờ giao dịch được khai thác: %v", err)
	}
	if receipt.Status != 1 {
		return fmt.Errorf("giao dịch thanh toán thất bại. Trạng thái: %d", receipt.Status)
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
		return fmt.Errorf("không thể tìm thấy sự kiện DownloadKeyGenerated trong biên lai giao dịch")
	}

	// hexStr := "850721e32817d473052c1ba7a5e249379dbd3ace4f5ab82fa95373249351dc4c"
	// bytes, err := hex.DecodeString(hexStr)
	// if err != nil {
	// 	panic(err)
	// }
	// // Đảm bảo đúng 32 byte
	// if len(bytes) != 32 {
	// 	panic("Không đúng 32 byte")
	// }
	// // Chuyển thành [32]byte
	// var downloadKey [32]byte
	// copy(downloadKey[:], bytes)

	downloadKeyHex := hex.EncodeToString(downloadKey[:])
	log.Printf("Thanh toán thành công! DownloadKey nhận được: %s", downloadKeyHex)

	// --- Bước 3: Tải xuống các chunk từ Rust servers ---
	fmt.Println("\nBước 3: Đang tải xuống các chunk từ Rust servers...")
	var wg sync.WaitGroup
	var mu sync.Mutex

	downloadedChunks := make(map[uint64][]byte, fileInfo.TotalChunks)
	conn1, err := processor.CreateQuicConnection(processor.RUST_SERVER_1_ADDR_QUIC)
	if err != nil {
		return fmt.Errorf("lỗi tạo kết nối QUIC đến server 1: %v", err)
	}
	conn2, err := processor.CreateQuicConnection(processor.RUST_SERVER_2_ADDR_QUIC)
	if err != nil {
		return fmt.Errorf("lỗi tạo kết nối QUIC đến server 2: %v", err)
	}
	for i := uint64(0); i < fileInfo.TotalChunks; i++ {
		wg.Add(1)
		go func(i uint64) {
			defer wg.Done()
			sign, err := listener.SignMessage(privateKey, downloadKeyHex)
			if err != nil {
				log.Printf("Lỗi ký cho chunk %d: %v", i, err)
				return // ✅ chỉ return trống
			}
			var conn quic.Connection
			if int(i)%2 == 0 {
				// pool = server1Pool
				conn = conn1
			} else {
				// pool = server2Pool
				conn = conn2
			}
			// Mượn 1 kết nối từ pool
			chunkData, err := processor.RequestChunkFromRustServerQuic(conn, fileKeyHex, downloadKeyHex, int(i), sign)
			if err != nil {
				log.Printf("Lỗi tải chunk %d: %v", i, err)
				return // ✅ chỉ return trống
			}
			// pool <- conn
			mu.Lock()
			downloadedChunks[i] = chunkData
			mu.Unlock()

			fmt.Printf("  ✅ Đã tải xong chunk %d.\n", i)
		}(i)
	}
	wg.Wait()

	if uint64(len(downloadedChunks)) != fileInfo.TotalChunks {
		return fmt.Errorf("tải tệp thất bại. Chỉ tải được %d/%d chunks", len(downloadedChunks), fileInfo.TotalChunks)
	}

	// --- Bước 4: Ghép các chunk và lưu tệp ---
	fmt.Println("\nBước 4: Đang ghép các chunk và lưu tệp...")
	var downloadedData []byte
	for i := uint64(0); i < fileInfo.TotalChunks; i++ {
		downloadedData = append(downloadedData, downloadedChunks[i]...)
	}

	outputFileName := config.OutputFile
	outputDir := filepath.Dir(outputFileName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("lỗi tạo thư mục đầu ra '%s': %v", outputDir, err)
	}
	err = os.WriteFile(outputFileName, downloadedData, 0644)
	if err != nil {
		return fmt.Errorf("lỗi ghi tệp đã tải xuống '%s': %v", outputFileName, err)
	}
	fmt.Printf("\n🎉 Tệp đã được lưu thành công với tên '%s'\n", outputFileName)
	return nil
}

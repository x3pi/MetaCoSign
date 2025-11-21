package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Các biến cấu hình sẽ được load từ file .env
var (
	RpcUrl             string
	HttpUrl            string
	PrivateKeyHex      string
	ContractAddressHex string
	FilePath           string
	ChainId            int64
	ChunkSize          uint64
	OutputFile         string
	Address            string
)

// init được thực thi tự động khi package này được import
func Load(envFile string) {
	// Tải file .env.
	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Cảnh báo: Không thể tải file '%s': %v. Sẽ dựa vào biến môi trường hệ thống.", envFile, err)
	}

	// Tải các giá trị
	RpcUrl = getEnv("RPC_URL", "ws://192.168.1.234:8545")
	HttpUrl = getEnv("HTTP_URL", "http://192.168.1.234:8545")
	ContractAddressHex = getEnv("CONTRACT_ADDRESS_HEX", "0xdC7b4fFD274112318083CCB3900d6287776f78b6")
	FilePath = getEnv("FILE_PATH", "./file_to_upload/output.txt")
	PrivateKeyHex = getEnvOrFatal("PRIVATE_KEY_HEX")
	OutputFile = getEnv("OUTPUT_FILE", "downloaded_file.txt")
	Address = getEnvOrFatal("ADDRESS")
	// Chuyển đổi các giá trị số
	var err error
	chainIdStr := getEnv("CHAIN_ID", "991")
	ChainId, err = strconv.ParseInt(chainIdStr, 10, 64)
	if err != nil {
		log.Fatalf("Lỗi: CHAIN_ID không hợp lệ: %v", err)
	}

	// 600 * 1024 = 614400
	chunkSizeStr := getEnv("CHUNK_SIZE", "614400")
	ChunkSize, err = strconv.ParseUint(chunkSizeStr, 10, 64)
	if err != nil {
		log.Fatalf("Lỗi: CHUNK_SIZE không hợp lệ: %v", err)
	}

	log.Printf("✅ Cấu hình đã được tải thành công từ '%s'", envFile)
}

// getEnv lấy biến môi trường, nếu không có thì dùng giá trị mặc định (fallback)
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Printf("Cảnh báo: Biến môi trường '%s' không được đặt. Sử dụng giá trị mặc định.", key)
	return fallback
}

// getEnvOrFatal lấy biến môi trường bắt buộc, nếu không có thì dừng chương trình
func getEnvOrFatal(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("Lỗi: Biến môi trường bắt buộc '%s' không được đặt.", key)
	}
	return value
}

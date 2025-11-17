package processor

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/models"
	"github.com/quic-go/quic-go"
)

var (
	// RUST_SERVER_1_ADDR_QUIC = "192.168.1.233:7081"
	// RUST_SERVER_2_ADDR_QUIC = "192.168.1.233:7082"

	RUST_SERVER_1_ADDR_QUIC = "206.189.152.114:7081"
	RUST_SERVER_2_ADDR_QUIC = "188.166.225.116:7082"
)

// writeFrameWithLength gửi data với 4-byte big-endian length prefix (match tokio_util::LengthDelimitedCodec)
func writeFrameWithLength(stream quic.Stream, data []byte) error {
	// LengthDelimitedCodec expect: [4-byte BE length][data]
	length := uint32(len(data))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)

	// Gửi length prefix
	if _, err := stream.Write(lengthBuf); err != nil {
		return fmt.Errorf("lỗi gửi length: %v", err)
	}

	// Gửi data
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("lỗi gửi data: %v", err)
	}

	return nil
}

// readFrameWithLength đọc data với 4-byte big-endian length prefix (match tokio_util::LengthDelimitedCodec)
func readFrameWithLength(stream quic.Stream) ([]byte, error) {
	// Đọc 4-byte length prefix
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lengthBuf); err != nil {
		return nil, fmt.Errorf("lỗi đọc length: %v", err)
	}

	length := binary.BigEndian.Uint32(lengthBuf)

	// Kiểm tra max frame size (LengthDelimitedCodec default: 8MB)
	if length > 8*1024*1024 {
		return nil, fmt.Errorf("frame quá lớn: %d bytes", length)
	}

	// Đọc data
	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, fmt.Errorf("lỗi đọc data: %v", err)
	}

	return data, nil
}

func CreateQuicConnection(serverAddr string) (quic.Connection, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}

	var conn quic.Connection
	var err error
	const maxRetries = 3
	const retryDelay = 200 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, err = quic.DialAddr(ctx, serverAddr, tlsConf, nil)
		cancel() // Hủy context
		if err == nil {
			log.Printf("✅ Kết nối QUIC thành công đến %s", serverAddr)
			return conn, nil // Thành công, trả về kết nối
		}

		log.Printf("⚠️ Kết nối QUIC đến %s FAILED (Lần thử %d/%d): %v", serverAddr, i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	// Trả về lỗi cuối cùng nếu tất cả retry đều thất bại
	return nil, fmt.Errorf("không thể kết nối QUIC đến %s sau %d lần thử: %v", serverAddr, maxRetries, err)
}

// RequestChunkFromRustServerQuic yêu cầu chunk qua QUIC
func RequestChunkFromRustServerQuic(
	conn quic.Connection,
	fileKey string,
	downloadKey string,
	chunkIndex int, sign string) ([]byte, error) {
	// Mở stream
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("❌ [Chunk %d] Mở stream FAILED: %v", chunkIndex, err)
		return nil, fmt.Errorf("không thể mở stream: %v", err)
	}
	defer stream.Close()

	// Tạo request
	request := models.DownloadChunkRequest{
		Command: "DownloadChunkRequest",
		Payload: models.DownloadChunkPayload{
			FileKey:     fileKey,
			DownloadKey: downloadKey,
			ChunkIndex:  chunkIndex,
			Signature:   sign,
		},
	}
	// Encode JSON và gửi với length prefix
	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Printf("❌ [Chunk %d] JSON Marshal FAILED: %v", chunkIndex, err)
		return nil, fmt.Errorf("lỗi khi encode request: %v", err)
	}

	// log.Printf("📨 [Chunk %d] Đang gửi request (%d bytes JSON + newline)...", chunkIndex, len(jsonData))

	// Thêm \n để Rust server parse JSON
	jsonData = append(jsonData, '\n')

	if err := writeFrameWithLength(stream, jsonData); err != nil {
		log.Printf("❌ [Chunk %d] Gửi frame FAILED: %v", chunkIndex, err)
		return nil, fmt.Errorf("lỗi khi gửi request: %v", err)
	}

	responseData, err := readFrameWithLength(stream)
	if err != nil {
		log.Printf("❌ [Chunk %d] Đọc frame FAILED: %v", chunkIndex, err)
		return nil, fmt.Errorf("lỗi khi đọc response: %v", err)
	}

	var response models.DownloadResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		log.Printf("❌ [Chunk %d] Parse JSON FAILED: %v", chunkIndex, err)
		log.Printf("📄 [Chunk %d] Raw response (first 200 bytes): %s",
			chunkIndex, string(responseData[:min(len(responseData), 200)]))
		return nil, fmt.Errorf("lỗi khi parse response: %v", err)
	}

	if response.Status != "SUCCESS" {
		log.Printf("❌ [Chunk %d] Server báo lỗi: %v", chunkIndex, response.Message)
		return nil, fmt.Errorf("server báo lỗi: %v", response.Message)
	}

	// Decode chunk data
	chunkData, err := base64.StdEncoding.DecodeString(*response.ChunkDataBase64)
	if err != nil {
		log.Printf("⚠️  [Chunk %d] Hex decode failed, trying base64...", chunkIndex)
		// Try base64 decode if hex fails
		chunkData, err = base64.StdEncoding.DecodeString(*response.ChunkDataBase64)
		if err != nil {
			log.Printf("❌ [Chunk %d] Decode FAILED: %v", chunkIndex, err)
			return nil, fmt.Errorf("failed to decode chunk data: %v", err)
		}
	}
	return chunkData, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package quic_network

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/meta-node-blockchain/meta-node/pkg/models/file_model"

	"github.com/quic-go/quic-go"
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
		NextProtos:         []string{"file-storage-v1"}, // ✅ THÊM ALPN
	}

	var conn quic.Connection
	var err error
	const maxRetries = 3
	const retryDelay = 200 * time.Millisecond
	const dialTimeout = 10 * time.Second // Tăng timeout lên 10s

	quicConfig := &quic.Config{
		MaxIdleTimeout:  7 * time.Second, // Timeout nhạy hơn để phát hiện đứt mạng nhanh (7s)
		KeepAlivePeriod: 2 * time.Second, // Liên tục Ping thăm dò mỗi 2s
	}

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		conn, err = quic.DialAddr(ctx, serverAddr, tlsConf, quicConfig)
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
	return nil, fmt.Errorf("không thể kết nối QUIC đến %s sau %d lần thử: %v", serverAddr, maxRetries, err)
}

type StreamContext struct {
	Mu     sync.Mutex
	Stream quic.Stream
}

// SendChunkToRustServerQuic gửi chunk data qua QUIC
func SendChunkToRustServerQuic(streamCtx *StreamContext, fileKey string, chunkIndex int, chunkData []byte, signature string, merkleProofHashes [][32]byte, merkleRoot [32]byte) (string, error) {
	// Sử dụng stream đã mở và lock để đồng bộ
	streamCtx.Mu.Lock()
	defer streamCtx.Mu.Unlock()
	stream := streamCtx.Stream

	const chunkTimeout = 120 * time.Second

	// Convert merkle proof hashes to hex strings
	merkleProofHexStrings := make([]string, len(merkleProofHashes))
	for i, hash := range merkleProofHashes {
		merkleProofHexStrings[i] = hex.EncodeToString(hash[:])
	}

	// Convert merkle root to hex string
	merkleRootHex := hex.EncodeToString(merkleRoot[:])

	// Tạo payload (không chứa Base64)
	payload := file_model.UploadChunkPayload{
		FileKey:           fileKey,
		ChunkIndex:        chunkIndex,
		Signature:         signature,
		MerkleProofHashes: merkleProofHexStrings,
		MerkleRoot:        merkleRootHex,
	}
	command := file_model.Command{Command: "UploadChunk", Payload: payload}

	// Encode JSON và gửi với length prefix
	jsonData, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("lỗi khi encode command: %v", err)
	}

	// Thêm \n để Rust server parse JSON
	jsonData = append(jsonData, '\n')

	// Set deadline cho write
	stream.SetWriteDeadline(time.Now().Add(chunkTimeout / 2))
	if err := writeFrameWithLength(stream, jsonData); err != nil {
		return "", fmt.Errorf("lỗi khi gửi command (Frame 1): %v", err)
	}

	// Gửi Frame 2: Raw Binary Data
	if err := writeFrameWithLength(stream, chunkData); err != nil {
		return "", fmt.Errorf("lỗi khi gửi binary data (Frame 2): %v", err)
	}

	// Set deadline cho read
	stream.SetReadDeadline(time.Now().Add(chunkTimeout / 2))
	responseData, err := readFrameWithLength(stream)
	if err != nil {
		return "", fmt.Errorf("lỗi khi đọc phản hồi (có thể timeout): %v", err)
	}
	var response file_model.GenericResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		return "", fmt.Errorf("lỗi khi parse response: %v", err)
	}

	if response.Status != "SUCCESS" && response.Status != "COMPLETED" {
		return response.Status, fmt.Errorf("server báo lỗi: %s", response.Message)
	}

	return response.Status, nil
}

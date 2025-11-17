package processor

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"

	"github.com/meta-node-blockchain/meta-node/hybrid-storage-demo/client_go_rpc/models"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

const (
	RUST_SERVER_1_ADDR = "192.168.1.233:7081"
	RUST_SERVER_2_ADDR = "192.168.1.233:7082"
	CHUNK_SIZE         = 1024 * 1 // 1KB
)

func getServer(chunkIndex int) string {
	var targetServer string
	if chunkIndex%2 == 0 {
		targetServer = RUST_SERVER_1_ADDR
	} else {
		targetServer = RUST_SERVER_2_ADDR
	}
	return targetServer
}

func sendChunkToRustServer(fileKey string, chunkIndex int, chunkData []byte) error {
	serverAddr := getServer(chunkIndex)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return fmt.Errorf("không thể kết nối đến Rust server %s: %v", serverAddr, err)
	}
	defer conn.Close()

	payload := models.UploadChunkPayload{
		FileKey:         fileKey,
		ChunkIndex:      chunkIndex,
		ChunkDataBase64: base64.StdEncoding.EncodeToString(chunkData),
	}
	command := models.Command{Command: "UploadChunk", Payload: payload}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(command); err != nil {
		return err
	}
	responseStr, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("lỗi khi đọc phản hồi từ Rust server: %v", err)
	}
	var response models.GenericResponse
	json.Unmarshal([]byte(responseStr), &response)
	if response.Status != "SUCCESS" {
		return fmt.Errorf("server báo lỗi: %s", response.Message)
	}
	logger.Error("Gửi chunk %d của file %s đến Rust server %s thành công response %v", chunkIndex, fileKey, serverAddr, response)
	return nil
}

// requestChunkFromRustServer thực hiện yêu cầu một chunk từ server Rust.
func RequestChunkFromRustServer(fileKey string, downloadKey string, chunkIndex int, sign string) ([]byte, error) {
	serverAddr := getServer(chunkIndex)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối đến server %s: %v", serverAddr, err)
	}
	defer conn.Close()
	// 4. Create request
	request := models.DownloadChunkRequest{
		Command: "DownloadChunkRequest",
		Payload: models.DownloadChunkPayload{
			FileKey:     fileKey,
			DownloadKey: downloadKey,
			ChunkIndex:  chunkIndex,
			Signature:   sign,
		},
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(request); err != nil {
		return nil, err
	}

	responseStr, _ := bufio.NewReader(conn).ReadString('\n')
	var response models.DownloadResponse
	json.Unmarshal([]byte(responseStr), &response)

	if response.Status != "SUCCESS" {
		return nil, fmt.Errorf("server báo lỗi khi tải chunk %v", response.Message)
	}
	chunkData, err := hex.DecodeString(*response.ChunkDataBase64)
	if err != nil {
		// Try base64 decode if hex fails
		chunkData, err = base64.StdEncoding.DecodeString(*response.ChunkDataBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode chunk data: %v", err)
		}
	}
	return chunkData, nil
}

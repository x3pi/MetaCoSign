package main

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/quic-go/quic-go"
)

// --- Thay đổi địa chỉ IP của server Rust của bạn ở đây ---
// const RUST_SERVER_ADDR_QUIC = "192.168.1.233:7082" // <--- ĐỊA CHỈ SERVER RUST
const RUST_SERVER_ADDR_QUIC = "206.189.152.114:7081"

// const RUST_SERVER_ADDR_QUIC = "188.166.225.116:7082"

// RUST_SERVER_1_ADDR_QUIC = "206.189.152.114:7081"

// --- CÁC STRUCT CŨ (CHO ListChunks) ---

type ListChunksPayload struct {
	FileKey string `json:"file_key"`
}
type ListChunksRequest struct {
	Command string            `json:"command"`
	Payload ListChunksPayload `json:"payload"`
}
type ListChunksResponse struct {
	Status       string   `json:"status"`
	Message      string   `json:"message"`
	ChunkIndices []uint64 `json:"chunk_indices"`
}

// --- CÁC STRUCT MỚI (CHO API LOG ĐÃ TÁCH) ---

// --- API 1: GetLogList ---
type GetLogListRequest struct {
	Command string      `json:"command"`
	Payload interface{} `json:"payload,omitempty"` // Sẽ gửi nil
}

type LogsListResponse struct {
	Status         string   `json:"status"`
	Message        string   `json:"message"`
	AvailableFiles []string `json:"available_files"`
}

// --- API 2: GetLogContent ---
type GetLogContentPayload struct {
	FileName string `json:"file_name"`
}
type GetLogContentRequest struct {
	Command string               `json:"command"`
	Payload GetLogContentPayload `json:"payload"`
}

type LogFileContent struct {
	FileName string `json:"file_name"`
	Content  string `json:"content"`
}
type LogsContentResponse struct {
	Status     string          `json:"status"`
	Message    string          `json:"message"`
	LogContent *LogFileContent `json:"log_content"`
}

// --- CÁC HÀM HELPER (Giữ nguyên) ---

// writeFrameWithLength gửi data với 4-byte big-endian length prefix
func writeFrameWithLength(stream quic.Stream, data []byte) error {
	length := uint32(len(data))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	if _, err := stream.Write(lengthBuf); err != nil {
		return fmt.Errorf("lỗi gửi length: %v", err)
	}
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("lỗi gửi data: %v", err)
	}
	return nil
}

// readFrameWithLength đọc data với 4-byte big-endian length prefix
func readFrameWithLength(stream quic.Stream) ([]byte, error) {
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lengthBuf); err != nil {
		return nil, fmt.Errorf("lỗi đọc length: %v", err)
	}
	length := binary.BigEndian.Uint32(lengthBuf)
	if length > 8*1024*1024 { // 8MB limit
		return nil, fmt.Errorf("frame quá lớn: %d bytes", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, fmt.Errorf("lỗi đọc data: %v", err)
	}
	return data, nil
}

// CreateQuicConnection
func CreateQuicConnection(serverAddr string) (quic.Connection, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
	}
	var conn quic.Connection
	var err error
	const maxRetries = 3
	const retryDelay = 200 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		conn, err = quic.DialAddr(ctx, serverAddr, tlsConf, nil)
		cancel()
		if err == nil {
			log.Printf("✅ Kết nối QUIC thành công đến %s", serverAddr)
			return conn, nil
		}
		log.Printf("⚠️ Kết nối QUIC đến %s FAILED (Lần thử %d/%d): %v", serverAddr, i+1, maxRetries, err)
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}
	return nil, fmt.Errorf("không thể kết nối QUIC đến %s sau %d lần thử: %v", serverAddr, maxRetries, err)
}

// findMissingChunks tìm các phần tử bị thiếu trong một dãy số
// với giả định mỗi phần tử cách nhau 2 đơn vị.
func findMissingChunks(data []uint64) []uint64 {
	// Slice để lưu các phần tử bị thiếu
	var missingElements []uint64

	// Nếu data có ít hơn 2 phần tử, không thể so sánh
	if len(data) < 2 {
		return missingElements
	}

	// Bắt đầu từ phần tử thứ 2 (index 1) để so sánh với phần tử trước nó
	for i := 1; i < len(data); i++ {
		previous := data[i-1]
		current := data[i]

		// Kiểm tra xem khoảng cách có lớn hơn 2 không
		if current-previous > 2 {
			// Nếu có, bắt đầu một vòng lặp để tìm tất cả các số bị thiếu
			// Bắt đầu từ số (previous + 2)
			expectedNum := previous + 2

			// Tiếp tục thêm các số bị thiếu cho đến khi bằng số 'current'
			for expectedNum < current {
				missingElements = append(missingElements, expectedNum)
				expectedNum += 2 // Tăng lên 2 cho lần tìm kiếm tiếp theo
			}
		}
	}

	return missingElements
}

// Hàm chính TestListChunks
func TestListChunks(conn quic.Connection, fileKey string) error {
	fmt.Printf("--- Testing ListChunks for: %s ---\n", fileKey)
	req := ListChunksRequest{
		Command: "ListChunksRequest",
		Payload: ListChunksPayload{
			FileKey: fileKey,
		},
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("Error marshalling JSON: %v", err)
	}
	jsonData = append(jsonData, '\n')

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("Error opening stream: %v", err)
	}
	defer stream.Close()

	if err := writeFrameWithLength(stream, jsonData); err != nil {
		return fmt.Errorf("Error writing frame: %v", err)
	}
	responseData, err := readFrameWithLength(stream)
	if err != nil {
		return fmt.Errorf("Error reading frame: %v", err)
	}

	var resp ListChunksResponse
	if err := json.Unmarshal(responseData, &resp); err != nil {
		fmt.Println("Error unmarshalling response:", err)
		fmt.Println("Raw response:", string(responseData))
		return err
	}

	// In kết quả gốc
	fmt.Printf("Status:  %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Chunks:  %v\n", resp.ChunkIndices)

	// --- PHẦN KIỂM TRA MỚI ---

	// 1. Kiểm tra độ dài (len) của data
	length := len(resp.ChunkIndices)
	fmt.Printf("Độ dài (len): %d\n", length)

	// 2. Tìm các phần tử bị thiếu
	missing := findMissingChunks(resp.ChunkIndices)

	if len(missing) > 0 {
		// In đậm và rõ ràng nếu có lỗi
		fmt.Printf("\n🔥🔥🔥 PHÁT HIỆN THIẾU %d PHẦN TỬ: %v 🔥🔥🔥\n", len(missing), missing)
	} else {
		fmt.Println("\n✅ KIỂM TRA: Dữ liệu liền mạch, không thiếu phần tử nào.")
	}
	// --- KẾT THÚC PHẦN KIỂM TRA ---

	fmt.Println("---------------------------------")
	return nil
}

// TestGetLogList (Hàm mới - API 1)
func TestGetLogList(conn quic.Connection) ([]string, error) {
	fmt.Println("--- Testing GetLogList (API 1) ---")
	req := GetLogListRequest{
		Command: "GetLogList",
		Payload: nil, // Gửi nil (Option<()>)
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling JSON: %v", err)
	}
	jsonData = append(jsonData, '\n')

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Error opening stream: %v", err)
	}
	defer stream.Close()

	if err := writeFrameWithLength(stream, jsonData); err != nil {
		return nil, fmt.Errorf("Error writing frame: %v", err)
	}
	responseData, err := readFrameWithLength(stream)
	if err != nil {
		return nil, fmt.Errorf("Error reading frame: %v", err)
	}

	var resp LogsListResponse
	if err := json.Unmarshal(responseData, &resp); err != nil {
		fmt.Println("Error unmarshalling response:", err)
		fmt.Println("Raw response:", string(responseData))
		return nil, err
	}

	fmt.Printf("Status:  %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)
	fmt.Printf("Available Files (%d):\n", len(resp.AvailableFiles))
	for _, f := range resp.AvailableFiles {
		fmt.Printf("  - %s\n", f)
	}
	fmt.Println("---------------------------------")

	if resp.Status != "SUCCESS" || len(resp.AvailableFiles) == 0 {
		return nil, fmt.Errorf("không tìm thấy file log nào")
	}
	return resp.AvailableFiles, nil
}
func TestGetLogContent(conn quic.Connection, fileName string) error {
	fmt.Printf("--- Testing GetLogContent (API 2 - file: %s) ---\n", fileName)

	req := GetLogContentRequest{
		Command: "GetLogContent",
		Payload: GetLogContentPayload{
			FileName: fileName,
		},
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("Error marshalling JSON: %v", err)
	}
	jsonData = append(jsonData, '\n')

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("Error opening stream: %v", err)
	}
	defer stream.Close()

	if err := writeFrameWithLength(stream, jsonData); err != nil {
		return fmt.Errorf("Error writing frame: %v", err)
	}
	responseData, err := readFrameWithLength(stream)
	if err != nil {
		return fmt.Errorf("Error reading frame: %v", err)
	}

	var resp LogsContentResponse
	if err := json.Unmarshal(responseData, &resp); err != nil {
		fmt.Println("Error unmarshalling response:", err)
		fmt.Println("Raw response:", string(responseData))
		return err
	}

	fmt.Printf("Status:  %s\n", resp.Status)
	fmt.Printf("Message: %s\n", resp.Message)

	// --- THAY ĐỔI LOGIC: GHI RA FILE ---
	if resp.Status == "SUCCESS" && resp.LogContent != nil {
		// Tạo thư mục nếu chưa có
		logDir := "./downloaded_logs"
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("❌ Không thể tạo thư mục %s: %v\n", logDir, err)
			return err
		}

		// Tạo đường dẫn file
		filePath := filepath.Join(logDir, resp.LogContent.FileName)

		// Ghi nội dung vào file
		err := os.WriteFile(filePath, []byte(resp.LogContent.Content), 0644)
		if err != nil {
			fmt.Printf("❌ Không thể ghi file %s: %v\n", filePath, err)
			return err
		}

		fmt.Printf("✅ Đã lưu log vào file: %s\n", filePath)

	} else {
		fmt.Printf("\nCould not retrieve content for %s.\n", fileName)
	}
	// ------------------------------------
	return nil
}

func main() {
	conn, err := CreateQuicConnection(RUST_SERVER_ADDR_QUIC)
	if err != nil {
		log.Fatalf("Không thể kết nối: %v", err)
	}
	defer conn.CloseWithError(0, "done")
	log.Println("✅ Đã kết nối tới", RUST_SERVER_ADDR_QUIC)

	// --- CHẠY TESTS ---

	// 1. Test ListChunks (bỏ comment nếu cần dùng)
	// fileKey := "cf76ab4d1ac1e8ea9c5664a87d9c7804f12f2180f8374c880220042a0b427911" // <--- THAY FILE KEY Ở ĐÂY
	// if err := TestListChunks(conn, fileKey); err != nil {
	// 	log.Printf("Lỗi khi test ListChunks: %v", err)
	// }

	// 2. Test API Log 1: Lấy danh sách tất cả các file log
	availableFiles, err := TestGetLogList(conn)
	if err != nil {
		log.Printf("Lỗi khi test GetLogList: %v", err)
		// Dừng chương trình nếu không thể lấy danh sách file
		return
	}

	// 3. Test API Log 2: Lặp qua từng file và tải nội dung
	if len(availableFiles) > 0 {
		log.Printf("✅ Tìm thấy %d file log. Bắt đầu tải về...", len(availableFiles))
		// Lặp qua tất cả các file đã tìm thấy
		for i, fileName := range availableFiles {
			if i == 10 {
				break
			}
			if err := TestGetLogContent(conn, fileName); err != nil {
				// Ghi lại lỗi của file cụ thể và tiếp tục với các file khác
				log.Printf("❌ Lỗi khi xử lý file '%s': %v", fileName, err)
			}
		}
	} else {
		log.Println("ℹ️ Không có file log nào để tải.")
	}

	log.Println("✅ Đã chạy xong test.")
}

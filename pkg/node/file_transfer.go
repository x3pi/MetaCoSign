package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort" // Cần import sort
	"strconv"
	"strings"
	"sync" // Cần import sync
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
	// Needed for UpdateState
)

// FileChunkSize defines the buffer size for reading/writing file data.
const FileChunkSize = 1024 * 64 // 64KB

const (
	FileRequestProtocol  = "/file-request/1.0.0"  // Giữ nguyên cho request ban đầu
	FileTransferProtocol = "/file-transfer/1.0.0" // Dùng để gửi file đơn hoặc file part
)

// ---- State Management cho việc nhận Split Archive ----

// ArchivePartState lưu trữ trạng thái nhận các phần của một split archive.
type ArchivePartState struct {
	BaseArchiveName string          // Tên gốc của archive (vd: data.7z)
	TotalParts      int             // Tổng số part dự kiến
	ReceivedParts   map[string]bool // Map các tên file part đã nhận (vd: "data.7z.001": true)
	TempDir         string          // Thư mục tạm lưu các part
	FirstPartPath   string          // Đường dẫn đầy đủ của part đầu tiên (.001)
	LastUpdate      time.Time       // Thời điểm nhận part cuối cùng (để dọn dẹp state cũ)
}

// receiveStateMutex bảo vệ truy cập vào receivingStates.
var receiveStateMutex sync.Mutex

// receivingStates lưu trạng thái của các split archive đang được nhận.
// Key là BaseArchiveName.
var receivingStates = make(map[string]*ArchivePartState)

// Constants for split archive metadata prefix
const SplitInfoPrefix = "SPLIT_INFO:" // Định dạng: SPLIT_INFO:TotalParts:BaseArchiveName

// ---- END State Management Split Archive ----

// ---- State Management cho File Synchronization ----

// syncStateMutex bảo vệ truy cập vào trạng thái đồng bộ file tổng thể.
var syncStateMutex sync.Mutex

// expectedSyncItems lưu trữ tập hợp các tên mục (file/folder base name)
// được mong đợi trong phiên đồng bộ hiện tại.
var expectedSyncItems = make(map[string]bool)

// receivedSyncItems lưu trữ tập hợp các tên mục đã được nhận và xử lý thành công.
var receivedSyncItems = make(map[string]bool)

// isSyncActive cho biết liệu có một phiên đồng bộ đang hoạt động hay không.
var isSyncActive bool = false

// ResetFileSyncState đặt lại trạng thái đồng bộ.
// Nên được gọi khi phiên đồng bộ bị hủy hoặc hoàn tất (hoặc khi khởi động).
func ResetFileSyncState() {
	syncStateMutex.Lock()
	defer syncStateMutex.Unlock()
	expectedSyncItems = make(map[string]bool)
	receivedSyncItems = make(map[string]bool)
	isSyncActive = false
	logger.Info("File sync state reset.")
}

// markItemReceived đánh dấu một mục đã được nhận và xử lý thành công.
// `receivedItemKey` phải là tên cơ sở đã được chuẩn hóa.
// Trả về true nếu mục được đánh dấu thành công (nằm trong danh sách mong đợi).
func markItemReceived(receivedItemKey string) bool {
	syncStateMutex.Lock()
	defer syncStateMutex.Unlock()

	if !isSyncActive {
		logger.Warn("Attempted to mark item received outside of active sync session:", receivedItemKey)
		return false
	}

	key := normalizeSyncItemKey(receivedItemKey) // Đảm bảo key nhất quán
	if _, expected := expectedSyncItems[key]; expected {
		if !receivedSyncItems[key] { // Chỉ log lần đầu
			receivedSyncItems[key] = true
			logger.Info(fmt.Sprintf("Marked sync item '%s' as received. Progress: %d/%d",
				key, len(receivedSyncItems), len(expectedSyncItems)))
			return true
		} else {
			logger.Debug("Sync item already marked as received:", key)
			return true // Vẫn coi là thành công nếu đã nhận rồi
		}
	} else {
		logger.Warn("Received item '%s' (normalized key: '%s') was not in the expected sync list.", receivedItemKey, key)
		// Quyết định: Có nên thêm vào danh sách đã nhận không? Hiện tại là không.
		return false
	}
}

// checkSyncComplete kiểm tra xem tất cả các mục mong đợi đã được nhận chưa.
// Nếu hoàn thành, nó sẽ gọi storage.UpdateState(2) và reset trạng thái đồng bộ.
func checkSyncComplete() {
	syncStateMutex.Lock()
	defer syncStateMutex.Unlock()
	logger.Info("checkSyncComplete", isSyncActive)

	if !isSyncActive {
		// logger.Debug("CheckSyncComplete called but sync is not active.")
		return // Không có phiên đồng bộ hoạt động
	}
	logger.Info("checkSyncComplete 1")

	if len(expectedSyncItems) == 0 {
		logger.Debug("CheckSyncComplete called with empty expected items list.")
		// storage.UpdateState(2)

		// Có thể reset ở đây nếu muốn
		// isSyncActive = false
		return
	}

	if len(receivedSyncItems) >= len(expectedSyncItems) {
		// Kiểm tra kỹ hơn: Mọi key trong expected đều có trong received
		allMatched := true
		for key := range expectedSyncItems {
			if !receivedSyncItems[key] {
				allMatched = false
				logger.Warn("Sync completion check failed: Missing expected item:", key)
				break // Chỉ cần thiếu 1 là đủ
			}
		}

		if allMatched {
			// Tất cả các mục mong đợi đã được nhận
			logger.Info("✅ All expected sync items received. Updating state to 2.")
			// storage.UpdateState(2) // *** CHỈ GỌI UpdateState TẠI ĐÂY ***

			// Reset trạng thái sau khi hoàn thành
			expectedSyncItems = make(map[string]bool)
			receivedSyncItems = make(map[string]bool)
			isSyncActive = false // Kết thúc phiên đồng bộ
			logger.Info("File sync session completed and state reset.")
		} else {
			logger.Debug("Sync completion check: Received count matches expected, but some keys mismatch.")
		}
	} else {
		// Chưa nhận đủ số lượng
		logger.Debug(fmt.Sprintf("Sync completion check: Still waiting for items (%d/%d received).", len(receivedSyncItems), len(expectedSyncItems)))
	}
}

// normalizeSyncItemKey chuẩn hóa tên mục để sử dụng làm key trong map trạng thái.
// Ví dụ: "folder.7z" -> "folder", "file.txt.gz" -> "file.txt", "data" -> "data"
func normalizeSyncItemKey(rawName string) string {
	name := filepath.Base(rawName) // Lấy tên file/folder cuối cùng
	ext := filepath.Ext(name)
	if ext == ".7z" || ext == ".gz" { // Các đuôi nén phổ biến
		name = strings.TrimSuffix(name, ext)
		// Xử lý trường hợp file có nhiều phần mở rộng như .tar.gz
		if filepath.Ext(name) == ".tar" {
			name = strings.TrimSuffix(name, ".tar")
		}
	}
	// Thêm các chuẩn hóa khác nếu cần (ví dụ: lowercase)
	return name
}

// ---- END State Management File Synchronization ----

// HandleFileReceive xử lý việc nhận file đơn hoặc file part.
func (node *HostNode) HandleFileReceive(stream network.Stream) {
	remotePeer := stream.Conn().RemotePeer()
	logger.Info(fmt.Sprintf("Receiving file stream from %s", remotePeer))

	// Gọi hàm xử lý stream và kiểm tra hoàn thành đồng bộ SAU KHI xử lý xong
	err := node.processIncomingStream(stream)
	if err != nil {
		logger.Error(fmt.Sprintf("Error handling file receive from %s: %v", remotePeer, err))
		// Không reset state đồng bộ khi lỗi nhận 1 file, vì các file khác có thể vẫn đang đến
		_ = stream.Reset() // Reset stream lỗi
	} else {
		logger.Info(fmt.Sprintf("Successfully handled file stream from %s", remotePeer))
		_ = stream.Close() // Đóng stream thành công

		// Kiểm tra xem việc nhận file này có hoàn thành phiên đồng bộ không
		checkSyncComplete()
	}
}

// processIncomingStream thực hiện logic đọc stream, lưu file/part, và đánh dấu hoàn thành mục.
// Nó KHÔNG còn gọi storage.UpdateState(2) trực tiếp nữa.
func (node *HostNode) processIncomingStream(stream network.Stream) error {
	defer func() {
		// Đảm bảo stream được đóng hoặc reset trong mọi trường hợp lỗi của hàm này
		if r := recover(); r != nil {
			logger.Error("Panic recovered in processIncomingStream:", r)
			_ = stream.Reset()
		}
	}()

	reader := bufio.NewReader(stream)
	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Minute))
	defer stream.SetReadDeadline(time.Time{})

	firstLine, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF && firstLine == "" {
			return fmt.Errorf("stream closed before sending any data")
		}
		return fmt.Errorf("lỗi khi đọc dòng đầu tiên từ stream: %w", err)
	}
	firstLine = strings.TrimSpace(firstLine)

	var partFileName string
	var fileSize int64
	var isSplitPart bool
	var totalParts int
	var baseArchiveName string

	if strings.HasPrefix(firstLine, SplitInfoPrefix) {
		isSplitPart = true
		partsInfo := strings.Split(strings.TrimPrefix(firstLine, SplitInfoPrefix), ":")
		if len(partsInfo) != 2 {
			return fmt.Errorf("định dạng split info không hợp lệ: '%s'", firstLine)
		}
		totalParts, err = strconv.Atoi(partsInfo[0])
		if err != nil || totalParts <= 0 {
			return fmt.Errorf("số lượng part không hợp lệ '%s': %w", partsInfo[0], err)
		}
		baseArchiveName = strings.TrimSpace(partsInfo[1])
		if baseArchiveName == "" {
			return fmt.Errorf("tên base archive không hợp lệ trong split info")
		}
		if !strings.HasSuffix(baseArchiveName, ".7z") {
			baseArchiveName += ".7z"
		}

		partFileNameLine, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("lỗi khi đọc tên file part sau split info: %w", err)
		}
		partFileName = strings.TrimSpace(partFileNameLine)

		fileSizeStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("lỗi khi đọc kích thước file part sau split info: %w", err)
		}
		parsedSize, err := strconv.ParseInt(strings.TrimSpace(fileSizeStr), 10, 64)
		if err != nil {
			return fmt.Errorf("lỗi chuyển đổi kích thước file part '%s': %w", fileSizeStr, err)
		}
		fileSize = parsedSize
		logger.Info(fmt.Sprintf("Receiving part '%s' for split archive '%s' (Total: %d parts, Size: %d bytes)", partFileName, baseArchiveName, totalParts, fileSize))

	} else {
		isSplitPart = false
		partFileName = firstLine
		fileSizeStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("lỗi khi đọc kích thước file (đơn): %w", err)
		}
		parsedSize, err := strconv.ParseInt(strings.TrimSpace(fileSizeStr), 10, 64)
		if err != nil {
			return fmt.Errorf("lỗi chuyển đổi kích thước file (đơn) '%s': %w", fileSizeStr, err)
		}
		fileSize = parsedSize
		logger.Info(fmt.Sprintf("Receiving single file '%s' (Size: %d bytes)", partFileName, fileSize))
	}

	if partFileName == "" {
		return fmt.Errorf("tên file nhận được rỗng")
	}
	if fileSize < 0 {
		return fmt.Errorf("kích thước file không hợp lệ: %d", fileSize)
	}
	parentDir := filepath.Dir(node.rootPath)

	// --- Xác định đường dẫn lưu trữ ---
	finalOutputDirBase := filepath.Join(parentDir, "received_files")
	if err := os.MkdirAll(finalOutputDirBase, 0755); err != nil {
		logger.Error("lỗi tạo thư mục output chính '%s': %w", finalOutputDirBase, err)
		panic("lỗi tạo thư mục output chính")
	}
	tempReceiveDir := filepath.Join(filepath.Dir(node.rootPath), "temp_receive")
	err = os.MkdirAll(tempReceiveDir, 0755)
	if err != nil {
		logger.Error("lỗi tạo thư mục tạm '%s': %w", tempReceiveDir, err)
		panic("lỗi tạo thư mục tạm ")
	}

	var targetFilePath string
	var partState *ArchivePartState
	var stateKey string
	var receivedItemKey string // Key để đánh dấu hoàn thành đồng bộ

	if isSplitPart {
		safeBaseName := strings.ReplaceAll(baseArchiveName, string(filepath.Separator), "_")
		safeBaseName = strings.ReplaceAll(safeBaseName, ":", "_")
		archiveTempDir := filepath.Join(tempReceiveDir, safeBaseName+"_parts")
		err = os.MkdirAll(archiveTempDir, 0755)
		if err != nil {
			logger.Error("lỗi tạo thư mục tạm cho archive '%s': %w", baseArchiveName, err)
			panic("lỗi tạo thư mục tạm cho archive")
		}
		safePartFileName := filepath.Base(partFileName)
		targetFilePath = filepath.Join(archiveTempDir, safePartFileName)
		stateKey = baseArchiveName
		receivedItemKey = normalizeSyncItemKey(baseArchiveName) // Key đồng bộ là tên base của archive

		receiveStateMutex.Lock()
		var exists bool
		partState, exists = receivingStates[stateKey]
		if !exists {
			partState = &ArchivePartState{BaseArchiveName: baseArchiveName, TotalParts: totalParts, ReceivedParts: make(map[string]bool), TempDir: archiveTempDir, LastUpdate: time.Now()}
			receivingStates[stateKey] = partState
		} else {
			partState.LastUpdate = time.Now()
			if partState.TotalParts != totalParts {
				partState.TotalParts = totalParts
			}
		}
		receiveStateMutex.Unlock()

		if strings.HasSuffix(safePartFileName, ".001") {
			receiveStateMutex.Lock()
			if partState.FirstPartPath == "" {
				partState.FirstPartPath = targetFilePath
			}
			receiveStateMutex.Unlock()
		}
	} else {
		safeFileName := filepath.Base(partFileName)
		targetFilePath = filepath.Join(tempReceiveDir, safeFileName)
		receivedItemKey = normalizeSyncItemKey(partFileName) // Key đồng bộ là tên file (đã chuẩn hóa)
	}

	// --- Nhận dữ liệu file ---
	outFile, err := os.Create(targetFilePath)
	if err != nil {
		logger.Error("lỗi tạo file đích '%s': %w", targetFilePath, err)
		panic("lỗi tạo file đích")

	}
	_ = stream.SetReadDeadline(time.Now().Add(20 * time.Minute))
	bytesReceived, err := io.CopyN(outFile, reader, fileSize)
	closeErr := outFile.Close()
	if closeErr != nil {
		logger.Error("Error closing output file:", targetFilePath, closeErr)
		panic("Error closing output file")
	}
	if err != nil {
		os.Remove(targetFilePath)
		logger.Error("lỗi khi nhận dữ liệu file '%s' (received %d/%d bytes): %w", partFileName, bytesReceived, fileSize, err)
		panic("lỗi khi nhận dữ liệu file ")

	}
	if bytesReceived != fileSize {
		os.Remove(targetFilePath)
		logger.Error("nhận file '%s' không đủ: nhận %d, dự kiến %d", partFileName, bytesReceived, fileSize)
		panic("lỗi khi nhận dữ liệu file ")

	}
	logger.Info(fmt.Sprintf("File/Part '%s' received successfully (%d bytes) and saved to '%s'.", partFileName, bytesReceived, targetFilePath))

	// --- Xử lý sau khi nhận ---
	processingSuccessful := false // Cờ để biết khi nào nên gọi markItemReceived

	if isSplitPart {
		receiveStateMutex.Lock()
		partState.ReceivedParts[partFileName] = true // Dùng tên gốc để đếm
		partState.LastUpdate = time.Now()
		receivedCount := len(partState.ReceivedParts)
		totalExpected := partState.TotalParts
		firstPartPath := partState.FirstPartPath // Lấy đường dẫn đã lưu
		tempDir := partState.TempDir
		receiveStateMutex.Unlock()

		logger.Debug(fmt.Sprintf("Archive '%s': Received %d/%d parts.", stateKey, receivedCount, totalExpected))

		if receivedCount >= totalExpected {
			logger.Info(fmt.Sprintf("All %d parts received for archive '%s'. Attempting decompression.", totalExpected, stateKey))
			if firstPartPath == "" {
				logger.Error(fmt.Sprintf("All parts received for '%s', but the first part path was not recorded!", stateKey))
				receiveStateMutex.Lock()
				delete(receivingStates, stateKey)
				receiveStateMutex.Unlock()
				os.RemoveAll(tempDir)

				logger.Info("không tìm thấy đường dẫn file part .001 của '%s' để giải nén", stateKey)
				panic("không tìm thấy đường dẫn file part ")

			}
			finalExtractDir := filepath.Dir(node.rootPath)

			logger.Info(fmt.Sprintf("Decompressing '%s' from '%s' into '%s'", stateKey, firstPartPath, finalExtractDir))
			if err := os.MkdirAll(finalExtractDir, 0755); err != nil {
				logger.Error("lỗi tạo thư mục giải nén cuối cùng '%s': %w", finalExtractDir, err)
				panic("lỗi tạo thư mục giải nén cuối cùng")
			}

			err = DecompressFolder(firstPartPath, finalExtractDir)
			if err != nil {
				logger.Error(fmt.Sprintf("Lỗi giải nén archive '%s' từ part '%s': %v", stateKey, firstPartPath, err))
				// Không xóa state/temp khi lỗi giải nén
				panic("Lỗi giải nén archive")

			}

			logger.Info(fmt.Sprintf("✅ Successfully decompressed split archive '%s' to '%s'.", stateKey, finalExtractDir))
			processingSuccessful = true // Chỉ đánh dấu thành công khi giải nén xong archive
			storage.UpdateState(2)
			logger.Debug(fmt.Sprintf("Cleaning up temporary parts directory: %s", tempDir))
			removeErr := os.RemoveAll(tempDir)
			if removeErr != nil {
				logger.Warn("Failed to remove temp parts directory:", tempDir, removeErr)
			}
			receiveStateMutex.Lock()
			delete(receivingStates, stateKey)
			receiveStateMutex.Unlock()
			logger.Debug(fmt.Sprintf("Removed state for completed archive '%s'.", stateKey))

		} else {
			logger.Debug(fmt.Sprintf("Waiting for more parts for archive '%s'...", stateKey))
			// Chưa giải nén xong, chưa thành công hoàn toàn
		}

	} else {
		// File đơn -> Xử lý giải nén hoặc di chuyển
		logger.Info("Processing received single file:", targetFilePath)
		isArchive := strings.HasSuffix(partFileName, ".7z")
		isCompressedFile := strings.HasSuffix(partFileName, ".gz")

		if isArchive || isCompressedFile {
			var baseName string
			if isArchive {
				baseName = strings.TrimSuffix(partFileName, ".7z")
			} else {
				baseName = strings.TrimSuffix(partFileName, ".gz")
			}
			finalExtractPathBase := filepath.Join(finalOutputDirBase, baseName)

			logger.Info(fmt.Sprintf("Decompressing '%s' to '%s'", targetFilePath, finalExtractPathBase))
			var decompErr error
			var finalPath string
			if isArchive {
				finalPath = finalExtractPathBase
				if err := os.MkdirAll(finalPath, 0755); err != nil {
					return fmt.Errorf("lỗi tạo thư mục giải nén '%s': %w", finalPath, err)
				}
				decompErr = DecompressFolder(targetFilePath, finalPath)
			} else {
				finalPath = finalExtractPathBase
				decompErr = DecompressFile(targetFilePath, finalOutputDirBase)
				if decompErr == nil {
					if _, statErr := os.Stat(finalPath); statErr != nil {
						logger.Warn("Decompressed file not found at expected location:", finalPath)
					}
				}
			}

			if decompErr != nil {
				logger.Error(fmt.Sprintf("Lỗi giải nén file '%s': %v", targetFilePath, decompErr))
				return fmt.Errorf("lỗi giải nén '%s': %w", targetFilePath, decompErr)
			}

			logger.Info(fmt.Sprintf("✅ Successfully decompressed '%s' to '%s'.", targetFilePath, finalPath))
			processingSuccessful = true // Đánh dấu thành công sau khi giải nén
			logger.Debug(fmt.Sprintf("Removing original compressed file: %s", targetFilePath))
			removeErr := os.Remove(targetFilePath)
			if removeErr != nil {
				logger.Warn("Failed to remove original compressed file:", targetFilePath, removeErr)
			}

		} else {
			// File thường, di chuyển đến đích
			finalPath := filepath.Join(finalOutputDirBase, partFileName)
			logger.Info(fmt.Sprintf("Received non-archive file. Moving from '%s' to '%s'", targetFilePath, finalPath))
			if _, err := os.Stat(finalPath); err == nil {
				logger.Warn("Destination file already exists, removing before move:", finalPath)
				if errRem := os.Remove(finalPath); errRem != nil {
					logger.Error("Failed to remove existing destination file:", finalPath, errRem)
				}
			}
			if err := os.Rename(targetFilePath, finalPath); err != nil {
				logger.Error(fmt.Sprintf("Lỗi di chuyển file '%s' đến đích cuối cùng '%s': %v", targetFilePath, finalPath, err))
				return fmt.Errorf("lỗi di chuyển file '%s': %w", targetFilePath, err)
			}
			logger.Info(fmt.Sprintf("File '%s' moved to final destination.", finalPath))
			processingSuccessful = true // Đánh dấu thành công sau khi di chuyển
		}
	}

	// --- Đánh dấu mục đã hoàn thành và kiểm tra đồng bộ ---
	if processingSuccessful {
		logger.Debug("Processing successful for item, marking as received:", receivedItemKey)
		markItemReceived(receivedItemKey)
		// Việc kiểm tra hoàn thành tổng thể sẽ được gọi trong HandleFileReceive sau khi hàm này trả về nil
	} else {
		logger.Debug("Processing not marked as successful for item:", receivedItemKey)
	}

	return nil // Trả về nil nếu không có lỗi nghiêm trọng xảy ra trong quá trình xử lý stream này
}

// SendFile gửi một file duy nhất (có thể là file part).
// Nếu splitInfo != nil, đây là một phần của split archive và metadata sẽ được gửi kèm.
func (node *HostNode) SendFile(ctx context.Context, peerIDStr, filePath string, splitInfo *SplitFileInfo) error {
	// Sử dụng helper để phân tích peer ID và lấy AddrInfo nếu có
	peerID, addrInfo, err := parsePeerID(peerIDStr)
	if err != nil {
		return fmt.Errorf("invalid peer identifier '%s': %w", peerIDStr, err)
	}
	// Thêm địa chỉ vào Peerstore nếu có AddrInfo
	if addrInfo != nil {
		node.Host.Peerstore().AddAddrs(peerID, addrInfo.Addrs, peerstore.TempAddrTTL) // Dùng TTL tạm thời
	}

	logger.Info(fmt.Sprintf("Attempting to send file '%s' to peer %s", filePath, peerID))

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("không thể mở file '%s': %w", filePath, err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("lỗi khi lấy thông tin file '%s': %w", filePath, err)
	}

	if fileInfo.IsDir() {
		// SendFile không dùng cho thư mục
		return fmt.Errorf("path '%s' is a directory, use SendFolder instead", filePath)
	}

	fileSize := fileInfo.Size()
	fileName := fileInfo.Name() // Tên file thực tế (vd: data.7z.001)

	// Create a context with timeout for the stream opening and transfer
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Minute) // Tăng timeout cho file lớn/mạng chậm
	defer cancel()

	// Open a new stream to the target peer for file transfer
	var stream network.Stream
	stream, err = node.Host.NewStream(sendCtx, peerID, FileTransferProtocol)
	if err != nil {
		logger.Warn(fmt.Sprintf("Failed to open transfer stream to %s (%v), attempting connect first...", peerID, err))
		// Cần AddrInfo để kết nối, thử lấy lại từ Peerstore hoặc map Peers
		// ** FIX: Kiểm tra addrInfo có nil không TRƯỚC khi truy cập **
		if addrInfo == nil {
			retrievedAddrInfo := node.Host.Peerstore().PeerInfo(peerID) // Đây là peer.AddrInfo (value)
			if len(retrievedAddrInfo.Addrs) > 0 {
				addrInfo = &retrievedAddrInfo // Lấy địa chỉ của value để có pointer *peer.AddrInfo
			}
		}
		// ** FIX: Nếu vẫn không có AddrInfo từ Peerstore, thử map Peers **
		if addrInfo == nil {
			node.reconnectMutex.Lock()
			pInfo, exists := node.Peers[peerID.String()]
			node.reconnectMutex.Unlock()
			if exists {
				addrInfo = &pInfo.Info // pInfo.Info là peer.AddrInfo, lấy địa chỉ
			} else {
				logger.Error("No address info found for peer", peerID, "cannot attempt connection.")
				return fmt.Errorf("cannot open transfer stream and no address info found for peer %s", peerID)
			}
		}

		// Thử kết nối với timeout ngắn hơn
		connectCtx, connectCancel := context.WithTimeout(sendCtx, 20*time.Second)
		logger.Debug("Attempting to connect to peer:", addrInfo)
		// ** FIX: Connect nhận peer.AddrInfo (value), KHÔNG phải pointer **
		// Nếu addrInfo là pointer (*peer.AddrInfo), cần dereference nó
		if addrInfo == nil {
			// Should not happen due to checks above, but safeguard
			connectCancel()
			return fmt.Errorf("internal error: addrInfo is nil before connect call for peer %s", peerID)
		}
		errConnect := node.Host.Connect(connectCtx, *addrInfo) // Dereference pointer
		connectCancel()                                        // Hủy context kết nối ngay sau khi xong

		if errConnect != nil {
			logger.Error("Failed to connect to peer", peerID, "before transfer stream:", errConnect)
			return fmt.Errorf("failed to connect to peer %s before transfer stream: %w", peerID, errConnect)
		}
		logger.Info("Successfully connected to peer", peerID, "retrying NewStream...")
		// Thử mở lại stream sau khi kết nối
		stream, err = node.Host.NewStream(sendCtx, peerID, FileTransferProtocol)
		if err != nil {
			logger.Error("Failed to open transfer stream to", peerID, "even after connecting:", err)
			return fmt.Errorf("không thể mở stream transfer tới %s sau khi kết nối: %w", peerID, err)
		}
	}
	// Đảm bảo stream được đóng hoặc reset khi kết thúc hàm SendFile
	defer func() {
		if err != nil { // Nếu có lỗi xảy ra trong hàm SendFile
			logger.Debug("Resetting stream due to error in SendFile")
			_ = stream.Reset()
		} else {
			logger.Debug("Closing stream after successful SendFile")
			_ = stream.Close()
		}
	}()

	writer := bufio.NewWriter(stream)
	// Set deadline ghi cho toàn bộ quá trình gửi (metadata + content)
	writeDeadline := time.Now().Add(10 * time.Minute) // Deadline khá dài cho việc ghi
	if err = stream.SetWriteDeadline(writeDeadline); err != nil {
		logger.Error("Failed to set write deadline:", err)
		// Có thể không cần trả lỗi ngay, tiếp tục thử ghi
	}
	defer stream.SetWriteDeadline(time.Time{}) // Clear deadline khi kết thúc

	// 1. Gửi metadata
	var meta strings.Builder
	if splitInfo != nil {
		// Đây là một phần của split archive, gửi thông tin split trước
		// Định dạng: SPLIT_INFO:TotalParts:BaseArchiveName
		meta.WriteString(fmt.Sprintf("%s%d:%s\n", SplitInfoPrefix, splitInfo.TotalParts, splitInfo.BaseArchiveName))
		logger.Debug(fmt.Sprintf("Sending split info for %s part %s: Total=%d", splitInfo.BaseArchiveName, fileName, splitInfo.TotalParts))
	}
	// Gửi tên file thực tế và kích thước (luôn luôn gửi)
	meta.WriteString(fmt.Sprintf("%s\n%d\n", fileName, fileSize))

	metadataString := meta.String()
	logger.Debug(fmt.Sprintf("Sending metadata to %s: %q", peerID, metadataString)) // Dùng %q để thấy \n
	_, err = writer.WriteString(metadataString)
	if err != nil {
		// Lỗi ghi metadata -> reset và trả lỗi
		err = fmt.Errorf("lỗi khi gửi metadata cho '%s': %w", fileName, err)
		return err // Trigger defer stream.Reset()
	}

	// Flush metadata trước khi gửi content
	err = writer.Flush()
	if err != nil {
		// Lỗi flush metadata -> reset và trả lỗi
		err = fmt.Errorf("lỗi flush metadata cho '%s': %w", fileName, err)
		return err // Trigger defer stream.Reset()
	}

	// 2. Gửi nội dung file
	logger.Debug(fmt.Sprintf("Sending file content for '%s' (%d bytes) to %s", fileName, fileSize, peerID))
	// Sử dụng buffer để đọc ghi hiệu quả
	buf := make([]byte, FileChunkSize)
	bytesSent, err := io.CopyBuffer(writer, file, buf) // CopyBuffer tự xử lý EOF của file đọc

	// Lỗi xảy ra KHI ĐANG copy hoặc SAU KHI copy xong
	if err != nil {
		// Lỗi trong quá trình copy -> reset và trả lỗi
		err = fmt.Errorf("lỗi khi gửi dữ liệu file '%s' (sent %d/%d): %w", fileName, bytesSent, fileSize, err)
		return err // Trigger defer stream.Reset()
	}

	// Kiểm tra xem có gửi đủ byte không (quan trọng nếu không có lỗi nhưng số byte không khớp)
	if bytesSent != fileSize {
		err = fmt.Errorf("gửi file '%s' không hoàn chỉnh: đã gửi %d bytes, dự kiến %d bytes", fileName, bytesSent, fileSize)
		return err // Trigger defer stream.Reset()
	}

	// Flush dữ liệu cuối cùng còn sót lại trong buffer của writer
	err = writer.Flush()
	if err != nil {
		// Lỗi flush cuối cùng -> reset và trả lỗi
		err = fmt.Errorf("lỗi flush cuối cùng cho file '%s': %w", fileName, err)
		return err // Trigger defer stream.Reset()
	}

	// Nếu đến đây mà không có lỗi nào -> thành công
	logger.Info(fmt.Sprintf("✅ Đã gửi file/part '%s' (%d bytes) tới peer %s thành công", fileName, fileSize, peerID))
	// err sẽ là nil ở đây, defer stream.Close() sẽ được gọi
	return nil
}

// SplitFileInfo chứa thông tin metadata cho một split archive part.
type SplitFileInfo struct {
	BaseArchiveName string // Tên gốc của archive (vd: data.7z)
	TotalParts      int    // Tổng số part
}

// SendFolder nén thư mục (có thể chia nhỏ) và gửi các phần.
// maxPartSizeMB: Kích thước tối đa mỗi part (MB). <= 0 để không chia nhỏ.
func (node *HostNode) SendFolder(ctx context.Context, peerIDStr, folderPath string, maxPartSizeMB int) error {
	startTime := time.Now()
	logger.Info(fmt.Sprintf("Preparing to send folder '%s' to peer %s (Max part size: %d MB)", folderPath, peerIDStr, maxPartSizeMB))

	// Kiểm tra thư mục nguồn
	info, err := os.Stat(folderPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("thư mục nguồn '%s' không tồn tại", folderPath)
		}
		return fmt.Errorf("lỗi kiểm tra thư mục nguồn '%s': %w", folderPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("đường dẫn nguồn '%s' không phải là thư mục", folderPath)
	}

	// Tạo thư mục tạm để lưu file nén/parts
	parentDir := filepath.Dir(folderPath)

	tempDir, err := os.MkdirTemp(parentDir, "sendfolder-compress-")
	if err != nil {
		return fmt.Errorf("lỗi tạo thư mục tạm để nén: %w", err)
	}
	// Đảm bảo dọn dẹp thư mục tạm khi hàm kết thúc (thành công hay lỗi)
	defer func() {
		logger.Debug("Removing compression temp directory:", tempDir)
		removeErr := os.RemoveAll(tempDir)
		if removeErr != nil {
			logger.Warn("Failed to remove compression temp directory:", tempDir, removeErr)
		}
	}()

	// Xác định tên cơ sở cho archive từ tên thư mục
	baseName := filepath.Base(folderPath)
	// Tên này sẽ được CompressFolderAndSplit sử dụng (tự thêm .7z)
	archiveBaseName := baseName

	// Nén thư mục, có thể chia nhỏ
	logger.Info(fmt.Sprintf("Compressing folder '%s' into temp dir '%s'...", folderPath, tempDir))
	compressStart := time.Now()
	// Gọi hàm nén và chia nhỏ
	generatedParts, err := CompressFolderAndSplitWithOptionalSnapshot(ctx, folderPath, tempDir, archiveBaseName, maxPartSizeMB)
	if err != nil {
		// Lỗi ngay từ bước nén -> trả về lỗi
		return fmt.Errorf("lỗi khi nén thư mục '%s': %w", folderPath, err)
	}
	compressDuration := time.Since(compressStart)
	logger.Info(fmt.Sprintf("Compression successful (%s). Found %d file(s)/part(s).", compressDuration, len(generatedParts)))

	if len(generatedParts) == 0 {
		// Nén không lỗi nhưng không có file nào được tạo? Lạ.
		logger.Error("Compression reported success but no archive files found in temp dir:", tempDir)
		return fmt.Errorf("nén thành công nhưng không tìm thấy file archive nào trong %s", tempDir)
	}

	// Sắp xếp các part theo tên để đảm bảo gửi đúng thứ tự (ví dụ: .001, .002)
	sort.Strings(generatedParts)

	totalParts := len(generatedParts)
	var splitInfo *SplitFileInfo = nil
	// isSplitArchive := false // Không cần biến này nữa

	// Xác định xem đây có phải là split archive không để gửi metadata phù hợp
	if totalParts > 1 || (totalParts == 1 && strings.Contains(filepath.Base(generatedParts[0]), ".7z.001")) {
		// isSplitArchive = true
		// Lấy tên base từ part đầu tiên để đảm bảo chính xác (vd: myfolder.7z)
		partBaseName := filepath.Base(generatedParts[0])
		// Lấy phần mở rộng cuối cùng (vd: .001)
		ext := filepath.Ext(partBaseName)
		// Tên base là phần còn lại trước phần mở rộng cuối cùng
		baseArchiveNameForInfo := strings.TrimSuffix(partBaseName, ext)

		splitInfo = &SplitFileInfo{
			BaseArchiveName: baseArchiveNameForInfo,
			TotalParts:      totalParts,
		}
		logger.Info(fmt.Sprintf("Detected split archive '%s' with %d parts.", splitInfo.BaseArchiveName, splitInfo.TotalParts))
	} else if totalParts == 1 {
		logger.Info("Detected single archive file:", generatedParts[0])
	} else {
		// Trường hợp này không nên xảy ra nếu len(generatedParts) > 0
		logger.Warn("Unexpected state: No parts detected after successful compression check.")
	}

	// Gửi từng file part (hoặc file đơn)
	transferStart := time.Now()
	logger.Info(fmt.Sprintf("Starting transfer of %d file(s)/part(s) to %s...", totalParts, peerIDStr))

	var firstPartErr error // Lưu lỗi đầu tiên gặp phải khi gửi part

	for i, partPath := range generatedParts {
		partFileName := filepath.Base(partPath)
		logger.Info(fmt.Sprintf("Sending part %d/%d: '%s'", i+1, totalParts, partFileName))

		// Nếu là split archive, gửi thông tin split kèm theo
		currentSplitInfo := splitInfo // Gửi cho mọi part để receiver dễ xử lý

		// Gọi SendFile để gửi từng part
		err = node.SendFile(ctx, peerIDStr, partPath, currentSplitInfo)
		if err != nil {
			// Lỗi khi gửi một part -> Log lỗi và dừng lại
			firstPartErr = fmt.Errorf("lỗi khi gửi part %d ('%s'): %w", i+1, partFileName, err)
			logger.Error(firstPartErr.Error())
			// Quan trọng: Dừng gửi các part tiếp theo nếu có lỗi
			break
		}
		// Gửi thành công part này
		logger.Info(fmt.Sprintf("Successfully sent part %d/%d: '%s'", i+1, totalParts, partFileName))
	}

	transferDuration := time.Since(transferStart)

	// Kiểm tra xem có lỗi nào xảy ra trong vòng lặp không
	if firstPartErr != nil {
		logger.Error(fmt.Sprintf("Transfer failed for folder '%s' due to error sending a part.", folderPath))
		return firstPartErr // Trả về lỗi đầu tiên gặp phải
	}

	// Nếu không có lỗi -> Hoàn thành
	logger.Info(fmt.Sprintf("✅ Finished sending all %d file(s)/part(s) for folder '%s' (%s total time).", totalParts, folderPath, time.Since(startTime)))
	logger.Info(fmt.Sprintf("   Compression: %s, Transfer: %s", compressDuration, transferDuration))
	return nil // Gửi thành công tất cả các part
}

// Helper function to parse Peer ID or AddrInfo string
// Trả về peer.ID, *peer.AddrInfo (nếu có), và error
func parsePeerID(peerIDStr string) (peer.ID, *peer.AddrInfo, error) {
	// 1. Thử decode trực tiếp thành Peer ID
	peerID, err := peer.Decode(peerIDStr)
	if err == nil {
		return peerID, nil, nil // Thành công, không có AddrInfo từ chuỗi ID
	}
	// Lưu lỗi decode ID để báo cáo nếu parsing AddrInfo cũng lỗi
	decodeErr := err

	// 2. Thử parse thành AddrInfo (bao gồm ID và Addrs)
	addrInfo, errInfo := peer.AddrInfoFromString(peerIDStr)
	if errInfo == nil {
		// Thành công parse AddrInfo
		return addrInfo.ID, addrInfo, nil
	}

	// 3. Cả hai cách đều lỗi
	logger.Error(fmt.Sprintf("Error decoding peer ID '%s': %v", peerIDStr, decodeErr))
	logger.Error(fmt.Sprintf("Error parsing peer AddrInfo '%s': %v", peerIDStr, errInfo))
	// Trả về lỗi cuối cùng (từ AddrInfoFromString) vì nó thường bao hàm cả lỗi ID
	return "", nil, fmt.Errorf("invalid peer identifier '%s': %w", peerIDStr, errInfo)
}

// SendRequest initiates a request to a peer to send a specific file/folder.
func (node *HostNode) SendRequest(ctx context.Context, peerAddr string, name string) error {
	logger.Info(fmt.Sprintf("Sending request for '%s' to peer %s", name, peerAddr))

	// Parse peer identifier
	peerID, addrInfo, err := parsePeerID(peerAddr)
	if err != nil {
		return err // Lỗi đã được log trong parsePeerID
	}
	// Thêm địa chỉ vào peerstore nếu có từ AddrInfo
	if addrInfo != nil {
		node.Host.Peerstore().AddAddrs(peerID, addrInfo.Addrs, peerstore.TempAddrTTL)
	}

	// Ensure the name is clean
	cleanName := strings.TrimSpace(name)
	if cleanName == "" || strings.ContainsAny(cleanName, "\r\n") {
		return fmt.Errorf("invalid name to request: '%s'", name)
	}

	// Create a context with a timeout for the request process
	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second) // Tăng timeout cho request + response
	defer cancel()

	// Open stream for making the request
	var stream network.Stream
	stream, err = node.Host.NewStream(reqCtx, peerID, FileRequestProtocol)
	if err != nil {
		logger.Warn(fmt.Sprintf("Failed to open request stream to %s (%v), attempting connect first...", peerID, err))
		// Cần AddrInfo để kết nối, thử lấy lại nếu chưa có
		// ** FIX: Kiểm tra addrInfo có nil không TRƯỚC khi truy cập **
		if addrInfo == nil {
			retrievedAddrInfo := node.Host.Peerstore().PeerInfo(peerID) // Đây là peer.AddrInfo (value)
			if len(retrievedAddrInfo.Addrs) > 0 {
				addrInfo = &retrievedAddrInfo // Lấy địa chỉ của value để có pointer *peer.AddrInfo
			}
		}
		// ** FIX: Nếu vẫn không có AddrInfo từ Peerstore, thử map Peers **
		if addrInfo == nil {
			node.reconnectMutex.Lock()
			pInfo, exists := node.Peers[peerID.String()]
			node.reconnectMutex.Unlock()
			if exists {
				addrInfo = &pInfo.Info // pInfo.Info là peer.AddrInfo, lấy địa chỉ
			} else {
				logger.Error("No address info found for peer", peerID, "cannot attempt connection.")
				return fmt.Errorf("cannot open request stream and no address info found for peer %s", peerID)
			}
		}

		// Thử kết nối
		connectCtx, connectCancel := context.WithTimeout(reqCtx, 20*time.Second) // Timeout kết nối
		logger.Debug("Attempting to connect to peer for request:", addrInfo)
		// ** FIX: Connect nhận peer.AddrInfo (value), KHÔNG phải pointer **
		if addrInfo == nil {
			// Should not happen due to checks above, but safeguard
			connectCancel()
			return fmt.Errorf("internal error: addrInfo is nil before connect call for peer %s", peerID)
		}
		errConnect := node.Host.Connect(connectCtx, *addrInfo) // Dereference pointer
		connectCancel()

		if errConnect != nil {
			logger.Error("Failed to connect to peer", peerID, "before request stream:", errConnect)
			return fmt.Errorf("failed to connect to peer %s before request stream: %w", peerID, errConnect)
		}
		logger.Info("Successfully connected to peer", peerID, "retrying NewStream for request...")
		// Thử mở lại stream
		stream, err = node.Host.NewStream(reqCtx, peerID, FileRequestProtocol)
		if err != nil {
			logger.Error("Failed to open request stream to", peerID, "even after connecting:", err)
			return fmt.Errorf("lỗi mở stream request tới peer %s sau khi kết nối: %w", peerID, err)
		}
	}
	// Đảm bảo stream được đóng hoặc reset
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.Close()
		}
	}()

	writer := bufio.NewWriter(stream)
	reader := bufio.NewReader(stream)

	// Send the request (name + newline)
	logger.Debug(fmt.Sprintf("Writing request '%s\\n' to stream for %s", cleanName, peerID))
	if err = stream.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		logger.Warn("Failed to set write deadline for request:", err)
	}
	_, err = writer.WriteString(cleanName + "\n")
	if err == nil {
		err = writer.Flush()
	}
	_ = stream.SetWriteDeadline(time.Time{}) // Clear deadline

	if err != nil {
		err = fmt.Errorf("lỗi gửi/flush yêu cầu '%s': %w", cleanName, err)
		return err // Trigger defer stream.Reset()
	}
	logger.Info(fmt.Sprintf("✅ Đã gửi yêu cầu lấy '%s' đến peer %s", cleanName, peerID))

	// Read the response (expect "OK\n" or "ERROR: ...\n")
	if err = stream.SetReadDeadline(time.Now().Add(20 * time.Second)); err != nil { // Timeout chờ phản hồi
		err = fmt.Errorf("lỗi set deadline đọc phản hồi từ %s: %w", peerID, err)
		return err // Trigger defer stream.Reset()
	}

	response, err := reader.ReadString('\n')
	_ = stream.SetReadDeadline(time.Time{}) // Clear deadline

	if err != nil {
		// Kiểm tra xem có phải context timeout không
		if ctxErr := reqCtx.Err(); ctxErr != nil {
			err = fmt.Errorf("timeout đọc phản hồi từ peer %s: %w", peerID, ctxErr)
		} else {
			err = fmt.Errorf("lỗi đọc phản hồi từ peer %s: %w", peerID, err)
		}
		return err // Trigger defer stream.Reset()
	}

	response = strings.TrimSpace(response)
	logger.Info(fmt.Sprintf("Received response from %s: '%s'", peerID, response))

	if response == "OK" {
		fmt.Printf("📩 Peer %s chấp nhận gửi '%s', chờ nhận trên stream mới (%s)...\n", peerID, cleanName, FileTransferProtocol)
		// Thành công, err là nil, defer stream.Close() sẽ chạy
		return nil
	} else {
		// Peer từ chối hoặc báo lỗi
		err = fmt.Errorf("❌ Peer %s từ chối yêu cầu '%s': %s", peerID, cleanName, response)
		return err // Trigger defer stream.Reset()
	}
}

// --- Cleanup function for stale receiving states ---

// CleanupOldStates removes entries from receivingStates that haven't been updated
// for a given duration and cleans up their temporary directories.
// Should be run periodically in a separate goroutine.
func CleanupOldStates(maxIdleTime time.Duration) {
	receiveStateMutex.Lock()
	defer receiveStateMutex.Unlock()

	now := time.Now()
	cleanedCount := 0
	for key, state := range receivingStates {
		if now.Sub(state.LastUpdate) > maxIdleTime {
			logger.Warn(fmt.Sprintf("Cleaning up stale receiving state for '%s' (idle for %v)", key, now.Sub(state.LastUpdate)))
			// Xóa thư mục tạm
			if state.TempDir != "" {
				logger.Debug("Removing stale temp directory:", state.TempDir)
				removeErr := os.RemoveAll(state.TempDir)
				if removeErr != nil {
					logger.Error("Failed to remove stale temp directory:", state.TempDir, removeErr)
				}
			}
			// Xóa state khỏi map
			delete(receivingStates, key)
			cleanedCount++
		}
	}
	if cleanedCount > 0 {
		logger.Info("Finished cleaning up stale receiving states. Removed:", cleanedCount)
	}
}

// --- Make sure to register handlers and potentially start cleanup in node.go ---
/*
In node.go's NewHostNode or similar:

	// Register File Request Handler (assuming fileRequestHandler exists)
	node.Host.SetStreamHandler(FileRequestProtocol, node.fileRequestHandler)
	logger.Info("Set stream handler for file requests:", FileRequestProtocol)

	// Register File Transfer Handler (for receiving files/parts)
	node.Host.SetStreamHandler(FileTransferProtocol, node.HandleFileReceive)
	logger.Info("Set stream handler for file transfers:", FileTransferProtocol)

	// Start periodic cleanup of stale receiving states
	go func() {
		// Use node's context if available for graceful shutdown
		nodeCtx := node.ctx // Assuming node.ctx exists
		if nodeCtx == nil {
			nodeCtx = context.Background() // Fallback
		}
		ticker := time.NewTicker(1 * time.Hour) // Check every hour
		defer ticker.Stop()
		logger.Info("Started periodic cleanup task for stale receiving states.")
		for {
			select {
			case <-ticker.C:
				logger.Debug("Running periodic cleanup for receiving states...")
				CleanupOldStates(24 * time.Hour) // Remove states idle for 24 hours
			case <-nodeCtx.Done():
				logger.Info("Stopping receiving state cleanup task.")
				return // Exit goroutine when node context is done
			}
		}
	}()

*/

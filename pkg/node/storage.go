package node

import (
	"context"
	"errors"
	"fmt"
	"math/rand" // Đảm bảo đã import
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/syndtr/goleveldb/leveldb" // Giả sử dùng leveldb cho backup
)

// --- Hằng số tiền tố cho key block ---
const blockDataKeyPrefix = common.BlockDataTopic

// Lỗi mới để báo hiệu việc tìm kiếm block đang diễn ra ở nền
var ErrBlockFetchInProgress = errors.New("block fetch from peers is in progress")

// createBlockDataKey generates the standardized key for block data.
func createBlockDataKey(blockNumber uint64) string {
	return fmt.Sprintf("%s-%d", blockDataKeyPrefix, blockNumber)
}

// GetBlockStorage kiểm tra block cục bộ (memory, backup).
// Nếu không tìm thấy, nó sẽ khởi chạy một goroutine nền (nếu chưa chạy)
// để yêu cầu block từ các peer "sub" và trả về lỗi ErrBlockFetchInProgress.
// Caller cần gọi lại hàm này sau đó để kiểm tra xem block đã có trong cache chưa.
func (node *HostNode) GetBlockStorage(blockNumber uint64) ([]byte, error) {
	// 1. Tạo key chuẩn hóa
	key := createBlockDataKey(blockNumber)

	// 2. Check In-Memory Store (KeyValueStore - sync.Map)
	value, memOk := node.KeyValueStore.Get(key)
	if memOk {
		// Không cần ép kiểu nữa vì Get đã trả về đúng kiểu []byte
		return value, nil
	}

	// 3. Check Backup Storage (nếu được cấu hình)
	backupStorageInstance, backupExists := node.GetTopicStorage(BackupStorageKey)
	if backupExists {
		type storageGetter interface {
			Get(key []byte) ([]byte, error)
		}
		getter, ok := backupStorageInstance.(storageGetter)
		if !ok {
			logger.Error(fmt.Sprintf("Internal error: backup storage instance ('%s') does not implement Get method for Block %d", BackupStorageKey, blockNumber))
		} else {
			data, err := getter.Get([]byte(key))
			if err == nil {
				logger.Debug(fmt.Sprintf("Retrieved Block %d (key: '%s') from backup storage.", blockNumber, key))
				// Cache vào memory
				node.KeyValueStore.Add(key, data)
				return data, nil // Found in backup
			}
			// Chỉ log lỗi nếu không phải là ErrNotFound
			if !errors.Is(err, leveldb.ErrNotFound) { // Giả sử dùng leveldb
				logger.Error(fmt.Sprintf("Backup storage error retrieving Block %d (key: '%s'): %v", blockNumber, key, err))
				// Có thể xem xét trả về lỗi này thay vì tiếp tục tìm kiếm qua mạng
				// return nil, fmt.Errorf("backup storage access error for Block %d: %w", blockNumber, err)
			}
		}
	}

	// --- 4. Không tìm thấy cục bộ, kiểm tra và khởi chạy tìm kiếm nền ---
	logger.Debug(fmt.Sprintf("Block %d (key: '%s') not found locally. Checking/Initiating peer fetch.", blockNumber, key))

	// Sử dụng node.fetchingBlocks (sync.Map) để theo dõi
	// LoadOrStore trả về (giá trị hiện có, true) nếu key đã tồn tại
	// hoặc (giá trị mới, false) nếu key chưa tồn tại và đã được lưu.
	_, loaded := node.fetchingBlocks.LoadOrStore(blockNumber, true)

	if loaded {
		// Block này ĐÃ đang được tìm kiếm bởi một goroutine khác.
		logger.Debug(fmt.Sprintf("Block %d fetch already in progress.", blockNumber))
		return nil, ErrBlockFetchInProgress // Báo cho caller biết là đang tìm kiếm
	}

	// Nếu 'loaded' là false, nghĩa là chúng ta vừa đánh dấu block này cần tìm kiếm lần đầu tiên.
	// Khởi chạy goroutine nền.
	// logger.Info(fmt.Sprintf("Initiating background fetch for Block %d.", blockNumber))
	go node.fetchBlockFromPeersAsync(blockNumber)

	// Trả về lỗi đặc biệt để báo cho caller biết tìm kiếm nền đã bắt đầu.
	return nil, ErrBlockFetchInProgress
}

// fetchBlockFromPeersAsync chạy ở chế độ nền để yêu cầu block từ các peer "sub".
// Nó sẽ thử nhiều peer và lưu kết quả vào KeyValueStore nếu thành công.
func (node *HostNode) fetchBlockFromPeersAsync(blockNumber uint64) {
	// Đảm bảo xóa cờ "đang tìm kiếm" khi goroutine kết thúc (dù thành công hay thất bại)
	defer node.fetchingBlocks.Delete(blockNumber)

	key := createBlockDataKey(blockNumber)
	ctx := node.ctx // Sử dụng context của node để có thể hủy bỏ nếu node dừng

	// === PHẦN CHỌN PEER NGẪU NHIÊN ===
	// 1. Lấy danh sách tất cả các peer "sub" đang kết nối
	candidatePeers := make([]peer.ID, 0) // Khởi tạo slice rỗng
	node.reconnectMutex.Lock()
	for peerStrID, peerInfo := range node.Peers {
		peerID, err := peer.Decode(peerStrID) // Chuyển string ID thành peer.ID
		if err != nil {
			logger.Warn(fmt.Sprintf("Invalid peer ID string in Peers map: %s", peerStrID))
			continue // Bỏ qua peer ID không hợp lệ
		}
		// Kiểm tra trạng thái kết nối thực tế và loại peer
		if peerInfo.Type == "sub" && node.Host.Network().Connectedness(peerID) == network.Connected {
			candidatePeers = append(candidatePeers, peerID)
		}
		// Không cần kiểm tra peerInfo.Status nữa vì Connectedness là trạng thái thực tế nhất
	}
	node.reconnectMutex.Unlock()

	if len(candidatePeers) == 0 {
		logger.Warn(fmt.Sprintf("No connected 'sub' peers found to fetch Block %d.", blockNumber))
		return // Không có peer nào để thử
	}

	logger.Debug(fmt.Sprintf("Found %d candidate 'sub' peers for Block %d.", len(candidatePeers), blockNumber))

	// 2. Xáo trộn (randomize) danh sách các peer ứng viên
	// Điều này đảm bảo thứ tự thử các peer là ngẫu nhiên mỗi lần chạy
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(candidatePeers), func(i, j int) {
		candidatePeers[i], candidatePeers[j] = candidatePeers[j], candidatePeers[i]
	})
	// ==================================

	// 3. Lặp qua danh sách peer ĐÃ ĐƯỢC XÁO TRỘN và thử yêu cầu block
	var firstError error
	for i, peerID := range candidatePeers { // Duyệt qua danh sách đã random
		// Kiểm tra context trước mỗi lần thử
		if ctx.Err() != nil {
			logger.Warn(fmt.Sprintf("Node context cancelled before trying peer %s for Block %d.", peerID, blockNumber))
			return // Dừng nếu node đã dừng
		}

		logger.Debug(fmt.Sprintf("Attempting to request Block %d from peer %s (%d/%d)...", blockNumber, peerID, i+1, len(candidatePeers)))

		// Tạo context riêng cho mỗi RequestBlock với timeout
		reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // Timeout 60s cho mỗi yêu cầu block
		blockData, err := node.RequestBlock(reqCtx, peerID, blockNumber)
		cancel() // Hủy context của request này ngay sau khi hoàn thành hoặc lỗi

		if err == nil {
			// Thành công!
			// Lưu vào bộ nhớ đệm (KeyValueStore)
			node.KeyValueStore.Add(key, blockData)
			logger.Debug(fmt.Sprintf("Cached Block %d (key: '%s') to memory store.", blockNumber, key))

			// Hoàn thành, không cần thử các peer khác
			return // Kết thúc goroutine thành công
		}

		// Thất bại, log lỗi và thử peer tiếp theo trong danh sách đã xáo trộn
		if firstError == nil {
			firstError = err // Lưu lại lỗi đầu tiên
		}

		// Không cần kiểm tra ctx.Err() lại ở đây vì RequestBlock đã dùng context
		// và sẽ trả lỗi nếu context bị hủy.
	}

	// Nếu đến đây nghĩa là đã thử tất cả các peer mà không thành công
	// logger.Error(fmt.Sprintf("Failed to retrieve Block %d after trying %d peers randomly. Last error: %v", blockNumber, len(candidatePeers), firstError))
}

// GetBlockStorageLocal chỉ tìm kiếm block trong bộ nhớ và backup storage cục bộ.
func (node *HostNode) GetBlockStorageLocal(blockNumber uint64) ([]byte, error) {
	key := createBlockDataKey(blockNumber)

	// Check In-Memory Store
	value, memOk := node.KeyValueStore.Get(key)
	if memOk {
		// Không cần ép kiểu nữa vì Get đã trả về đúng kiểu []byte
		return value, nil
	}

	// Check Backup Storage
	backupStorageInstance, backupExists := node.GetTopicStorage(BackupStorageKey)
	if backupExists {
		type storageGetter interface {
			Get(key []byte) ([]byte, error)
		}
		getter, ok := backupStorageInstance.(storageGetter)
		if !ok {
			logger.Error(fmt.Sprintf("Internal error: backup storage instance unusable (missing Get method) for Block %d", blockNumber))
		} else {
			data, err := getter.Get([]byte(key))
			if err == nil {
				node.KeyValueStore.Add(key, data) // Cache lại khi đọc từ backup
				return data, nil
			}
			if !errors.Is(err, leveldb.ErrNotFound) {
				logger.Error(fmt.Sprintf("Backup storage error retrieving Block %d (key: '%s'): %v", blockNumber, key, err))
			}
		}
	}

	// Không tìm thấy cục bộ
	return nil, fmt.Errorf("block %d not found locally (memory/backup)", blockNumber)
}

// SetStorage stores a key-value pair in the in-memory KeyValueStore.
func (node *HostNode) SetStorage(key string, value []byte) {
	node.KeyValueStore.Add(key, value)
}

// DeleteStorage removes a key from the in-memory KeyValueStore.
func (node *HostNode) DeleteStorage(key string) {
	node.KeyValueStore.Remove(key)
}

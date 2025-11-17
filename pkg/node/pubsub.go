package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

func (node *HostNode) SubscribeToTopic(ctx context.Context, topicName string, addressMaster string, topicStorageMap map[string]storage.Storage, resync bool) error {
	if _, exists := node.Subscribers[topicName]; exists {
		logger.Info(fmt.Sprintf("Already subscribed to topic: %s", topicName))
		return nil
	}
	topic, err := node.PubSub.Join(topicName)
	if err != nil {
		return fmt.Errorf("không thể tham gia topic '%s': %w", topicName, err)
	}
	subscription, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		return fmt.Errorf("không thể tạo subscription cho topic '%s': %w", topicName, err)
	}
	node.Subscribers[topicName] = topic
	fmt.Printf("Đã đăng ký và bắt đầu lắng nghe topic: %s\n", topicName)
	go node.handleSubscriptionMessages(ctx, subscription, topicName, addressMaster, topicStorageMap, resync)
	return nil
}

// SỬA ĐỔI: Tái cấu trúc hoàn toàn để tách biệt việc nhận và xử lý
func (node *HostNode) handleSubscriptionMessages(ctx context.Context, subscription *pubsub.Subscription, topicName string, addressMaster string, topicStorageMap map[string]storage.Storage, resync bool) {
	defer subscription.Cancel()
	logger.Info(fmt.Sprintf("Message handler started for topic: %s", topicName))

	for {
		msg, err := subscription.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Info(fmt.Sprintf("Subscription context cancelled for topic '%s'. Exiting handler.", topicName))
				return
			}
			if errors.Is(err, pubsub.ErrSubscriptionCancelled) || errors.Is(err, pubsub.ErrTopicClosed) {
				logger.Warn(fmt.Sprintf("Subscription closed for topic '%s'. Exiting handler.", topicName))
				return
			}
			logger.Error(fmt.Sprintf("Error receiving message from topic '%s': %v", topicName, err))
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if msg.ReceivedFrom == node.Host.ID() {
			continue
		}

		switch *msg.Topic {
		case common.BlockDataTopic:
			// Nhiệm vụ của goroutine này chỉ là giải mã và đẩy vào hàng đợi xử lý.
			// Đây là một thao tác rất nhanh, không bị block.
			backupDb, err := storage.DeserializeBackupDb(msg.Data)
			if err != nil {
				logger.Error(fmt.Sprintf("CRITICAL: Không thể giải mã BlockData trên topic '%s' (size: %d): %v. Bỏ qua block.", topicName, len(msg.Data), err))
				continue // Bỏ qua tin nhắn bị lỗi và tiếp tục
			}

			// In log ngay lập tức để xác nhận đã nhận
			log.Printf("++++++++++++ new block data:  %v", backupDb.BockNumber)

			select {
			case node.BlockProcessingQueue <- &backupDb:
				// logger.Debug(fmt.Sprintf("Đã đẩy block %d vào hàng đợi xử lý.", backupDb.BockNumber))
			case <-ctx.Done():
				logger.Warn(fmt.Sprintf("Context bị hủy, không thể đẩy block %d vào hàng đợi xử lý.", backupDb.BockNumber))
				return
			default:
				// Hàng đợi xử lý đầy là một dấu hiệu hệ thống xử lý không kịp,
				// nhưng nó không làm nghẽn mạng như trước.
				logger.Warn(fmt.Sprintf("BlockProcessingQueue is full! Dropping block %d. The processing worker is falling behind.", backupDb.BockNumber))
			}

		case common.TransactionsFromSubTopic:
			select {
			case node.TransactionChan <- msg.Data:
			case <-ctx.Done():
				logger.Warn(fmt.Sprintf("Context cancelled, dropping transaction data from topic '%s'", topicName))
				return
			default:
				logger.Warn(fmt.Sprintf("TransactionChan is full, dropping transaction data from topic '%s'", topicName))
			}

		case common.ReadTransactionsFromSubTopic:
			select {
			case node.ReadTransactionChan <- msg.Data:
			case <-ctx.Done():
				logger.Warn(fmt.Sprintf("Context cancelled, dropping read transaction data from topic '%s'", topicName))
				return
			default:
				logger.Warn(fmt.Sprintf("ReadTransactionsFromSubTopic is full, dropping read transaction data from topic '%s'", topicName))
			}

		default:
			logger.Warn(fmt.Sprintf("Received message on unhandled topic: %s", *msg.Topic))
		}
	}
}

// processMessage handles the content of a single received Pub/Sub message.
// LƯU Ý: HÀM NÀY KHÔNG CÒN ĐƯỢC GỌI TRỰC TIẾP TỪ `handleSubscriptionMessages` NỮA.
// LOGIC CỦA NÓ ĐÃ ĐƯỢC CHUYỂN VÀO `TxsProcessor` MỚI.
// Giữ lại nó ở đây có thể hữu ích cho các mục đích khác hoặc có thể xóa đi.
func (node *HostNode) processMessage(topicName string, topicStorageMap map[string]storage.Storage, msg *pubsub.Message, resync bool) error {
	// Assuming storage.DeserializeBackupDb handles the data parsing
	backupDb, err := storage.DeserializeBackupDb(msg.Data)
	if err != nil {
		// Consider logging the raw data size or a snippet for debugging
		return fmt.Errorf("lỗi khi giải mã dữ liệu (size: %d): %w", len(msg.Data), err)
	}

	// Use structured logging if possible
	// logger.Info(fmt.Sprintf("Processing message for block %d on topic '%s'", backupDb.BockNumber, topicName))

	// Update master block number tracking
	log.Printf("++++++++++++ new block data:  %v", backupDb.BockNumber)
	if storage.GetLastBlockNumberFromMaster() < backupDb.BockNumber {
		storage.UpdateLastBlockNumberFromMaster(backupDb.BockNumber)
		// logger.Debug(fmt.Sprintf("Updated last block number from master to: %d", backupDb.BockNumber))
	}

	// Store in KeyValueStore (in-memory sync.Map)
	key := fmt.Sprintf("%s-%d", topicName, backupDb.BockNumber)
	node.SetStorage(key, msg.Data) // Uses the SetStorage method from storage.go
	// logger.Debug(fmt.Sprintf("Stored message in memory with key: %s", key))

	// Persist to backup storage (LevelDB via storage package?)
	backupStorage, ok := topicStorageMap["backup"]
	if !ok {
		return fmt.Errorf("backup storage not found in topicStorageMap")
	}

	if storage.GetUpdateState() < 3 {
		if err := backupStorage.Put([]byte(key), msg.Data); err != nil {
			return fmt.Errorf("lỗi khi lưu dữ liệu vào backup storage với key '%s': %w", key, err)
		}
	}

	// logger.Debug(fmt.Sprintf("Persisted message to backup storage with key: %s", key))

	// Update application state based on received block numbers
	// Ensure GetUpdateState, GetFirstUpdateInRam, etc. are thread-safe if accessed concurrently
	if storage.GetUpdateState() == 2 && storage.GetFirstUpdateInRam() == 0 {
		storage.UpdateFirstUpdateInRam(backupDb.BockNumber)
		// logger.Debug(fmt.Sprintf("Updated first update in RAM to: %d", backupDb.BockNumber))
	}

	if storage.GetFirstUpdateInDb() == 0 {
		storage.UpdateFirstUpdateInDb(backupDb.BockNumber)
		// logger.Debug(fmt.Sprintf("Updated first update in DB to: %d", backupDb.BockNumber))
	}

	// Add any other necessary logic for processing the backupDb data
	// ...

	return nil
}

// HandleSubscriptionRequest is likely intended for dynamically joining topics based on external requests.
// This implementation just joins the topic and stores it.
func (node *HostNode) HandleSubscriptionRequest(ctx context.Context, topicName string) error {
	if _, exists := node.Subscribers[topicName]; exists {
		logger.Info(fmt.Sprintf("Already handling subscription requests for topic: %s", topicName))
		return nil
	}

	topic, err := node.PubSub.Join(topicName)
	if err != nil {
		return fmt.Errorf("không thể tham gia topic '%s' cho request: %w", topicName, err)
	}

	// Store the topic, but don't start a message listener here unless intended.
	// This might just be for allowing publishing to the topic later.
	node.Subscribers[topicName] = topic
	fmt.Printf("Đã thêm topic '%s' vào Subscribers (sẵn sàng để publish)\n", topicName)
	return nil
}

// PublishMessage sends a message to a specific topic with retry logic.
func (node *HostNode) PublishMessage(ctx context.Context, topicName string, msg []byte) error {
	job := PublishJob{
		TopicName: topicName,
		Data:      msg,
		Ctx:       ctx,
	}

	select {
	case node.publishQueue <- job:
		// Đã thêm vào hàng đợi thành công
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before enqueuing message for topic %s", topicName)
	case <-time.After(2 * time.Second): // Timeout 2 giây để tránh block vô hạn
		logger.Warn(fmt.Sprintf("Timeout enqueuing message for topic %s. Publish queue is likely full (size: %d). Backpressure is active.", topicName, len(node.publishQueue)))
		return fmt.Errorf("timeout enqueuing message for topic %s; queue is full", topicName)
	}
}

// publishWorker là goroutine chạy nền để xử lý việc publish message từ hàng đợi.
func (node *HostNode) publishWorker() {
	logger.Info("Starting Pub/Sub publish worker...")
	// Worker này chạy suốt vòng đời của node.
	for job := range node.publishQueue {
		ctx := job.Ctx
		if ctx == nil {
			ctx = node.ctx // Sử dụng context chính của node nếu không có context cụ thể
		}
		// Gọi hàm publish nội bộ (blocking)
		if err := node.publishMessageInternal(ctx, job.TopicName, job.Data); err != nil {
			log.Fatalf("Publish worker failed to send message to topic '%s': %v", job.TopicName, err)
		}
	}
	logger.Info("Pub/Sub publish worker has shut down.")
}

// publishMessageInternal là hàm PublishMessage cũ, giờ là hàm nội bộ.
// Nó chứa logic thực sự để gửi message và retry.
func (node *HostNode) publishMessageInternal(ctx context.Context, topicName string, msg []byte) error {
	topic, exists := node.Subscribers[topicName]
	if !exists {
		return fmt.Errorf("chưa đăng ký hoặc tham gia topic '%s' để publish", topicName)
	}

	if topic == nil {
		delete(node.Subscribers, topicName)
		return fmt.Errorf("tham chiếu topic '%s' không hợp lệ (nil)", topicName)
	}

	const maxRetries = 3
	const initialRetryDelay = 200 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		publishCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := topic.Publish(publishCtx, msg)
		cancel()

		if err == nil {
			return nil // Thành công
		}

		lastErr = err
		logger.Error(fmt.Sprintf("Failed to publish message to topic '%s' (attempt %d/%d): %v",
			topicName, attempt+1, maxRetries, err))

		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled during publish retry: %w", ctx.Err())
		}

		if attempt < maxRetries-1 {
			retryDelay := initialRetryDelay * time.Duration(attempt+1)
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during publish retry delay: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("không thể gửi tin nhắn tới topic '%s' sau %d lần thử: %w",
		topicName, maxRetries, lastErr)
}

// SendDataToSubscriber is just an alias for PublishMessage, assuming topicName acts as the subscriber identifier via the topic.
func (node *HostNode) SendDataToSubscriber(ctx context.Context, topicName string, data []byte) error {
	logger.Debug(fmt.Sprintf("Sending data (%d bytes) via pubsub to topic '%s'", len(data), topicName))
	return node.PublishMessage(ctx, topicName, data)
}

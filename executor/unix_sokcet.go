package executor

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/meta-node-blockchain/meta-node/pkg/blockchain"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

// SocketExecutor quản lý kết nối Unix domain socket từ Go sang Rust
type SocketExecutor struct {
	socketPath     string
	listener       net.Listener
	requestHandler *RequestHandler
	mu             sync.Mutex
	wg             sync.WaitGroup
	quit           chan struct{}
	isRunning      bool
}

// NewSocketExecutor tạo một instance mới của SocketExecutor
func NewSocketExecutor(socketPath string, storageManager *storage.StorageManager, chainState *blockchain.ChainState) *SocketExecutor {
	if socketPath == "" {
		socketPath = "/tmp/rust-go.sock"
	}
	return &SocketExecutor{
		socketPath:     socketPath,
		requestHandler: NewRequestHandler(storageManager, chainState),
		quit:           make(chan struct{}),
		isRunning:      false,
	}
}

// listenAndServe bắt đầu lắng nghe và chấp nhận kết nối
func (se *SocketExecutor) listenAndServe() error {
	// Xóa socket file cũ nếu tồn tại để tránh lỗi "address already in use"
	if _, err := os.Stat(se.socketPath); err == nil {
		if err := os.Remove(se.socketPath); err != nil {
			return fmt.Errorf("không thể xóa socket file cũ: %w", err)
		}
	}
	listener, err := net.Listen("unix", se.socketPath)
	if err != nil {
		return fmt.Errorf("không thể lắng nghe trên socket %s: %w", se.socketPath, err)
	}
	se.listener = listener
	logger.Info("[Go Server] Đang lắng nghe trên %s", se.socketPath)
	// Vòng lặp chấp nhận kết nối
	se.wg.Add(1)
	go func() {
		defer se.wg.Done()
		for {
			// Chấp nhận kết nối mới
			conn, err := se.listener.Accept()
			if err != nil {
				// Kiểm tra nếu lỗi là do listener đã bị đóng (trong hàm Stop)
				select {
				case <-se.quit: // Nếu Stop() đã được gọi, đây là lỗi dự kiến
					logger.Info("[Go Server] Đã dừng chấp nhận kết nối.")
					return
				default:
					logger.Error("[Go Server] Lỗi chấp nhận kết nối: %v", err)
				}
				continue
			}
			logger.Info("[Go Server] Client Rust đã kết nối!")
			// Xử lý mỗi kết nối trong một goroutine riêng
			se.wg.Add(1)
			go se.handleConnection(conn)
		}
	}()

	return nil
}

// handleConnection lắng nghe request từ Rust và gửi lại response
func (se *SocketExecutor) handleConnection(conn net.Conn) {
	defer se.wg.Done()
	defer conn.Close()
	for {
		var wrappedRequest pb.Request
		err := ReadMessage(conn, &wrappedRequest)
		if err != nil {
			if err == io.EOF {
				logger.Info("[Go Server] Rust client đã đóng kết nối.")
			} else {
				logger.Error("[Go Server] Lỗi đọc message: %v", err)
			}
			return
		}

		var wrappedResponse *pb.Response
		switch req := wrappedRequest.GetPayload().(type) {
		case *pb.Request_BlockRequest:
			logger.Info("[Go Server] Nhận được yêu cầu BlockRequest cho block: %d", req.BlockRequest.GetBlockNumber())
			res, err := se.requestHandler.HandleBlockRequest(req.BlockRequest)
			if err != nil {
				logger.Error("[Go Server] Lỗi xử lý block request: %v", err)
				continue // Bỏ qua và chờ request tiếp theo
			}
			logger.Error("[Go Server] Đã xử lý xong BlockRequest, gửi ValidatorList với %d validators", len(res.GetValidators()))
			wrappedResponse = &pb.Response{
				Payload: &pb.Response_ValidatorList{
					ValidatorList: res,
				},
			}
		case *pb.Request_StatusRequest:
			logger.Info("[Go Server] Nhận được yêu cầu StatusRequest")
			status := &pb.ServerStatus{
				StatusMessage: "Server is running smoothly",
				UptimeSeconds: 9001,
			}
			wrappedResponse = &pb.Response{
				Payload: &pb.Response_ServerStatus{
					ServerStatus: status,
				},
			}
		default:
			logger.Error("[Go Server] Loại request không xác định")
			continue // Bỏ qua và chờ request tiếp theo
		}

		if err := WriteMessage(conn, wrappedResponse); err != nil {
			logger.Error("[Go Server] Lỗi gửi response: %v", err)
			return
		}
	}
}

// Start bắt đầu Socket Executor
func (se *SocketExecutor) Start() error {
	se.mu.Lock()
	if se.isRunning {
		se.mu.Unlock()
		return fmt.Errorf("SocketExecutor đang chạy")
	}
	se.isRunning = true
	se.mu.Unlock()

	// THAY ĐỔI: Không Connect() nữa, mà là listenAndServe()
	if err := se.listenAndServe(); err != nil {
		se.mu.Lock()
		se.isRunning = false
		se.mu.Unlock()
		return err
	}

	logger.Info("[Go Server] SocketExecutor đã được khởi động")
	return nil
}

// Stop dừng Socket Executor và đóng kết nối
func (se *SocketExecutor) Stop() error {
	se.mu.Lock()
	defer se.mu.Unlock()
	if !se.isRunning {
		return fmt.Errorf("SocketExecutor chưa chạy")
	}
	close(se.quit)
	// Đóng listener để ngừng chấp nhận kết nối mới
	if se.listener != nil {
		se.listener.Close()
	}
	// Đợi tất cả goroutines kết thúc
	se.wg.Wait()
	se.isRunning = false
	logger.Info("[Go Server] SocketExecutor đã dừng.")
	return nil
}

// RunSocketExecutor là hàm tiện ích để khởi tạo và chạy SocketExecutor
func RunSocketExecutor(socketPath string, storageManager *storage.StorageManager, chainState *blockchain.ChainState) (*SocketExecutor, error) {
	executor := NewSocketExecutor(socketPath, storageManager, chainState)
	if err := executor.Start(); err != nil {
		return nil, fmt.Errorf("không thể khởi động SocketExecutor: %v", err)
	}

	return executor, nil
}

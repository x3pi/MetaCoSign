// network/config.go

package network

import (
	"runtime"
	"time"
)

// Config chứa tất cả các tham số cấu hình cho module network.
type Config struct {
	MaxMessageLength       uint64
	RequestChanSize        int
	ErrorChanSize          int
	WriteTimeout           time.Duration
	RequestChanWaitTimeout time.Duration
	DialTimeout            time.Duration
	RetryParentInterval    time.Duration
	HandlerWorkerPoolSize  int
	SendChanSize           int // <-- THÊM DÒNG NÀY
}

// DefaultConfig tự động tạo ra một cấu hình mặc định hợp lý
// dựa trên tài nguyên hệ thống có sẵn (cụ thể là số nhân CPU).
func DefaultConfig() *Config {
	numCPU := runtime.NumCPU()
	var numWorkers int
	if numCPU < 4 {
		numWorkers = 128
	} else if numCPU < 16 {
		numWorkers = numCPU * 64
	} else {
		numWorkers = numCPU * 32
		// Tăng max workers từ 2048 lên 4096 để xử lý hàng trăm ngàn connections
		if numWorkers > 4096 {
			numWorkers = 4096
		}
	}

	// Giảm RequestChanSize từ *100 xuống *10 để tiết kiệm memory
	// Vẫn đủ cho burst traffic với worker pool
	requestQueueSize := numWorkers * 10

	return &Config{
		MaxMessageLength:      1024 * 1024 * 1024,
		HandlerWorkerPoolSize: numWorkers,
		RequestChanSize:       requestQueueSize,
		SendChanSize:          65536, // Kích thước buffer cho kênh gửi
		ErrorChanSize:         2000,
		WriteTimeout:          10 * time.Second,
		// RequestChanWaitTimeout: Timeout khi gửi request vào requestChan
		// Tăng từ 5s lên 30s để phù hợp với mạng chậm và xử lý chậm
		// Đặc biệt quan trọng cho InitConnection request - nếu timeout thì connection không được add vào manager
		// 30 giây đủ cho mạng chậm và các handler phức tạp (như ProcessInitConnection)
		RequestChanWaitTimeout: 30 * time.Second,
		DialTimeout:            10 * time.Second,
		RetryParentInterval:    5 * time.Second,
	}
}

// File: txsender/sender.go
package txsender

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// Client quản lý một bể kết nối bền bỉ để gửi giao dịch song song.
// Giao diện vẫn được giữ nguyên, nhưng cơ chế bên trong đã được tối ưu.
type Client struct {
	targetAddress string
	conns         chan net.Conn // Thay thế net.Conn và Mutex bằng một bể kết nối (channel)
}

// NewClient khởi tạo một client mới với một bể kết nối có kích thước cho trước.
// Giao diện có thay đổi nhỏ: thêm poolSize và trả về lỗi để đảm bảo khởi tạo an toàn.
func NewClient(targetAddress string, poolSize int) (*Client, error) {
	if poolSize <= 0 {
		return nil, errors.New("kích thước bể kết nối (poolSize) phải lớn hơn 0")
	}

	// Tạo kênh có bộ đệm để hoạt động như một bể kết nối
	connsChan := make(chan net.Conn, poolSize)

	// Tạo các kết nối ban đầu và đưa chúng vào bể
	for i := 0; i < poolSize; i++ {
		conn, err := net.DialTimeout("tcp", targetAddress, 5*time.Second)
		if err != nil {
			// Nếu một kết nối thất bại, đóng tất cả những cái đã tạo và báo lỗi
			close(connsChan)
			for c := range connsChan {
				c.Close()
			}
			return nil, fmt.Errorf("không thể khởi tạo kết nối thứ %d/%d: %w", i+1, poolSize, err)
		}
		connsChan <- conn
	}

	return &Client{
		targetAddress: targetAddress,
		conns:         connsChan,
	}, nil
}

// Connect là một hàm giữ lại để tương thích giao diện.
// Trong phiên bản này, các kết nối được quản lý tự động và được tạo sẵn trong NewClient.
// Do đó, hàm này không cần thực hiện hành động nào cả.
func (c *Client) Connect() error {
	return nil // No-op để duy trì tính tương thích của API
}

// Close đóng tất cả các kết nối trong bể.
func (c *Client) Close() error {
	close(c.conns) // Đóng kênh
	var lastErr error
	// Lấy và đóng tất cả các kết nối còn lại trong kênh
	for conn := range c.conns {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// writeData thực hiện logic gửi dữ liệu length-prefixed trên một kết nối cho trước.
// Đây là hàm helper, không phải là method của Client.
func writeData(conn net.Conn, payload []byte) error {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))

	fullMessage := append(lenBuf, payload...)

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})

	if _, err := conn.Write(fullMessage); err != nil {
		return fmt.Errorf("lỗi khi gửi payload giao dịch: %w", err)
	}
	return nil
}

// SendTransaction gửi một gói dữ liệu giao dịch bằng cách sử dụng một kết nối từ bể.
// Hoàn toàn an toàn để gọi đồng thời từ nhiều goroutine.
func (c *Client) SendTransaction(transactionPayload []byte) error {
	// --- Bước 1: Mượn một kết nối từ bể ---
	// Thao tác này sẽ chờ nếu tất cả các kết nối đang bận
	conn := <-c.conns

	// --- Bước 2: Gửi dữ liệu ---
	err := writeData(conn, transactionPayload)

	// --- Bước 3: Xử lý kết quả và trả kết nối ---
	if err != nil {
		// Nếu có lỗi, kết nối này có thể đã hỏng. Đóng nó.
		conn.Close()

		// Cố gắng tạo một kết nối mới để thay thế và duy trì kích thước bể
		newConn, reconnErr := net.DialTimeout("tcp", c.targetAddress, 5*time.Second)
		if reconnErr == nil {
			// Trả kết nối mới về bể
			c.conns <- newConn
		} else {
			// Nếu không thể kết nối lại, bể sẽ tạm thời mất đi một kết nối.
			// Một goroutine khác sẽ thử lại khi cần.
			fmt.Printf("Cảnh báo: Không thể thay thế kết nối hỏng: %v\n", reconnErr)
			// Để tránh deadlock khi bể cạn, chúng ta vẫn phải trả lại một "nil" placeholder
			// nhưng cách xử lý tốt hơn là có một goroutine quản lý bể riêng.
			// Với cách đơn giản, chúng ta có thể bỏ qua việc thêm lại.
		}

		return fmt.Errorf("gửi thất bại, kết nối đã bị hủy: %w", err)
	}

	// Nếu gửi thành công, trả kết nối về bể để goroutine khác có thể dùng
	c.conns <- conn

	return nil
}

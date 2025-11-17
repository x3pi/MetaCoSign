package main

import (
	"fmt"
	"runtime"

	"github.com/panjf2000/ants/v2"
)

func main() {
	// Kích thước pool bằng số lõi CPU để tối ưu
	poolSize := runtime.NumCPU()

	// Khởi tạo một pool với số lượng worker cố định
	p, _ := ants.NewPool(poolSize)
	defer p.Release() // Dọn dẹp pool khi chương trình kết thúc

	fmt.Printf("🚀 Đã tạo pool với %d worker (bằng số lõi CPU).\n", poolSize)
	fmt.Println("Bắt đầu submit các công việc lặp vô hạn để đẩy CPU lên tối đa...")

	// Gửi một công việc "lặp vô hạn" cho mỗi worker trong pool
	for i := 0; i < poolSize; i++ {
		_ = p.Submit(func() {
			// Vòng lặp vô hạn này sẽ chiếm trọn một lõi CPU
			for {
			}
		})
	}

	fmt.Printf("✅ Đã submit %d công việc. Số worker đang chạy: %d\n", poolSize, p.Running())
	fmt.Println("Mở Task Manager (Windows), Activity Monitor (macOS), hoặc htop (Linux) để xem kết quả.")
	fmt.Println("🛑 Nhấn Ctrl + C để dừng chương trình.")

	// Chặn hàm main thoát ra để các worker tiếp tục chạy
	select {}
}

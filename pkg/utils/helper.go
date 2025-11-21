package utils

import (
	"strings"

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

func Abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func ParseCfgStr2Arr(rawKeys string) []string {
	if rawKeys == "" {
		logger.Warn("Cảnh báo: Chuỗi  đầu vào rỗng.")
		return []string{} // Trả về slice rỗng thay vì nil để an toàn hơn
	}
	cleanedRawKeys := strings.Trim(rawKeys, `"`)
	// 1. Tách chuỗi bằng dấu phẩy
	//    strings.Split sẽ tạo ra một slice các chuỗi con.
	//    Ví dụ: "pk1, \n pk2" -> ["pk1", " \n pk2"]
	keyParts := strings.Split(cleanedRawKeys, ",")
	cleanKeys := make([]string, 0, len(keyParts))
	for _, part := range keyParts {
		// strings.TrimSpace là hàm "thần kỳ", nó loại bỏ tất cả các ký tự
		// khoảng trắng (space, tab, newline, carriage return...) ở cả đầu và cuối chuỗi.
		// Ví dụ: " \n  mykey  \t " -> "mykey"
		trimmedKey := strings.TrimSpace(part)
		// 4. Chỉ thêm vào kết quả nếu chuỗi không rỗng sau khi làm sạch
		//    Điều này để xử lý các trường hợp như "pk1,,pk2" (có dấu phẩy thừa).
		if trimmedKey != "" {
			cleanKeys = append(cleanKeys, trimmedKey)
		}
	}
	return cleanKeys
}

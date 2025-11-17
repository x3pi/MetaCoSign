package constants

// Cấu hình
const (
	// Thay bằng URL của node Ethereum (ví dụ: Ganache)
	// rpcUrl = "wss://rpc-proxy-sequoia.iqnb.com:8446"
	RpcUrl  = "ws://192.168.1.234:8545"
	HttpUrl = "http://192.168.1.234:8545"

	// Thay bằng khóa riêng của tài khoản deploy hoặc tương tác
	PrivateKeyHex = "fb64857fe95b55dff91a11d2da0c8db2dddb29f617d3d1ddaa9a9880733d5407"

	ContractAddressHex = "0xdC7b4fFD274112318083CCB3900d6287776f78b6"
	FilePath           = "./file_to_upload/output.txt"
	ChainId            = 991 // Thay bằng Chain ID của mạng bạn sử dụng
	// Kích thước mỗi chunk (ví dụ: 1KB)
	ChunkSize = 600 * 1024
)

package models

// --- Các hằng số cho việc kết nối ---

type FileStatus int

const (
	Processing FileStatus = 0
	Active     FileStatus = 1
)

// Info: Metadata của file, được lưu trữ (mô phỏng blockchain state).
type Info struct {
	Owner       string
	FileHash    string
	ContentLen  int64
	TotalChunks int
	Name        string
	Status      FileStatus
}

// --- Các cấu trúc để giao tiếp với Rust qua TCP ---

type Command struct {
	Command string      `json:"command"`
	Payload interface{} `json:"payload"`
}

type UploadChunkPayload struct {
	FileKey         string `json:"file_key"`
	ChunkIndex      int    `json:"chunk_index"`
	ChunkDataBase64 string `json:"chunk_data_base64"`
	Signature       string `json:"signature"`
}

type GenericResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Request/Response structures
type DownloadChunkPayload struct {
	FileKey     string `json:"file_key"`
	DownloadKey string `json:"download_key"`
	ChunkIndex  int    `json:"chunk_index"`
	Signature   string `json:"signature"`
}

type DownloadChunkRequest struct {
	Command string               `json:"command"`
	Payload DownloadChunkPayload `json:"payload"`
}

type DownloadResponse struct {
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	ChunkDataBase64 *string `json:"chunk_data_base64,omitempty"`
}

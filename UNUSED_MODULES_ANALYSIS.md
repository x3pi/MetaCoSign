# Phân tích các module không được sử dụng trong pkg/

## Các module có thể loại bỏ:

### 1. **pkg/node_consensus/**
- **Lý do**: Không có file nào import hoặc sử dụng module này
- **Mô tả**: Module này có code để khởi tạo libp2p host và consensus node, nhưng không được sử dụng trong codebase
- **File**: `pkg/node_consensus/node.go`

### 2. **pkg/storagenode/**
- **Lý do**: Không có file nào import hoặc sử dụng module này
- **Mô tả**: Module này có code để tạo và quản lý storage node, nhưng không được sử dụng trong codebase
- **File**: `pkg/storagenode/storage_node.go`

### 3. **pkg/poh/**
- **Lý do**: Chỉ có proto file (poh.pb.go) reference, không có import thực sự từ pkg/poh
- **Mô tả**: Module này có code cho LeaderSchedule, nhưng không được import ở đâu cả. Proto file chỉ là generated code
- **Files**: `pkg/poh/leader_schedule.go`, `pkg/poh/leader_schedule_test.go`

### 4. **pkg/filters/**
- **Lý do**: Không có file nào import hoặc sử dụng module này
- **Mô tả**: Module này có FilterSystem và EventSystem cho Ethereum filtering, nhưng không được sử dụng trong codebase
- **Files**: `pkg/filters/filter_system.go`, `pkg/filters/filters.go`

## Các module ĐƯỢC SỬ DỤNG (KHÔNG nên xóa):

- **pkg/pathdetector/** - Được sử dụng trong `pkg/config/config.go`
- **pkg/ldb_storage/** - Được sử dụng trong `pkg/storage/bls_account_storage.go` và `cmd/rpc-client/app/context.go`
- **pkg/quic_network/** - Được sử dụng trong `pkg/file_handler/file_handler.go`
- **pkg/mining/** - Được sử dụng trong `pkg/storage/storage_manager.go`
- **pkg/explorer/** - Được sử dụng trong `pkg/storage/storage_manager.go`
- **pkg/monitor_service/** - Được sử dụng trong `pkg/monitor_service/monitor.go` (nội bộ)
- **pkg/script/** - Được sử dụng trong `cmd/rpc-client/client-tcp/network/handler.go`
- **pkg/stats/** - Được sử dụng trong `pkg/storage/storage_manager.go`

## Tổng kết:

**4 module** có thể được loại bỏ an toàn:
1. `pkg/node_consensus/`
2. `pkg/storagenode/`
3. `pkg/poh/`
4. `pkg/filters/`

**Lưu ý**: 
- Trước khi xóa, nên kiểm tra lại bằng cách:
  1. Chạy `go build ./...` để đảm bảo không có lỗi compile
  2. Chạy tests để đảm bảo không có test nào phụ thuộc vào các module này
  3. Tìm kiếm trong toàn bộ codebase (bao gồm cả comments, docs) để chắc chắn không có reference nào


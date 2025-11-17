# 🚀 Unix Domain Socket - Rust ↔ Go Protocol Implementation

## ✅ Đã hoàn thành

### 1. **Protocol Buffer Definitions**
- ✅ Tạo `validator.proto` với 3 messages: `BlockRequest`, `Validator`, `ValidatorList`
- ✅ Auto-generate Rust code từ proto file
- ✅ Tích hợp với Go protobuf (đã có sẵn)

### 2. **Rust Implementation** (`socket-rust/socket/`)
- ✅ Server lắng nghe trên 2 socket paths: `/tmp/rust-go.sock_1` và `/tmp/rust-go.sock_2`
- ✅ Gửi `BlockRequest` mỗi 5 giây với block number tăng dần
- ✅ Nhận và deserialize `ValidatorList` từ Go
- ✅ In thông tin validators ra console
- ✅ Support multi-threading cho mỗi connection

### 3. **Go Implementation** (`executor/unitSokcet.go`)
- ✅ Nhận `BlockRequest` từ Rust
- ✅ Query validators từ `ChainState` và `StorageManager`
- ✅ Tạo và serialize `ValidatorList`
- ✅ Gửi response về Rust
- ✅ Thread-safe với mutex
- ✅ Graceful shutdown

### 4. **Build & Deployment**
- ✅ Rust build script (`build.rs`) để compile proto
- ✅ Run script (`run.sh`) để dễ dàng start Rust server
- ✅ Go test program (`cmd/socket_server/main.go`)

### 5. **Documentation**
- ✅ `PROTOCOL_README.md` - Chi tiết về protocol và cách sử dụng
- ✅ `IMPLEMENTATION_SUMMARY.md` - Tóm tắt implementation (file này)
- ✅ Inline comments trong code

## 📦 Cấu trúc thư mục

```
socket-rust/socket/
├── proto/
│   └── validator.proto          # ✅ Proto definitions
├── src/
│   ├── proto/
│   │   ├── mod.rs              # ✅ Module export
│   │   └── validator.rs        # ✅ Generated code
│   └── main.rs                 # ✅ Rust server
├── build.rs                     # ✅ Build script
├── run.sh                       # ✅ Run script
├── PROTOCOL_README.md          # ✅ Documentation
└── Cargo.toml

mtn-simple-2025/
├── executor/
│   ├── unitSokcet.go           # ✅ Go server implementation
│   └── SOCKET_README.md         # ✅ Documentation
└── cmd/
    └── socket_server/
        └── main.go              # ✅ Test program
```

## 🔄 Protocol Flow

```
┌─────────┐                              ┌─────────┐
│  Rust   │                              │   Go    │
│ Client  │                              │ Server  │
└────┬────┘                              └────┬────┘
     │                                        │
     │  1. Connect to Unix Socket             │
     ├───────────────────────────────────────>│
     │                                        │
     │  2. Send: [4 bytes len] + BlockRequest │
     ├───────────────────────────────────────>│
     │                                        │
     │                     3. Query Validators│
     │                        from ChainState │
     │                                        │
     │  4. Send: [4 bytes len] + ValidatorList│
     │<───────────────────────────────────────┤
     │                                        │
     │  5. Print validator info               │
     │                                        │
     │  6. Wait 5 seconds                     │
     │                                        │
     │  7. Send next BlockRequest (num+1)     │
     ├───────────────────────────────────────>│
     │                                        │
     └────────────────────────────────────────┘
```

## 🎯 Cách sử dụng

### Terminal 1: Start Go Server
```bash
cd /home/abc/nhat/mtn-simple-2025

# Trong main application của bạn:
executor1, _ := executor.RunSocketExecutor("/tmp/rust-go.sock_1", storageManager, chainState)
executor2, _ := executor.RunSocketExecutor("/tmp/rust-go.sock_2", storageManager, chainState)

# Hoặc test với:
go run cmd/socket_server/main.go
```

### Terminal 2: Start Rust Client
```bash
cd /home/abc/nhat/socket-rust/socket
./run.sh

# Hoặc:
cargo run --release
```

## 📊 Output mẫu

### Rust:
```
[Rust Server] Listening on /tmp/rust-go.sock_1
[Rust Server] Listening on /tmp/rust-go.sock_2
[Rust Server] Go client connected on /tmp/rust-go.sock_1
[/tmp/rust-go.sock_1] Gửi request cho block number: 1
[/tmp/rust-go.sock_1] Nhận được 10 validators cho block 1
  Validator 1: ValidatorName - 192.168.1.1:8080
  Validator 2: ValidatorName2 - 192.168.1.2:8080
  ... và 8 validators khác
```

### Go:
```
[Go Client] Đã kết nối thành công tới /tmp/rust-go.sock_1
[Go Client] SocketExecutor đã được khởi động
[Go Client]____ ValidatorList cho block number: 1
[Go Client]____ ValidatorList cho block number: 2
```

## 🔧 API Reference

### Rust

**Main Function:**
```rust
fn handle_client(stream: UnixStream, socket_path: &'static str)
```
- Xử lý connection từ Go
- Gửi BlockRequest định kỳ
- Nhận và parse ValidatorList

### Go

**Main Function:**
```go
func RunSocketExecutor(
    socketPath string, 
    storageManager *storage.StorageManager, 
    chainState *blockchain.ChainState
) (*SocketExecutor, error)
```
- Tạo và start SocketExecutor
- Lắng nghe BlockRequest
- Query và trả về ValidatorList

**Internal:**
```go
func (se *SocketExecutor) generateAndFillValidators(blockNumber uint64) (*pb.ValidatorList, error)
```
- Query validators từ ChainState
- Chuyển đổi sang Protobuf format

## 🧪 Testing

### Build Test:
```bash
# Rust
cd socket-rust/socket
cargo build --release

# Go
cd mtn-simple-2025
go build ./cmd/socket_server
```

### Run Test:
```bash
# Terminal 1
cd mtn-simple-2025
go run cmd/socket_server/main.go

# Terminal 2
cd socket-rust/socket
./run.sh
```

## 🐛 Common Issues

### 1. "Address already in use"
```bash
rm /tmp/rust-go.sock_1 /tmp/rust-go.sock_2
```

### 2. "Connection refused"
- Đảm bảo Go server chạy trước Rust client
- Check socket paths đúng

### 3. "No validators found"
- Đảm bảo ChainState đã được khởi tạo đúng
- Check database có dữ liệu validators

## 📈 Performance

- **Frequency**: Request mỗi 5 giây
- **Connections**: 2 concurrent connections
- **Serialization**: Protocol Buffers (rất nhanh)
- **Threading**: Multi-threaded trên cả Rust và Go
- **Memory**: Minimal overhead

## 🎉 Kết luận

Hệ thống đã được implement đầy đủ và sẵn sàng sử dụng:

✅ Protocol Buffers definitions  
✅ Rust client hoàn chỉnh  
✅ Go server hoàn chỉnh  
✅ Build scripts & tools  
✅ Documentation đầy đủ  
✅ Error handling  
✅ Multi-connection support  

**Bạn có thể bắt đầu test ngay!** 🚀

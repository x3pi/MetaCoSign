# Hướng Dẫn Khởi Chạy Chain

## 1. Cấu Hình Các Loại Node

Trong hệ thống, có các loại node với cấu hình như sau:

- **WRITE**: Node có thể gửi giao dịch lên master.
- **MASTER**: Node chính.
- **READONLY**: Node chỉ đọc từ master, thể gửi giao dịch lên master.
- **SIMPLE**: Chạy 1 node độc lập.

Ví dụ về các file cấu hình:

- `cmd/simple_chain/config-write-2.json`
- `cmd/simple_chain/config-write.json`
- `cmd/simple_chain/config.json`

Tạo khối genesis theo ví dụ config:

- `cmd/simple_chain/genesis.json`


## 2. Khởi Chạy Chain

- cần cài thêm thư viện tbb

### 2.1. Lệnh Chạy

Để khởi chạy chain, sử dụng lệnh sau trong terminal:

Ví dụ run `MASTER`:

Thiết lập biến môi trường cho evm master. Cần để các thự mục database này trong thư mục gốc chung `sample/simple/data/data` vì đồng bộ sẽ nén toàn bộ thư mục này 

```
export XAPIAN_BASE_PATH="sample/simple/data/data/xapian_node"
```


```sh
go run  . -config=config-master.json
```

Ví dụ run `WRITE`:
Thiết lập biến môi trường cho evm sub write

```
export XAPIAN_BASE_PATH="sample/simple/data-write/data/xapian_node"
```
```sh
go run . -config=config-sub-write.json
```


```
export XAPIAN_BASE_PATH="sample/simple/data-write-2/data/xapian_node"
```
```sh
go run . -config=config-sub-write-2.json
```

> **Lưu ý**: Thay `config.json` bằng file cấu hình phù hợp với môi trường của bạn.

- Có thể chạy master hoặc sub trước đều được nhưng nếu tắt chain thì tắt master trước nếu khống master vẫn chạy trong lúc sub tắt.



## 3. Chạy RPC-Client Trỏ Tới RPC Node

### 3.1. File Cấu Hình RPC-Client

Ví dụ về file cấu hình:

```json
{
  "rpc_server_url": "http://localhost:8646",
  "wss_server_url": "ws://localhost:8646/ws",
  "private_key": "2b3aa0f620d2d73c046cd93eb64f2eb687a95b22e278500aa251c8c9dda1203b",
  "server_port": ":8545",
  "chain_id": 1000
}
```
`rpc_server_url` và `wss_server_url` có thể trỏ tới node `MASTER` hoặc node `WRITE`

File cấu hình ví dụ nằm tại:

```
cmd/rpc-client/config.json
```


### 3.2. Kết Nối RPC-Client Với MetaMask

Bạn có thể kết nối `rpc-client` với MetaMask bằng cách:

1. Mở MetaMask và vào phần **Cài đặt**.
2. Chọn **Mạng** → **Thêm mạng mới**.
3. Nhập thông tin mạng từ file `config.json`, bao gồm:
   - RPC URL: `http://localhost:8545`
   - Chain ID: `1234` (hoặc giá trị tương ứng trong file config)
   - Tên mạng: Tùy chọn
4. Lưu và kết nối.




## 5. Note đồng bộ giữa các node sub và `Master` chỉ mới đồng bộ block, chưa được đồng bộ dữ liệu về:
- `cmd/simple_chain/sample/simple/data/backup_device_key_storage`: Dữ liệu device key
- `cmd/simple_chain/sample/simple/data/txs_eth`: dữ liệu map ánh xạ hash của giao dịch secp và giao dịc bls
- Nếu cần bổ sung đầy đủ hiện tại cần phải sao chép thêm hai thư mục này hoặc tham khảo tạo bản nén backup cho toàn bộ chain `docs/CreateBackup.md`


# Hướng dẫn chạy ứng dụng (How to Run App)

## 1.1. Cấu hình RPC
file `config-client-tcp.json` 

```json
{
  "version": "0.0.1.0",
  "private_key": "2b3aa0f620d2d73c046cd93eb64f2eb687a95b22e278500aa251c8c9dda1203b",
  "parent_connection_address": "0.0.0.0:4200",
  "parent_address": "0x7d03201fee4675987894617138e5ee7e038a6b39",
  "chain_id": 991,
  "parent_connection_type:": "client",
  "owner_file_storage_address":"0x7d03201fee4675987894617138e5ee7e038a6b39",
  "pk_admin_file_storage":"87d931eaa2f76709f2615586e0d560ca9b80f247c9cc431e197ba3e7167db623",
  "bls_admin_storage":"2b3aa0f620d2d73c046cd93eb64f2eb687a95b22e278500aa251c8c9dda1203b"
}
```
### Giải thích các trường quan trọng:
- **`parent_connection_address`**: Ip đến chain .

file `config-rpc.json` tại `cmd/rpc-client` (hoặc cập nhật cấu hình tương ứng) với nội dung sau:

```json
{
  "rpc_server_url": "http://localhost:8646",
  "wss_server_url": "ws://localhost:8646/ws",
  "private_key": "2b3aa0f620d2d73c046cd93eb64f2eb687a95b22e278500aa251c8c9dda1203b",
  "server_port": ":8545",
  "chain_id": 991,
  "https_port": ":8666",
  "cert_file": "certificate.pem",
  "key_file": "private.key",
  "master_password": "your_strong_master_password_here",
  "app_pepper": "your_unique_secret_pepper_here",
  "ldb_bls_wallet_path": "./db/bls_wallets",
  "ldb_bls_account_noti": "./db/bls_account_noti",
  "owner_rpc_address": "0x0b143e894a600114c4a3729874214e5fc5ea9cbc",
  "contracts_interceptor": [
    "0x00000000000000000000000000000000D844bb55"
  ]
}
```
### Giải thích các trường quan trọng:
- **`rpc_server_url / wss_server_url`**: Đường dẫn đến chain.
- **`private_key`**: private_key của bls.
- **`server_port`**: port của rpc.
- **`ldb_bls_wallet_path`**: Đường dẫn đến database lưu trữ ví BLS.
- **`ldb_bls_account_noti`**: Đường dẫn đến database lưu thông tin notification của các tài khoản BLS.
- **`owner_rpc_address`**: Địa chỉ ví của chủ sở hữu (Admin) để ký các giao dịch quản lý tài khoản.
- **`contracts_interceptor`**: Danh sách contract chặn lại để xử lý event (phần tử index 0 là địa chỉ của contract quản lý tài khoản).


---
## 1.1. Cấu hình FE
file `src/constants/customChain.ts`, cập nhật `GO_BACKEND_RPC_URL` và `WSS_RPC` với địa chỉ RPC và WSS tương ứng của bạn:

```typescript
export const GO_BACKEND_RPC_URL = "http://192.168.1.234:8545";
export const WSS_RPC = "ws://192.168.1.234:8545";
```
abi được đặt cấu hình ở `metaCoSign/web3/dapp/register-private-key-rpc/src/constants/contracts.ts`

## 2. Hướng dẫn chạy (Run Instructions)

### Bước 1: Chạy RPC Client (Backend)

Di chuyển vào thư mục `rpc-client` và chạy lệnh, logs nằm ở `cmd/rpc-client/logs` log theo ngày:

```bash
cd cmd/rpc-client
go run .
```

### Bước 2: Chạy Giao diện (Frontend)

Di chuyển vào thư mục dự án frontend:

```bash
cd web3/dapp/register-private-key-rpc
```

Cài đặt dependencies (nếu chưa cài):
```bash
yarn install
```

Khởi chạy ứng dụng ở chế độ development:
```bash
yarn dev
```

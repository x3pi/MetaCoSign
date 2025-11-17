## Hướng Dẫn Debug Thời Gian Thực

Tài liệu này mô tả cách:

1. Theo dõi log theo thời gian thực qua WebSocket.
2. Liệt kê các kết nối TCP mà `ConnectionsManager` đang quản lý.
3. Xem log cho `rpc-client` qua HTTP.
4. Đọc log master/sub qua API JSON-RPC (tùy chọn định dạng).

Các ví dụ dưới đây giả sử node RPC đang chạy tại `http://127.0.0.1:8545`. Thay đổi địa chỉ cho phù hợp với môi trường của bạn.

---

### 1. Stream log qua WebSocket

- **Endpoint**: `ws://<host>/debug/logs/ws`
- **Query params**

| Tham số | Bắt buộc | Mặc định | Giải thích |
| ------- | -------- | -------- | ---------- |
| `file`  | Có       | –        | Tên file log (ví dụ `App.log`, `App__.log`). |
| `date`  | Không    | Ngày hiện tại | Định dạng `YYYY-MM-DD`. Bỏ trống để luôn bám theo ngày hiện tại và tự chuyển file khi sang ngày mới. |
| `root`  | Không    | `app.config.LogPath` hoặc `loggerfile` global dir | Thư mục gốc chứa cây log `YYYY/MM/DD`. Có thể truyền path tuyệt đối. |
| `follow`| Không    | `true` nếu `date` rỗng, ngược lại `false` | Nếu `true` và `date` rỗng, server tự động chuyển qua thư mục ngày mới lúc nửa đêm. |

- **Ví dụ với `wscat`**

```bash
wscat -c "ws://127.0.0.1:8545/debug/logs/ws?file=App.log&root=/mnt/Data/AAA/mtn-simple-2025/cmd/simple_chain/sample/simple/data/logs"
```

```bash
   wscat -c "ws://127.0.0.1:8545/debug/logs/ws?file=App.log"
```

Sau khi kết nối thành công:

```
< info: streaming /mnt/.../2025/11/14/App.log (offset 123456)
< 2025-11-14T07:00:01Z [INFO] Node started
< 2025-11-14T07:00:02Z [WARN] ...
```

- **Ví dụ với Go (sử dụng `gorilla/websocket`)**

```go
conn, _, err := websocket.DefaultDialer.Dial(
    "ws://127.0.0.1:8545/debug/logs/ws?file=App__.log",
    nil,
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

for {
    _, msg, err := conn.ReadMessage()
    if err != nil {
        log.Println("stream ended:", err)
        break
    }
    fmt.Print(string(msg))
}
```

**Lưu ý**:
- Server đẩy cả thông báo lỗi dạng `error: ...` nếu file chưa tồn tại hoặc mất quyền truy cập.
- Khi `follow=true` và sang ngày mới, server tự đóng file cũ, mở file `YYYY/MM/DD/App.log` mới và thông báo `info: streaming ...`.

---

### 2. Liệt kê các kết nối đang được quản lý

- **Phương thức JSON-RPC**: `debug_listManagedConnections`
- **Tham số**:

| Trường | Kiểu | Bắt buộc | Mô tả |
| ------ | ---- | -------- | ----- |
| `type` | string | Không | Tên loại kết nối cần lọc (`child_node`, `master`, `storage`, ...). Bỏ trống để lấy tất cả. |

- **Ví dụ gọi bằng `curl`**

```bash
curl -X POST http://127.0.0.1:8545 \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "debug_listManagedConnections",
    "params": [{
      "type": "child_node"
    }]
  }'
```

- **Kết quả mẫu**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": [
    {
      "type": "child_node",
      "address": "0x07d5...3ba2",
      "remoteAddr": "203.113.1.10:9000",
      "connected": true,
      "isParent": false,
      "tcpRemoteAddr": "203.113.1.10:9000",
      "tcpLocalAddr": "10.0.0.5:34567",
      "label": "child_node"
    }
  ]
}
```

- **Ý nghĩa các trường**:
  - `type`: loại kết nối (chuẩn hóa từ `conn.Type()`).
  - `address`: BLS/ETH address mà kết nối đại diện (dạng hex).
  - `remoteAddr`: địa chỉ TCP hiện tại mà socket sử dụng (nếu có).
  - `connected`: trạng thái `IsConnect()`.
  - `isParent`: `true` nếu đây là kết nối cha (parent connection) đang được `ConnectionsManager` theo dõi.
  - `tcpRemoteAddr` / `tcpLocalAddr`: giá trị từ `conn.TcpRemoteAddr()` và `conn.TcpLocalAddr()` (chuỗi `IP:port`, có thể rỗng).

- **Gợi ý**:
  - Có thể tạo script định kỳ gọi API này để giám sát số lượng kết nối theo từng loại.
  - Kết hợp với endpoint WebSocket log để kiểm tra realtime khi số kết nối tăng/giảm bất thường.

---

### 3. Log HTTP cho rpc-client

Binary `cmd/rpc-client` cung cấp hai endpoint REST đơn giản để xem log:

1. `GET /debug/logs/list?date=YYYY-MM-DD&root=<tùy chọn>`
   - Trả về danh sách file log trong ngày chỉ định.
   - `root` bỏ trống thì dùng đường dẫn mặc định (cấu hình qua flag `--logs-root`, mặc định `./logs`).

2. `GET /debug/logs/content?file=App.log&date=YYYY-MM-DD&root=<tùy chọn>&maxBytes=131072`
   - Trả về nội dung file log (mặc định đọc tối đa 4MB; có thể điều chỉnh qua `maxBytes`).

Ví dụ:

```bash
curl "http://127.0.0.1:8080/debug/logs/list?date=2025-11-14"

curl "http://127.0.0.1:8080/debug/logs/content?file=rpc-client.log&date=2025-11-14&maxBytes=200000"

# Mở trực tiếp trên trình duyệt:
# http://127.0.0.1:8080/debug/logs/content?file=rpc-client.log&date=2025-11-14&format=html
```

> Lưu ý: cần chạy rpc-client với `--logs-root=/đường/dẫn/logs` (hoặc dùng mặc định) để các endpoint trên biết thư mục log hiện tại.

---

### 4. Log JSON-RPC/HTTP của master/sub (Debug API)

Các phương thức `debug_listLogFiles` và `debug_getLogFileContent` (JSON-RPC) trên node master/sub hỗ trợ tham số `format` khi gọi `debug_getLogFileContent`:

- `format="html"`: trả về HTML `<pre>` đã escape ký tự đặc biệt, tiện xem trong trình duyệt.
- `format="plain"` hoặc bỏ trống: trả string thuần.

Ví dụ:

```bash
curl -X POST http://127.0.0.1:8646 \
  -H 'Content-Type: application/json' \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "debug_getLogFileContent",
    "params": [{
      "root": "./cmd/simple_chain/sample/simple/data/logs",
      "date": "2025-11-14",
      "fileName": "App.log",
      "maxBytes": 131072,
      "format": "html"
    }]
  }'

# Hoặc mở trực tiếp (ví dụ node master chạy port 8646):
# http://127.0.0.1:8646/debug/logs/content?file=App.log&date=2025-11-14&format=html
```

Bạn cũng có thể gọi trực tiếp qua HTTP:

```
http://<master-node>/debug/logs/content?file=App.log&date=2025-11-14&format=html&maxBytes=131072
```

Endpoint này tái sử dụng logic `debug_getLogFileContent` và hỗ trợ `format=html` / `plain`.

---

### 5. Tổng kết

- WebSocket `/debug/logs/ws`: theo dõi log realtime + auto rotate theo ngày.
- JSON-RPC `debug_listManagedConnections`: quan sát toàn bộ kết nối đang được quản lý bởi `ConnectionsManager`.
- HTTP `/debug/logs/*` (rpc-client): lấy danh sách log và đọc nội dung mà không cần SSH.
- JSON-RPC `debug_getLogFileContent`: thêm `format=html/plain` để đọc log master/sub dễ nhìn hơn.

Hai công cụ này giúp giảm nhu cầu SSH vào máy chủ để kiểm tra log/kết nối, đặc biệt hữu ích khi vận hành nhiều node cùng lúc.


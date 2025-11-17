# Race Condition: Transaction có thể được xử lý trước ProcessInitConnection

## Luồng cũ (CÓ RACE CONDITION):

### Timeline khi client kết nối:

```
T0: Client kết nối TCP → TCP connection established
T1: OnConnect() được gọi → Gửi InitConnection message qua TCP
T2: readLoop() nhận InitConnection → Đưa vào requestChan
T3: readLoop() nhận SendTransactionWithDeviceKey → Đưa vào requestChan
T4: Worker Pool xử lý requests từ requestChan (song song, không đảm bảo thứ tự)
    - Worker 1: Xử lý SendTransactionWithDeviceKey → ProcessTransactionFromClientWithDeviceKey()
    - Worker 2: Xử lý InitConnection → ProcessInitConnection()
T5: ProcessTransactionFromClientWithDeviceKey() hoàn thành TRƯỚC ProcessInitConnection()
T6: Transaction được xử lý và tạo receipt
T7: BroadCastReceipts() được gọi → Tìm connection trong manager → KHÔNG TÌM THẤY (chưa được add)
T8: ProcessInitConnection() hoàn thành → Add connection vào manager (QUÁ MUỘN)
```

### Vấn đề:

1. **Worker Pool xử lý song song**: Nhiều workers có thể xử lý requests đồng thời
2. **Không đảm bảo thứ tự**: InitConnection và Transaction có thể được xử lý bởi các workers khác nhau
3. **Race condition**: Transaction có thể hoàn thành trước khi connection được add vào manager
4. **Receipt không gửi được**: BroadCastReceipts không tìm thấy connection để gửi receipt

## Luồng mới (ĐÃ SỬA):

### Timeline khi client kết nối:

```
T0: Client kết nối TCP → TCP connection established
T1: OnConnect() được gọi → Gửi InitConnection message qua TCP
T2: readLoop() nhận InitConnection → Đưa vào requestChan
T3: readLoop() nhận SendTransactionWithDeviceKey → Đưa vào requestChan
T4: Worker Pool xử lý requests từ requestChan
    - Worker 1: Xử lý InitConnection → ProcessInitConnection() → Add vào manager ✅
    - Worker 2: Xử lý SendTransactionWithDeviceKey → ProcessTransactionFromClientWithDeviceKey()
      → checkConnectionInitialized() → Retry nếu chưa có → Đợi connection được add ✅
T5: ProcessTransactionFromClientWithDeviceKey() chỉ tiếp tục SAU KHI connection đã được add
T6: Transaction được xử lý và tạo receipt
T7: BroadCastReceipts() được gọi → Tìm connection trong manager → TÌM THẤY ✅
T8: Receipt được gửi thành công ✅
```

### Giải pháp đã áp dụng:

1. **checkConnectionInitialized()**: Kiểm tra connection đã được add vào manager chưa
2. **Retry logic**: Đợi tối đa 500ms (10 retries × 50ms) để connection được add
3. **Error nếu timeout**: Nếu sau 500ms vẫn chưa có, return error yêu cầu gửi InitConnection trước

## Code Flow:

### Luồng cũ (KHÔNG AN TOÀN):

```go
// Worker Pool xử lý requests song song
case request, ok := <-s.requestChan:
    // Worker 1: Xử lý InitConnection
    // Worker 2: Xử lý SendTransactionWithDeviceKey (có thể chạy trước Worker 1)
    s.handler.HandleRequest(request)
```

### Luồng mới (AN TOÀN):

```go
// ProcessTransactionFromClientWithDeviceKey
func (tp *TransactionProcessor) ProcessTransactionFromClientWithDeviceKey(request network.Request) error {
    // ✅ CHECK TRƯỚC KHI XỬ LÝ
    if err := tp.checkConnectionInitialized(request.Connection()); err != nil {
        return fmt.Errorf("connection not initialized: %w", err)
    }
    // Chỉ tiếp tục nếu connection đã được init
    // ...
}

// checkConnectionInitialized với retry
func (tp *TransactionProcessor) checkConnectionInitialized(conn network.Connection) error {
    // Retry tối đa 10 lần, mỗi lần đợi 50ms
    for retry := 0; retry < 10; retry++ {
        // Tìm connection trong manager
        if found {
            return nil // ✅ Đã tìm thấy
        }
        time.Sleep(50 * time.Millisecond)
    }
    return fmt.Errorf("connection not initialized") // ❌ Timeout
}
```

## Kết quả:

- ✅ **Đảm bảo thứ tự**: Transaction chỉ được xử lý SAU KHI connection đã được init
- ✅ **Tránh race condition**: Retry logic đợi connection được add vào manager
- ✅ **Receipt được gửi**: BroadCastReceipts luôn tìm thấy connection để gửi receipt
- ✅ **Error rõ ràng**: Nếu timeout, client biết cần gửi InitConnection trước

## Debug:

Với logs đã thêm, bạn có thể thấy:
- `[PROCESS_INIT]` - Khi ProcessInitConnection được gọi và hoàn thành
- `[TXS_PROCESSOR2]` - Khi kiểm tra master connection
- `checkConnectionInitialized: retry` - Khi đợi connection được add vào manager


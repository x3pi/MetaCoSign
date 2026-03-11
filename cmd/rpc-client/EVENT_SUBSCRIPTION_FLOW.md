# Luồng Subscribe Event KHÔNG đánh chặn (Forward trực tiếp lên Chain)

## Tổng quan

Đây là luồng khi client subscribe vào contract **KHÔNG** nằm trong `ContractsInterceptor[]`.
RPC proxy chỉ forward 2 chiều, không can thiệp.

```
=== LUỒNG SUBSCRIBE (WebSocket) ===

Client (viem/ethers)
    │
    │  ws://rpc:8545/   (default path, KHÔNG có /interceptor)
    ▼
main.go:176  → ServeWebSocketWithoutInterceptor()
    │
    │  1. Upgrade HTTP → WebSocket (clientConn)
    │  2. Dial đến chain WebSocket (targetConn) 
    │     targetURL = appCtx.ClientRpc.UrlWS  (ws://localhost:10646/ws)
    │  
    ▼
proxyWebSocketTrafficWithoutInterceptor()
    │
    ├── Goroutine 1: Client → Chain
    │   proxyClientToUpstreamWithoutInterceptor()
    │   │  clientConn.ReadJSON(&req) → targetWriter.WriteJSON(req)
    │   │  (Forward thẳng, không check intercept)
    │
    └── Goroutine 2: Chain → Client  
        proxyUpstreamToClient()
        │  targetConn.ReadMessage() → clientWriter.WriteMessage()
        │  (Forward thẳng mọi message từ chain về client)
```

## Chi tiết: Chain xử lý eth_subscribe như nào?

### 1. Client gửi eth_subscribe qua WebSocket

```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscribe",
  "params": ["logs", {
    "address": "0x1234...contractOnChain",
    "topics": [["0xEventSigHash..."]]
  }],
  "id": 1
}
```

RPC Proxy forward thẳng request này lên chain (vì không intercept).

### 2. Chain nhận eth_subscribe

**File:** `mtn-simple-2025/cmd/simple_chain/rpc_subscription.go`

Chain sử dụng thư viện `go-ethereum/rpc` có sẵn WebSocket server.
Khi nhận `eth_subscribe` với param `"logs"`, nó gọi:

```go
// rpc_subscription.go:71
func (api *MetaAPI) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
    notifier, _ := rpc.NotifierFromContext(ctx)  // Lấy WebSocket notifier
    rpcSub := notifier.CreateSubscription()      // Tạo subscription ID
    matchedLogs := make(chan []*types.Log)
    
    // Đăng ký lắng nghe logs vào EventSystem
    logsSub, _ := api.events.SubscribeLogs(crit, matchedLogs)
    
    go func() {
        for {
            select {
            case logs := <-matchedLogs:          // Nhận logs từ EventSystem
                for _, log := range logs {
                    notifier.Notify(rpcSub.ID, &log)  // ★ PUSH LOG VỀ CLIENT QUA WEBSOCKET
                }
            case <-rpcSub.Err():                 // Client unsubscribe
                return
            }
        }
    }()
    
    return rpcSub, nil  // Trả subscription ID về client
}
```

### 3. EventSystem (bộ lọc event) hoạt động như nào?

**File:** `mtn-simple-2025/pkg/filters/filter_system.go`

```go
type EventSystem struct {
    LogsFeed   *event.Feed    // Feed nhận logs từ block processor
    logsCh     chan []*types.Log
    install    chan *subscription
    // ...
}
```

EventSystem chạy goroutine `eventLoop()` liên tục:

```go
// filter_system.go:555
func (es *EventSystem) eventLoop() {
    for {
        select {
        case ev := <-es.logsCh:         // Nhận logs từ LogsFeed
            es.handleLogs(index, ev)    // Phân phối tới các subscriptions
        case f := <-es.install:         // Đăng ký subscription mới
            index[f.typ][f.id] = f
        case f := <-es.uninstall:       // Hủy subscription
            delete(index[f.typ], f.id)
        }
    }
}

// filter_system.go:434
func (es *EventSystem) handleLogs(filters filterIndex, ev []*types.Log) {
    for _, f := range filters[LogsSubscription] {
        // Lọc logs theo criteria (address, topics, block range)
        matchedLogs := FilterLogs(ev, f.logsCrit.Addresses, f.logsCrit.Topics, ...)
        if len(matchedLogs) > 0 {
            f.logs <- matchedLogs  // ★ ĐẨY LOGS VÀO CHANNEL CỦA SUBSCRIPTION
        }
    }
}
```

### 4. Block Processor đẩy logs vào EventSystem khi nào?

**File:** `mtn-simple-2025/cmd/simple_chain/processor/block_processor_broadcast.go`

Khi block mới được xử lý xong, `broadcastEventsAndReceipts()` được gọi:

```go
// block_processor_broadcast.go:70
func (bp *BlockProcessor) broadcastEventsAndReceipts(lastBlock, allReceipts, allEventLogs) {
    // 1. Gửi chain event (cho newHeads subscription)
    bp.eventSystem.ChainFeed.Send(*chainEventData)
    
    // 2. Thu thập TẤT CẢ event logs từ receipts
    var allEthEventLogs []*eth_types.Log
    for _, rpc := range allReceipts {
        for _, eventLog := range rpc.EventLogs() {
            evL := &eth_types.Log{
                Address:     eventLog.Address,
                Topics:      eventLog.Topics,
                Data:        eventLog.Data,
                TxHash:      rpc.TransactionHash(),
                BlockNumber: lastBlock.Header().BlockNumber(),
            }
            allEthEventLogs = append(allEthEventLogs, evL)
        }
    }
    
    // 3. ★ ĐẨY LOGS VÀO EVENT SYSTEM → WebSocket subscribers nhận được
    bp.eventSystem.LogsFeed.Send(allEthEventLogs)
    
    // 4. Broadcast receipts cho TCP clients
    go bp.BroadCastReceipts(allReceipts)
    
    // 5. Broadcast event logs cho TCP subscribers (SubscribeProcessor)
    go bp.BroadCastEventLogs(mapEventLogs)
}
```

### 5. RPC Proxy forward từ chain về client

**File:** `metaCoSign/cmd/rpc-client/internal/proxy/ws_handler.go`

```go
// ws_handler.go:458
func (p *RpcReverseProxy) proxyUpstreamToClient(
    targetConn *websocket.Conn,    // WebSocket tới chain 
    clientWriter *ws_writer.WebSocketWriter,  // Writer tới client
) {
    for {
        // Đọc message từ chain
        messageType, message, _ := targetConn.ReadMessage()
        
        // Forward NGUYÊN VẸN về client (không parse, không modify)
        clientWriter.WriteMessage(messageType, message)
    }
}
```

Message chain gửi về có format:
```json
{
  "jsonrpc": "2.0",
  "method": "eth_subscription",
  "params": {
    "subscription": "0x...",
    "result": {
      "address": "0x1234...",
      "topics": ["0xEventSigHash...", "0xIndexedParam1..."],
      "data": "0x...packedData...",
      "blockNumber": "0x3DF",
      "transactionHash": "0x...",
      "logIndex": "0x0",
      "removed": false
    }
  }
}
```

## Sơ đồ tổng hợp

```
                    ┌──────────────────────────────────────────────────────┐
                    │                    CHAIN NODE                        │
                    │                                                      │
  Block mới được    │  BlockProcessor.broadcastEventsAndReceipts()         │
  xử lý xong       │       │                                              │
                    │       ▼                                              │
                    │  eventSystem.LogsFeed.Send(allEthEventLogs)          │
                    │       │                                              │
                    │       ▼                                              │
                    │  EventSystem.eventLoop()                             │
                    │       │ es.logsCh <- logs                            │
                    │       ▼                                              │
                    │  handleLogs() → FilterLogs() → khớp address/topics  │
                    │       │ f.logs <- matchedLogs                        │
                    │       ▼                                              │
                    │  rpc_subscription.go: Logs() goroutine               │
                    │       │ logs := <-matchedLogs                        │
                    │       ▼                                              │
                    │  notifier.Notify(rpcSub.ID, &log)                    │
                    │       │                                              │
                    │       │  (go-ethereum/rpc WebSocket notifier)         │
                    │       ▼                                              │
                    │  Chain WebSocket Server gửi message                  │
                    └───────┬──────────────────────────────────────────────┘
                            │  WebSocket message
                            ▼
                    ┌──────────────────────────────────────────────────────┐
                    │              RPC PROXY (metaCoSign)                   │
                    │                                                      │
                    │  proxyUpstreamToClient()                             │
                    │       │ targetConn.ReadMessage()                      │
                    │       │ clientWriter.WriteMessage()                   │
                    │       │ (FORWARD NGUYÊN VẸN, không modify)           │
                    └───────┬──────────────────────────────────────────────┘
                            │  WebSocket message
                            ▼
                    ┌──────────────────────────────────────────────────────┐
                    │              CLIENT (viem/ethers)                     │
                    │                                                      │
                    │  Nhận eth_subscription message                       │
                    │  Gọi callback với event log data                     │
                    └──────────────────────────────────────────────────────┘
```

## So sánh: Interceptor vs Non-Interceptor

| Tiêu chí | Interceptor (/interceptor) | Non-Interceptor (/) |
|----------|---------------------------|---------------------|
| **Ai tạo event?** | RPC handler tự tạo (fake) | Chain tạo từ receipt EventLogs |
| **Đi qua chain?** | KHÔNG | CÓ |
| **Subscribe lưu ở đâu?** | SubscriptionInterceptor (RPC) | EventSystem (Chain) |
| **Event format?** | Handler pack bằng ABI | Chain tạo từ MVM execution |
| **Broadcast bởi ai?** | SubInterceptor.BroadcastEventToContract() | notifier.Notify() (go-ethereum/rpc) |
| **RPC proxy làm gì?** | Chặn subscribe + tự broadcast | Forward 2 chiều (passthrough) |

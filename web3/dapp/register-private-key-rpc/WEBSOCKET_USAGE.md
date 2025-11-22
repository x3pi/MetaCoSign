# WebSocket Global Usage Guide

## Tổng quan

Hệ thống WebSocket global đã được tạo để sử dụng lại ở nhiều nơi trong ứng dụng. WebSocket connection được quản lý tập trung thông qua `WebSocketContext`.

## Cấu trúc

### 1. WebSocketContext (`src/contexts/WebSocketContext.tsx`)

**Chức năng:**
- Quản lý WebSocket connection global
- Auto-reconnect với exponential backoff (tối đa 5 lần, delay tối đa 30s)
- Hỗ trợ subscribe/unsubscribe cho nhiều listeners
- Broadcast messages tới tất cả subscribers

**API:**
```typescript
interface WebSocketContextType {
  ws: WebSocket | null;           // WebSocket instance
  isConnected: boolean;            // Connection status
  subscribe: (topic, callback) => unsubscribe;  // Subscribe to messages
  send: (data) => void;            // Send message
}
```

### 2. NotificationContext (`src/contexts/NotificationContext.tsx`)

**Chức năng:**
- Sử dụng WebSocket global để lắng nghe event `AccountConfirmed`
- Load notifications từ smart contract
- Quản lý unread count
- Hiển thị browser notifications

**Notification Fields (từ `types/notification.ts`):**
```typescript
interface Notification {
  id: string;        // Unique ID
  account: string;   // Account address
  time: number;      // Unix timestamp
  message: string;   // Notification message
}
```

## Cách sử dụng

### Setup trong App.tsx

```tsx
import { WebSocketProvider } from "./contexts/WebSocketContext";
import { NotificationProvider } from "./contexts/NotificationContext";

function App() {
  const wsUrl = "ws://192.168.1.234:8546"; // WebSocket endpoint
  
  return (
    <ThemeProvider>
      <WebSocketProvider url={wsUrl}>
        <NotificationProvider>
          {/* Your app components */}
        </NotificationProvider>
      </WebSocketProvider>
    </ThemeProvider>
  );
}
```

### Sử dụng WebSocket trong component

```tsx
import { useWebSocket } from "~/contexts/WebSocketContext";

function MyComponent() {
  const { isConnected, subscribe, send } = useWebSocket();
  
  useEffect(() => {
    // Subscribe to messages
    const unsubscribe = subscribe("my_topic", (data) => {
      console.log("Received:", data);
    });
    
    // Cleanup
    return () => unsubscribe();
  }, [subscribe]);
  
  const sendMessage = () => {
    send({
      jsonrpc: "2.0",
      method: "eth_subscribe",
      params: ["logs", { address: "0x..." }]
    });
  };
  
  return <div>Connected: {isConnected ? "Yes" : "No"}</div>;
}
```

### Sử dụng Notifications

```tsx
import { useNotifications } from "~/contexts/NotificationContext";

function MyComponent() {
  const {
    notifications,
    unreadCount,
    markAsRead,
    markAllAsRead,
    loadNotifications
  } = useNotifications();
  
  return (
    <div>
      <p>Unread: {unreadCount}</p>
      {notifications.map(notif => (
        <div key={notif.id} onClick={() => markAsRead(notif.id)}>
          <p>{notif.message}</p>
          <p>{notif.account}</p>
          <p>{new Date(notif.time * 1000).toLocaleString()}</p>
        </div>
      ))}
    </div>
  );
}
```

## Event Listening

### AccountConfirmed Event

**Event signature:** `AccountConfirmed(address,uint256,string)`

**Topic hash:** Được tính bằng `keccak256(toHex("AccountConfirmed(address,uint256,string)"))`

**Subscription:**
```javascript
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "eth_subscribe",
  "params": [
    "logs",
    {
      "address": "0xContractAddress",
      "topics": ["0xEventTopicHash"]
    }
  ]
}
```

**Response:**
```javascript
{
  "method": "eth_subscription",
  "params": {
    "result": {
      "topics": [
        "0xEventSignature",
        "0x000...AddressInHex"  // indexed address parameter
      ],
      "data": "0x..."  // non-indexed parameters (time, message)
    }
  }
}
```

## Tạo thêm Event Listeners

Để lắng nghe event khác, tạo custom hook:

```tsx
// hooks/useMyEventListener.ts
import { useEffect } from "react";
import { useWebSocket } from "~/contexts/WebSocketContext";
import { keccak256, toHex } from "viem";

export function useMyEventListener(contractAddress: string, onEvent: (data: any) => void) {
  const { subscribe, send, isConnected } = useWebSocket();
  
  useEffect(() => {
    if (!isConnected) return;
    
    // Calculate event topic
    const eventSignature = "MyEvent(address,uint256)";
    const eventTopic = keccak256(toHex(eventSignature));
    
    // Subscribe
    send({
      jsonrpc: "2.0",
      id: Date.now(),
      method: "eth_subscribe",
      params: [
        "logs",
        {
          address: contractAddress,
          topics: [eventTopic]
        }
      ]
    });
    
    // Listen for events
    const unsubscribe = subscribe("my_event", (data: unknown) => {
      const message = data as { method?: string; params?: { result?: any } };
      
      if (message.method === "eth_subscription" && message.params?.result) {
        onEvent(message.params.result);
      }
    });
    
    return () => unsubscribe();
  }, [contractAddress, isConnected, onEvent, subscribe, send]);
}
```

## Cấu hình

### WebSocket URL

Thay đổi URL trong `App.tsx`:

```tsx
const wsUrl = process.env.VITE_WS_URL || "ws://192.168.1.234:8546";
```

Hoặc thêm vào `.env`:
```
VITE_WS_URL=ws://192.168.1.234:8546
```

### Reconnect Settings

Trong `WebSocketContext.tsx`:
```typescript
const maxReconnectAttempts = 5;  // Số lần retry
const delay = Math.min(1000 * Math.pow(2, attempts), 30000);  // Max 30s
```

## Lưu ý

1. **Browser Notification Permission:** Ứng dụng sẽ tự động request permission khi mount NotificationProvider
2. **WebSocket Support:** Backend phải hỗ trợ `eth_subscribe` method
3. **Event Topic:** Phải tính chính xác hash của event signature
4. **Notification Fields:** Chỉ sử dụng các field có trong `types/notification.ts`
5. **Auto-reconnect:** WebSocket sẽ tự động kết nối lại khi mất kết nối

## Debugging

Kiểm tra WebSocket connection:
```javascript
// Browser console
console.log(ws);  // Check WebSocket instance
ws.readyState;    // 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED
```

Log WebSocket messages:
- Tất cả messages được log trong console với prefix "WebSocket message received:"
- Subscription requests được log với "Subscribing to..."
- Connection status được log với "WebSocket connected/disconnected"

# Transaction List Feature - Implementation Guide

## 📊 Overview

Đã thêm tính năng xem giao dịch của một account (address) với:
- ✅ Danh sách giao dịch phân trang (pagination)
- ✅ Modal hiển thị chi tiết từng giao dịch
- ✅ Tích hợp với BLS Account List page
- ✅ Giao diện đẹp sử dụng shadcn components

---

## 🎯 User Flow

```
┌─────────────────────────────────────┐
│   BLS Account List Page             │
│  (Danh sách các accounts)           │
│                                     │
│  Account 1  [Confirm] [View TX] ← Click "View TX"
│  Account 2  [Confirm] [View TX]
│  Account 3  [Confirm] [View TX]
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│   Transaction List Modal (Overlay)  │
│  (Danh sách giao dịch của account)  │
│                                     │
│  TX 1  [View Details]               │
│  TX 2  [View Details]
│  TX 3  [View Details]
│                                     │
│  [Previous] Page 1/5 [Next]         │
│                      [Close]        │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│   Transaction Detail Modal          │
│  (Chi tiết đầy đủ của 1 TX)         │
│                                     │
│  Hash: 0xabcd1234...                │
│  Block: 5                           │
│  From: 0x111...                     │
│  To: 0x222...                       │
│  Value: 1.0 ETH                     │
│  Status: SUCCESS                    │
│  ...                                │
│                      [Close] (✕)    │
└─────────────────────────────────────┘
```

---

## 📁 Files Created/Modified

### 1. **New Type Definition**
📄 `/src/types/transaction.ts`
```typescript
export interface Transaction {
  blockHash: string;
  blockNumber: number;
  chainId: number;
  data: string;
  exception: string;
  from: string;
  gas: number;
  gasFee: number;
  gasPrice: number;
  gasUsed: number;
  hash: string;
  logs: Record<string, unknown>[];
  nonce: number;
  r: string;
  rHash: string;
  returnValue: string;
  s: string;
  status: string;
  timestamp: number;
  to: string;
  transactionIndex: number;
  v: string;
  value: string;
}

export interface TransactionResponse {
  total: number;
  transactions: Transaction[];
}
```

### 2. **Transaction List Page**
📄 `/src/pages/TransactionList/TransactionListPage.tsx`

**Key Features:**
- Fetch transactions từ RPC endpoint
- Pagination với Previous/Next buttons
- List view với các thông tin chính:
  - Transaction hash (truncated)
  - To address
  - Value (ETH)
  - Block number
  - Timestamp
  - Status badge

**Code Structure:**
```typescript
interface TransactionListPageProps {
  address: string;              // Address để query
  onClose?: () => void;         // Callback khi đóng modal
}

export function TransactionListPage({
  address,
  onClose,
}: TransactionListPageProps) {
  // States
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [page, setPage] = useState(0);
  const [total, setTotal] = useState(0);
  const [selectedTx, setSelectedTx] = useState<Transaction | null>(null);
  
  // Load transactions từ RPC
  const loadTransactions = useCallback(async () => {
    const response = await fetch("http://192.168.1.234:8545", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "mtn_searchTransactions",
        params: [`from:${address}`, page, pageSize],
        id: 1,
      }),
    });
    // Process response...
  }, [address, page, pageSize]);
}
```

### 3. **BLS Account List Page (Updated)**
📄 `/src/pages/BlsAccountList/BlsAccountListPage.tsx`

**Changes:**
```typescript
// State thêm mới
const [showTransactionList, setShowTransactionList] = useState(false);
const [selectedAddressForTx, setSelectedAddressForTx] = useState<string>("");

// Button thêm mới
<Button
  onClick={() => {
    setShowTransactionList(true);
    setSelectedAddressForTx(bytesToAddress(account.address));
  }}
  size="sm"
  variant="outline"
>
  View TX
</Button>

// Modal render
{showTransactionList && selectedAddressForTx && (
  <div className="fixed inset-0 z-40 bg-black/50">
    <div className="h-full overflow-auto">
      <TransactionListPage
        address={selectedAddressForTx}
        onClose={() => {
          setShowTransactionList(false);
          setSelectedAddressForTx("");
        }}
      />
    </div>
  </div>
)}
```

---

## 🎨 UI Layout

### BLS Account List Page
```
┌─ BLS Account Management ─────────────────────────┐
│                                                  │
│ [Unconfirmed (5)] [Confirmed (10)]              │
│                                                  │
│ ┌─ Account Item ────────────────────────────┐   │
│ │ 0xabcd...ef01  [Pending]                  │   │
│ │                                           │   │
│ │ Registered: 2024-11-19 10:30              │   │
│ │ Register TX: 0xabcd...                    │   │
│ │                                           │   │
│ │ [Confirm]  [View TX] ← New Button!       │   │
│ └───────────────────────────────────────────┘   │
│                                                  │
│ [Refresh]                                       │
│                                                  │
└──────────────────────────────────────────────────┘
```

### Transaction List Modal
```
┌─ Transaction History ────────────────────────────┐
│ Transactions from 0xabcd...ef01                 │
│ [Refresh]                            [Close]    │
│                                                  │
│ ┌─ Transaction Item ───────────────────────┐   │
│ │ 0xba47a9b91...ccdcc  [SUCCESS]          │   │
│ │                                         │   │
│ │ To: 0x0572...3c6                        │   │
│ │ Value: 1.0000 ETH                       │   │
│ │ Block: 5                                │   │
│ │ Time: 2024-11-19 10:30                  │   │
│ │                   [View Details]        │   │
│ └─────────────────────────────────────────┘   │
│                                                  │
│ ┌─ Transaction Item ───────────────────────┐   │
│ │ 0xf9bad9f8...a1030b  [SUCCESS]          │   │
│ │                                         │   │
│ │ To: 0x0Ec0...aae                        │   │
│ │ Value: 1.0000 ETH                       │   │
│ │ Block: 3                                │   │
│ │ Time: 2024-11-19 10:28                  │   │
│ │                   [View Details]        │   │
│ └─────────────────────────────────────────┘   │
│                                                  │
│ [Previous] Page 1 of 5 (10 total) [Next]       │
│                                                  │
└──────────────────────────────────────────────────┘
```

### Transaction Detail Modal
```
┌─ Transaction Details ────────────────────────────┐
│ Transaction Hash                          [✕]   │
│ 0xba47a9b91037572521cfca18d789aefeb6e45752...  │
│                                                  │
│ Block Number: 5  │  Transaction Index: 0       │
│                                                  │
│ From:                                          │
│ 0x0B143e894a600114C4A3729874214e5fC5EA9cbc    │
│                                                  │
│ To:                                            │
│ 0x0572E49F9902721fE24C9D50Af2F8c573677f3c6    │
│                                                  │
│ Value (ETH): 1.0000      │  Gas Used: 21000000000
│                                                  │
│ Status: [SUCCESS]  │  Chain ID: 991            │
│                                                  │
│ Timestamp:                                     │
│ 2024-11-19 10:30:52                           │
│                                                  │
│ Input Data:                                    │
│ 0x                                             │
│                                                  │
└──────────────────────────────────────────────────┘
```

---

## 🔄 Data Flow

### 1. Load Transactions
```
User clicks "View TX"
    ↓
setShowTransactionList(true)
setSelectedAddressForTx(address)
    ↓
<TransactionListPage address={address} />
    ↓
loadTransactions() useEffect
    ↓
fetch("http://192.168.1.234:8545") {
  method: "mtn_searchTransactions"
  params: ["from:0xabcd...", page, pageSize]
}
    ↓
Parse JSON response
    ↓
setTransactions(data.transactions)
setTotal(data.total)
    ↓
Render transaction list
```

### 2. View Transaction Detail
```
User clicks "View Details" on transaction
    ↓
setSelectedTx(transaction)
    ↓
Render detail modal
    ↓
User clicks [✕] or click outside
    ↓
setSelectedTx(null)
    ↓
Modal closes
```

---

## 🚀 Usage

### Start Feature
```tsx
// BLS Account List Page sẽ có nút "View TX" cho mỗi account
// Click nút đó để mở Transaction List

<Button
  onClick={() => {
    setShowTransactionList(true);
    setSelectedAddressForTx(address);
  }}
  size="sm"
  variant="outline"
>
  View TX
</Button>
```

### Customize RPC Endpoint
Nếu RPC endpoint thay đổi, update tại:
```typescript
// File: /src/pages/TransactionList/TransactionListPage.tsx
const response = await fetch("http://192.168.1.234:8545", {
  // ↑ Change this URL
  method: "POST",
  // ...
});
```

---

## 🔧 Configuration

### Page Size
```typescript
// File: TransactionListPage.tsx
const [pageSize] = useState(10);  // Transactions per page
```

Để thay đổi số transaction mỗi trang, sửa value này.

### RPC Method
```typescript
method: "mtn_searchTransactions",  // Custom RPC method
params: [`from:${address}`, page, pageSize],
```

Đảm bảo RPC server của bạn support `mtn_searchTransactions` method.

---

## 🎯 Features

| Feature | Status | Description |
|---------|--------|-------------|
| List transactions | ✅ | Fetch và hiển thị danh sách giao dịch |
| Pagination | ✅ | Previous/Next buttons với page info |
| Status badge | ✅ | Màu khác nhau cho SUCCESS/FAILED |
| Detail modal | ✅ | Click "View Details" để xem chi tiết |
| Format data | ✅ | Value (ETH), Timestamp, Address truncate |
| Search | ❌ | Có thể thêm sau |
| Filter | ❌ | Có thể thêm status/value filter |
| Export | ❌ | Có thể thêm export CSV |

---

## 💡 Possible Enhancements

1. **Add search/filter**
```tsx
const [filterStatus, setFilterStatus] = useState<"all" | "success" | "failed">("all");
const filteredTx = transactions.filter(tx => {
  if (filterStatus === "all") return true;
  return tx.status.toUpperCase() === filterStatus.toUpperCase();
});
```

2. **Copy to clipboard**
```tsx
const copyToClipboard = (text: string) => {
  navigator.clipboard.writeText(text);
  // Show toast notification
};

<button onClick={() => copyToClipboard(tx.hash)}>
  📋 Copy
</button>
```

3. **Export transactions**
```tsx
const exportToCSV = () => {
  const csv = transactions.map(tx => `${tx.hash},${tx.from},${tx.to},${tx.value}`);
  const blob = new Blob([csv.join('\n')]);
  // Download blob
};
```

4. **Sort transactions**
```tsx
const [sortBy, setSortBy] = useState<"time" | "value">("time");
const sortedTx = [...transactions].sort((a, b) => {
  if (sortBy === "time") return b.timestamp - a.timestamp;
  return BigInt(b.value) - BigInt(a.value);
});
```

---

## 🐛 Troubleshooting

### "Cannot find module" error
```
Fix: Make sure type file exists
  /src/types/transaction.ts
```

### RPC method not found
```
Error: mtn_searchTransactions not supported
Fix: Check RPC server supports this custom method
  Or implement server-side support
```

### Modal not showing
```
Fix: Check showTransactionList state is true
  And selectedAddressForTx is not empty
```

---

## 📝 Summary

✅ **Created:**
- TransactionListPage component - hiển thị danh sách phân trang
- Transaction detail modal - xem chi tiết từng giao dịch
- Type definitions - TypeScript interfaces

✅ **Updated:**
- BLS Account List Page - thêm "View TX" button

✅ **Features:**
- Pagination support (Previous/Next)
- Status badge (colored)
- Data formatting (ETH value, timestamps)
- Modal overlay interaction
- Responsive design using shadcn components

Bây giờ bạn có một hệ thống hoàn chỉnh để xem giao dịch của mỗi account! 🎉

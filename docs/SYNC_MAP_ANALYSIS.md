# Phân tích sync.Map vs RWMutex + map

## So sánh

### sync.Map
**Ưu điểm:**
- ✅ **Non-blocking reads** - Không có lock contention trên reads
- ✅ **Tốt cho read-heavy workloads** - Nhiều reads, ít writes
- ✅ **Thread-safe** - Built-in thread safety
- ✅ **Không cần lock** - Reads không block nhau

**Nhược điểm:**
- ❌ **Type assertion** - Cần type assertion khi load
- ❌ **Không có len()** - Phải dùng Range() để đếm
- ❌ **Memory overhead** - Nhiều memory hơn một chút
- ❌ **Không có iteration order** - Range() không đảm bảo thứ tự

### RWMutex + map
**Ưu điểm:**
- ✅ **Type-safe** - Compile-time type checking
- ✅ **Có len()** - Dễ đếm số phần tử
- ✅ **Memory efficient** - Ít memory hơn
- ✅ **Có iteration order** - Có thể control thứ tự

**Nhược điểm:**
- ❌ **Lock contention** - Reads có thể block nhau khi có write
- ❌ **RWMutex overhead** - Có overhead cho lock/unlock
- ❌ **Write blocks reads** - Write lock block tất cả reads

## Khi nào dùng sync.Map?

### ✅ Nên dùng sync.Map khi:
1. **Read-heavy workloads** (> 90% reads)
2. **Nhiều goroutines đọc đồng thời**
3. **Writes ít và rải rác**
4. **Không cần iteration order**
5. **Không cần len() thường xuyên**

### ❌ Không nên dùng sync.Map khi:
1. **Write-heavy workloads** (> 50% writes)
2. **Cần iteration order**
3. **Cần len() thường xuyên**
4. **Cần type safety tại compile-time**

## Benchmark (ước tính)

| Operation | sync.Map | RWMutex + map |
|-----------|----------|---------------|
| Read (1 goroutine) | ~50ns | ~100ns |
| Read (100 goroutines) | ~50ns | ~500ns+ (contention) |
| Write | ~200ns | ~150ns |
| Range | ~O(n) | ~O(n) |

## Kết luận cho ConnectionsManager

**Hiện tại:** RWMutex + map với sharding
- Đã tốt với sharding (32 shards)
- Lock contention đã được giảm

**sync.Map sẽ tốt hơn nếu:**
- Có > 1000 connections/shard
- Read-heavy (nhiều lookups, ít adds)
- Cần non-blocking reads tuyệt đối

**Khuyến nghị:** 
- Giữ nguyên RWMutex + map nếu < 1000 connections/shard
- Chuyển sang sync.Map nếu > 1000 connections/shard hoặc cần non-blocking reads tuyệt đối


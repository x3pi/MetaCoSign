# Tài liệu Smart Contract AccountManager

Tài liệu này mô tả các sự kiện (Events) và các hàm (Functions) trong Smart Contract `AccountManager`. Contract này quản lý việc đăng ký khóa BLS, xác thực tài khoản và quản lý trạng thái người dùng.

## 1. Events (Sự kiện)

Các sự kiện được emit ra blockchain để frontend hoặc các dịch vụ backend lắng nghe và xử lý.

### `RegisterBls`
Được bắn ra khi người dùng gọi hàm đăng ký khóa BLS. Admin lắng nghe sự kiện này để biết có yêu cầu đăng ký mới cần duyệt.

**Parameters:**
- `account` (`address`): Địa chỉ ví của người dùng thực hiện đăng ký.
- `time` (`uint`): Thời điểm gửi yêu cầu (timestamp).
- `publicKey` (`bytes`): Chuỗi bytes của khóa công khai BLS mà người dùng muốn đăng ký.
- `message` (`string`): Thông điệp kèm theo hoặc ghi chú hệ thống (nếu có).

---

### `AccountConfirmed`
Được bắn ra khi Admin chấp nhận yêu cầu đăng ký. Frontend người dùng lắng nghe sự kiện này để cập nhật trạng thái giao diện.

**Parameters:**
- `account` (`address`): Địa chỉ ví của người dùng vừa được duyệt.
- `time` (`uint`): Thời điểm được duyệt (timestamp).
- `message` (`string`): Thông báo xác nhận (ví dụ: "Registration Successful").

---

## 2. Functions (Hàm)

### `setBlsPublicKey`
Đăng ký khóa công khai BLS lên hệ thống.

```solidity
function setBlsPublicKey(bytes memory _publicKey) external
```

**Parameters:**
- `_publicKey` (`bytes`): Khóa công khai BLS của người dùng cần đăng ký.

---

### `setAccountType`
Cài đặt kiểu xác thực chữ ký cho tài khoản.

```solidity
function setAccountType(uint8 _type) external
```

**Parameters:**
- `_type` (`uint8`): Loại cấu hình chữ ký.
  - `0`: Sử dụng 1 chữ ký (Single Signature).
  - `1`: Sử dụng 2 chữ ký (Dual/Multi Signature).

---

### `getAllAccount`
Admin lấy danh sách tài khoản (hỗ trợ lọc và phân trang).

```solidity
function getAllAccount(
    bytes memory _sign, 
    bytes memory _publicKeyBls, 
    uint _time, 
    uint _page, 
    uint _pageSize, 
    bool _isConfirm
) external
```

**Parameters:**
- `_sign` (`bytes`): Chữ ký xác thực quyền của Admin.
- `_publicKeyBls` (`bytes`): (Tùy chọn) Dùng để tìm kiếm theo khóa BLS cụ thể.
- `_time` (`uint`): Thời gian thực hiện request (dùng để chống tấn công replay).
- `_page` (`uint`): Số thứ tự trang hiện tại (bắt đầu từ 1).
- `_pageSize` (`uint`): Số lượng tài khoản hiển thị trên một trang.
- `_isConfirm` (`bool`): Trạng thái lọc danh sách.
  - `false`: Lấy danh sách đang chờ duyệt (Pending).
  - `true`: Lấy danh sách đã được duyệt (Approved).

---

### `confirmAccount`
Admin xác nhận duyệt đăng ký cho một tài khoản cụ thể.

```solidity
function confirmAccount(address _account, uint time, bytes memory _sign) external
```

**Parameters:**
- `_account` (`address`): Địa chỉ ví của người dùng cần xác nhận.
- `time` (`uint`): Thời điểm xác nhận.
- `_sign` (`bytes`): Chữ ký xác thực của Admin để cấp quyền duyệt.

---

### `getNotifications`
Lấy danh sách thông báo của người dùng.

```solidity
function getNotifications(address _account, uint page, uint pageSize) external
```

**Parameters:**
- `_account` (`address`): Địa chỉ ví người dùng muốn xem thông báo.
- `page` (`uint`): Số thứ tự trang cần xem.
- `pageSize` (`uint`): Số lượng thông báo trên mỗi trang.

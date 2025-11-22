# AccountManager Smart Contract Documentation

Tài liệu kỹ thuật cho Smart Contract `AccountManager`. Contract này quản lý việc đăng ký khóa BLS, cấu hình loại chữ ký của tài khoản và quy trình phê duyệt từ Admin.

---

## 1. Events (Sự kiện)

Các sự kiện được emit để các dịch vụ bên ngoài (Backend/Frontend) lắng nghe và xử lý.

### `RegisterBls`
Sự kiện này được bắn ra khi người dùng gọi hàm đăng ký BLS. Admin sẽ lắng nghe sự kiện này để nhận biết và duyệt yêu cầu.

| Tham số | Kiểu dữ liệu | Mô tả |
| :--- | :--- | :--- |
| `account` | `address` | Địa chỉ ví của người dùng thực hiện đăng ký. |
| `time` | `uint` | Thời điểm gửi yêu cầu (timestamp). |
| `publicKey` | `bytes` | Chuỗi bytes của khóa công khai BLS người dùng đăng ký. |
| `message` | `string` | Thông điệp hệ thống hoặc ghi chú kèm theo. |

### `AccountConfirmed`
Sự kiện này được bắn ra khi Admin chấp nhận yêu cầu đăng ký. Frontend người dùng lắng nghe để cập nhật trạng thái hiển thị.

| Tham số | Kiểu dữ liệu | Mô tả |
| :--- | :--- | :--- |
| `account` | `address` | Địa chỉ ví của người dùng vừa được duyệt. |
| `time` | `uint` | Thời điểm được duyệt (timestamp). |
| `message` | `string` | Thông báo xác nhận (VD: "Registration Successful"). |

---

## 2. User Functions (Chức năng người dùng)

### `setBlsPublicKey`
Người dùng gọi hàm này để gửi khóa công khai BLS lên hệ thống chờ duyệt.

```solidity
function setBlsPublicKey(bytes memory _publicKey) external
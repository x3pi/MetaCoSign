Những thay đổi
thêm cấu hình  trong `config-client-tcp.json`  
- **`extra_account`**: Số tiền gửi thêm khi user k đủ tiền giao dịch trên chain .
- **`disable_free_gas`**: user tự trả phí nếu k đủ gas.

Hoạt động trả phí gas:
Khi tài khoản user có số dư 0.001 MTN thì sẻ chuyển 1 lượng `extra_account` cho user
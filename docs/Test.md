1. Địa chỉ ngắn hơn hoặc dài hơn 20 byte thì đang được thêm số 0 phía trước hoặc là cắt lấy 20 byte phía sau. Cái này chain qua bước xử lý proto nữa nên nếu proto không giải mã được thì giao dịch gửi lên lỗi còn giải mã được thì sẽ luôn nhận được 20 byte đang phụ thuộc vào qúa trình xử lý proto client gửi lên

2. Xem lại giới hạn dung lượng `Call data`
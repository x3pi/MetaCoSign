#!/bin/bash
SESSION_NAME="go_servers"
LOG_DIR="log"

echo "🚀 Chuẩn bị khởi động session tmux '$SESSION_NAME'..."

# Nếu session cũ tồn tại thì kill
tmux has-session -t $SESSION_NAME 2>/dev/null
if [ $? == 0 ]; then
  echo "🛑 Phát hiện session cũ '$SESSION_NAME' — đang dừng..."
  tmux kill-session -t $SESSION_NAME
  echo "✅ Đã dừng session cũ."
fi

# Tạo thư mục log nếu chưa tồn tại
mkdir -p $LOG_DIR
echo "📁 Thư mục log: '$LOG_DIR' đã sẵn sàng."

# Xóa log cũ
echo "🧹 Đang xóa log cũ..."
rm -f $LOG_DIR/*.log
echo "✅ Đã xóa toàn bộ log cũ trong '$LOG_DIR'."

# Tạo session mới và chạy 3 server
echo "⚙️  Đang tạo session tmux mới tên là '$SESSION_NAME'..."

tmux new-session -d -s $SESSION_NAME -n server1 \
  "go run . -envfile=.env.1 2>&1 | tee $LOG_DIR/server1.log"

tmux new-window -t $SESSION_NAME: -n server2 \
  "go run . -envfile=.env.2 2>&1 | tee $LOG_DIR/server2.log"

tmux new-window -t $SESSION_NAME: -n server3 \
  "go run . -envfile=.env.3 2>&1 | tee $LOG_DIR/server3.log"

echo "✅ Đã khởi động 3 server trong session tmux '$SESSION_NAME'."
echo "📄 Log đang được ghi trong thư mục '$LOG_DIR'."
echo "💡 Dùng lệnh sau để xem session:"
echo "   tmux attach -t $SESSION_NAME"

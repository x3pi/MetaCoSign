#!/bin/bash
SESSION_NAME="go_servers"
LOG_DIR="log"

echo "🛑 Đang kiểm tra session '$SESSION_NAME'..."

# Kiểm tra session có tồn tại không
tmux has-session -t $SESSION_NAME 2>/dev/null
if [ $? != 0 ]; then
  echo "⚠️  Không tìm thấy session '$SESSION_NAME' đang chạy."
  exit 0
fi

# Kill session
echo "💥 Đang dừng session '$SESSION_NAME'..."
tmux kill-session -t $SESSION_NAME
echo "✅ Session '$SESSION_NAME' đã được dừng."

# Hỏi người dùng có muốn xóa log không
read -p "🧹 Bạn có muốn xóa toàn bộ log trong '$LOG_DIR'? (y/N): " confirm
if [[ "$confirm" =~ ^[Yy]$ ]]; then
  rm -f $LOG_DIR/*.log
  echo "🗑️  Đã xóa toàn bộ file log trong '$LOG_DIR'."
else
  echo "📄 Giữ nguyên log trong '$LOG_DIR'."
fi

echo "✅ Hoàn tất việc dừng toàn bộ servers."

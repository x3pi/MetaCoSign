#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
# Kiểm tra nếu chạy thủ công từ terminal (không phải do systemd tự gọi)
if [ -z "$INVOCATION_ID" ]; then
    RUNNING_PID=$(lsof -t privatekey_db_encrypted/LOCK 2>/dev/null || fuser privatekey_db_encrypted/LOCK 2>/dev/null || lsof -t -i :8545 2>/dev/null || true)
    RUNNING_PID=$(echo "$RUNNING_PID" | tr -s ' ' '\n' | grep -v '^$' | head -n 1)

    IS_SYSTEMD=false
    if systemctl --user is-active --quiet metacosign.service 2>/dev/null; then
        IS_SYSTEMD=true
    fi

    if [ -n "$RUNNING_PID" ] || [ "$IS_SYSTEMD" = true ]; then
        echo -e "\033[1;33m==============================================================\033[0m"
        echo -e "\033[1;31m⚠️  CẢNH BÁO: Đang có một tiến trình RPC Client đang chạy!\033[0m"
        [ -n "$RUNNING_PID" ] && echo -e "📌 PID đang chạy: \033[1;36m$RUNNING_PID\033[0m"
        
        if [ "$IS_SYSTEMD" = true ]; then
            echo -e "📌 Tiến trình đang chạy ngầm qua service: \033[1;32mmetacosign.service\033[0m"
            echo -e "\033[1;33m--------------------------------------------------------------\033[0m"
            echo -e "👉 Xem log đang chạy:      \033[1;37mjournalctl --user -u metacosign.service -f\033[0m"
            echo -e "👉 Xem trạng thái service: \033[1;37msystemctl --user status metacosign.service\033[0m"
            echo -e "👉 Dừng service nếu muốn chạy tay: \033[1;37msystemctl --user stop metacosign.service\033[0m"
        else
            echo -e "\033[1;33m--------------------------------------------------------------\033[0m"
            echo -e "👉 Dừng tiến trình cũ bằng lệnh: \033[1;37mkill $RUNNING_PID\033[0m"
        fi
        echo -e "\033[1;33m==============================================================\033[0m"

        if [ -t 0 ]; then
            echo ""
            read -r -p "Bạn có muốn dừng tiến trình cũ để chạy ở terminal này không? [y/N]: " choice
            case "$choice" in
                [yY][eE][sS]|[yY])
                    if [ "$IS_SYSTEMD" = true ]; then
                        echo "Đang dừng metacosign.service..."
                        systemctl --user stop metacosign.service
                    elif [ -n "$RUNNING_PID" ]; then
                        echo "Đang dừng PID $RUNNING_PID..."
                        kill "$RUNNING_PID" 2>/dev/null || true
                    fi
                    sleep 1
                    ;;
                *)
                    echo "Đã hủy bỏ chạy run.sh."
                    exit 1
                    ;;
            esac
        else
            exit 1
        fi
    fi
fi

export PATH="/usr/local/go/bin:$PATH"
export GOTOOLCHAIN="go1.23.5+auto"
export CGO_CFLAGS="-std=gnu17"

exec /usr/local/go/bin/go run . "$@"
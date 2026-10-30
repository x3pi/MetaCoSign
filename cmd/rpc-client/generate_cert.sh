#!/usr/bin/env bash
set -e

# Xác định thư mục chứa script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

CERT_FILE="certificate.pem"
KEY_FILE="private.key"

FORCE=false
if [ "$1" == "--force" ] || [ "$1" == "-f" ]; then
    FORCE=true
fi

if [ -f "$CERT_FILE" ] && [ -f "$KEY_FILE" ] && [ "$FORCE" = false ]; then
    echo "✅ [OK] Chứng chỉ TLS đã tồn tại:"
    echo "   - Certificate: $SCRIPT_DIR/$CERT_FILE"
    echo "   - Private Key: $SCRIPT_DIR/$KEY_FILE"
    echo "👉 Chạy './generate_cert.sh --force' nếu muốn tạo lại mới."
    exit 0
fi

echo "🔐 Đang tạo mới chứng chỉ TLS (Self-signed Certificate & Private Key)..."

# Lấy IP local tự động để thêm vào SAN
LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
SAN_EXT="DNS:localhost,IP:127.0.0.1,IP:0.0.0.0"
if [ -n "$LOCAL_IP" ]; then
    SAN_EXT="$SAN_EXT,IP:$LOCAL_IP"
fi

openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$KEY_FILE" \
    -out "$CERT_FILE" \
    -days 3650 \
    -subj "/C=VN/ST=Hanoi/L=Hanoi/O=MetaNode/OU=RPC/CN=localhost" \
    -addext "subjectAltName=$SAN_EXT"

chmod 600 "$KEY_FILE"
chmod 644 "$CERT_FILE"

echo "🎉 Tạo chứng chỉ TLS thành công!"
echo "   - Certificate : $SCRIPT_DIR/$CERT_FILE"
echo "   - Private Key : $SCRIPT_DIR/$KEY_FILE"
echo "   - SAN (IP/DNS): $SAN_EXT"
echo "   - Hạn dùng    : 10 năm (3650 ngày)"

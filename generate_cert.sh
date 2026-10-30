#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Chạy tạo cert trong cmd/rpc-client
"$SCRIPT_DIR/cmd/rpc-client/generate_cert.sh" "$@"

# Tạo symlink ở thư mục gốc metaCoSign nếu chưa có
cd "$SCRIPT_DIR"
if [ ! -e "certificate.pem" ]; then
    ln -sf cmd/rpc-client/certificate.pem certificate.pem
fi
if [ ! -e "private.key" ]; then
    ln -sf cmd/rpc-client/private.key private.key
fi

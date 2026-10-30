#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Chạy tạo certificate.pem & private.key trực tiếp bên trong cmd/rpc-client
exec "$SCRIPT_DIR/cmd/rpc-client/generate_cert.sh" "$@"

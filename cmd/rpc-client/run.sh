#!/usr/bin/env bash
set -e

# Tự động tạo certificate.pem & private.key nếu chưa có
./generate_cert.sh

export GOTOOLCHAIN=go1.23.5+auto
export CGO_CFLAGS="-std=gnu17"
go run main.go
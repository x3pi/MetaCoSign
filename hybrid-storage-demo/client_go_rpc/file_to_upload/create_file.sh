#!/usr/bin/env bash
set -euo pipefail

# Tên file đầu ra (mặc định output.txt) - có thể truyền tên file như tham số đầu tiên
OUT="${1:-output.txt}"
SIZE_MB=2000 # Kích thước file mong muốn test
TARGET_BYTES=$((SIZE_MB * 1024 * 1024 ))

echo "Tạo file '${OUT}' kích thước ${SIZE_MB}MB (~${TARGET_BYTES} bytes)..."

# Ưu tiên tạo file text có ký tự in được: dùng `yes` (nhiều hệ thống có sẵn)
if command -v yes >/dev/null 2>&1 && command -v head >/dev/null 2>&1; then
  yes "This is a sample text line for filling the file." | head -c "${TARGET_BYTES}" > "${OUT}"
  echo "Hoàn tất (đã dùng yes | head)."
  exit 0
fi

# Nếu fallocate có sẵn (tạo file nhanh, nhưng có thể không ghi ký tự in được)
if command -v fallocate >/dev/null 2>&1; then
  fallocate -l "${SIZE_MB}M" "${OUT}"
  echo "Hoàn tất (đã dùng fallocate)."
  exit 0
fi

# Nếu dd có sẵn, dùng dd từ /dev/zero (có thể chứa byte null, vẫn là file .txt nhưng không in đẹp)
if command -v dd >/dev/null 2>&1; then
  dd if=/dev/zero of="${OUT}" bs=1M count="${SIZE_MB}" status=none
  echo "Hoàn tất (đã dùng dd from /dev/zero)."
  exit 0
fi

# Dự phòng: dùng head từ /dev/zero nếu có
if command -v head >/dev/null 2>&1; then
  head -c "${TARGET_BYTES}" </dev/zero > "${OUT}"
  echo "Hoàn tất (đã dùng head từ /dev/zero)."
  exit 0
fi

echo "Lỗi: Không tìm thấy công cụ phù hợp (yes, fallocate, dd hoặc head)."
exit 1

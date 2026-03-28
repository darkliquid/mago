#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUT_PATH=${1:-"$ROOT_DIR/native/libminiaudio.so"}
CC_BIN=${CC:-cc}

mkdir -p "$(dirname "$OUT_PATH")"

exec "$CC_BIN" \
  -std=c11 \
  -O2 \
  -fPIC \
  -shared \
  -Wl,-soname,libminiaudio.so \
  -o "$OUT_PATH" \
  "$ROOT_DIR/native/miniaudio_bridge.c" \
  -ldl -lm -lpthread

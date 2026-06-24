#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_PORT="${HTTP_PORT:-8892}"
GO_BIN="${GO_BIN:-go}"

if command -v docker >/dev/null 2>&1; then
  (cd "$ROOT_DIR" && docker compose up -d mysql)
else
  echo "Docker not found. MySQL is required for E2E API tests." >&2
  exit 1
fi

cd "$ROOT_DIR/learning-api"
HTTP_PORT="$API_PORT" "$GO_BIN" run ./cmd/api

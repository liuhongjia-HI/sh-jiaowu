#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$ROOT_DIR/tmp/start"
API_PORT="${HTTP_PORT:-8892}"
WEB_PORT="${WEB_PORT:-5173}"
API_LOG="$LOG_DIR/learning-api.log"
WEB_LOG="$LOG_DIR/web.log"
API_PID=""
WEB_PID=""

mkdir -p "$LOG_DIR"

info() {
  printf "\033[1;34m[starline]\033[0m %s\n" "$1"
}

warn() {
  printf "\033[1;33m[starline]\033[0m %s\n" "$1"
}

fail() {
  printf "\033[1;31m[starline]\033[0m %s\n" "$1" >&2
  exit 1
}

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing command: $1"
}

port_in_use() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
    return $?
  fi
  return 1
}

stop_port_listener() {
  local port="$1"
  local name="$2"
  local pids
  local attempt=1

  pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -z "$pids" ]; then
    return
  fi

  info "Stopping existing $name process on port $port..."
  kill $pids >/dev/null 2>&1 || true

  while [ "$attempt" -le 10 ]; do
    if ! port_in_use "$port"; then
      info "Port $port is available."
      return
    fi
    sleep 1
    attempt=$((attempt + 1))
  done

  pids="$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    warn "$name process did not stop gracefully. Force stopping it..."
    kill -9 $pids >/dev/null 2>&1 || true
  fi

  port_in_use "$port" && fail "Port $port is still in use after stopping the existing $name process."
}

wait_for_url() {
  local url="$1"
  local name="$2"
  local max_attempts="${3:-40}"
  local attempt=1

  while [ "$attempt" -le "$max_attempts" ]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      info "$name is ready: $url"
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done

  return 1
}

cleanup() {
  if [ -z "$WEB_PID" ] && [ -z "$API_PID" ]; then
    return
  fi
  info "Stopping Starline..."
  if [ -n "$WEB_PID" ] && kill -0 "$WEB_PID" >/dev/null 2>&1; then
    kill "$WEB_PID" >/dev/null 2>&1 || true
  fi
  if [ -n "$API_PID" ] && kill -0 "$API_PID" >/dev/null 2>&1; then
    kill "$API_PID" >/dev/null 2>&1 || true
  fi
}

trap cleanup INT TERM EXIT

need_command go
need_command npm
need_command curl
need_command lsof

if port_in_use "$API_PORT"; then
  stop_port_listener "$API_PORT" "API"
fi

if port_in_use "$WEB_PORT"; then
  fail "Port $WEB_PORT is already in use. Stop the existing web process first."
fi

if command -v docker >/dev/null 2>&1; then
  info "Starting MySQL dependency..."
  if (cd "$ROOT_DIR" && docker compose up -d mysql); then
    info "MySQL dependency started."
  else
    fail "MySQL failed to start. MySQL is required for the API."
  fi
else
  fail "Docker not found. MySQL is required for the API."
fi

if [ ! -d "$ROOT_DIR/web/node_modules" ]; then
  info "Installing web dependencies..."
  (cd "$ROOT_DIR/web" && npm install)
fi

: > "$API_LOG"
: > "$WEB_LOG"

info "Starting learning-api on http://127.0.0.1:$API_PORT ..."
(
  cd "$ROOT_DIR/learning-api"
  HTTP_PORT="$API_PORT" go run ./cmd/api
) >"$API_LOG" 2>&1 &
API_PID="$!"

if ! wait_for_url "http://127.0.0.1:$API_PORT/api/health" "learning-api"; then
  warn "learning-api did not become ready. Last log lines:"
  tail -n 40 "$API_LOG" || true
  exit 1
fi

info "Starting web on http://127.0.0.1:$WEB_PORT ..."
(
  cd "$ROOT_DIR/web"
  HTTP_PORT="$API_PORT" npm run dev -- --host 0.0.0.0 --port "$WEB_PORT" --strictPort
) >"$WEB_LOG" 2>&1 &
WEB_PID="$!"

if ! wait_for_url "http://127.0.0.1:$WEB_PORT" "web"; then
  warn "web did not become ready. Last log lines:"
  tail -n 40 "$WEB_LOG" || true
  exit 1
fi

cat <<EOF

Starline is running.

Admin web: http://127.0.0.1:$WEB_PORT
API health: http://127.0.0.1:$API_PORT/api/health

Logs:
  API: $API_LOG
  Web: $WEB_LOG

Press Ctrl+C to stop API and web.
Docker dependencies remain running. Stop them with: docker compose down

EOF

wait

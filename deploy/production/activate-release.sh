#!/usr/bin/env bash
set -euo pipefail

APP_ROOT="${1:-/opt/starline}"
RELEASE_ID="${2:-}"
API_PORT="${STARLINE_API_PORT:-8892}"
SERVICE_NAME="${STARLINE_SERVICE_NAME:-starline-api}"

if [ -z "$RELEASE_ID" ]; then
  echo "Usage: activate-release.sh <app-root> <release-id>" >&2
  exit 1
fi

RELEASE_DIR="$APP_ROOT/releases/$RELEASE_ID"
CURRENT_LINK="$APP_ROOT/current"

if [ -L "$CURRENT_LINK" ] || [ -d "$CURRENT_LINK" ]; then
  CURRENT_DIR="$(readlink -f "$CURRENT_LINK" 2>/dev/null || true)"
else
  CURRENT_DIR=""
fi

if [ ! -x "$RELEASE_DIR/learning-api/learning-api" ]; then
  if [ -n "$CURRENT_DIR" ] && [ -x "$CURRENT_DIR/learning-api/learning-api" ]; then
    mkdir -p "$RELEASE_DIR/learning-api"
    cp "$CURRENT_DIR/learning-api/learning-api" "$RELEASE_DIR/learning-api/learning-api"
  else
    echo "Missing API binary: $RELEASE_DIR/learning-api/learning-api" >&2
    exit 1
  fi
fi

if [ ! -f "$RELEASE_DIR/web/dist/index.html" ]; then
  if [ -n "$CURRENT_DIR" ] && [ -f "$CURRENT_DIR/web/dist/index.html" ]; then
    mkdir -p "$RELEASE_DIR/web/dist"
    cp -a "$CURRENT_DIR/web/dist/." "$RELEASE_DIR/web/dist/"
  else
    echo "Missing web entry: $RELEASE_DIR/web/dist/index.html" >&2
    exit 1
  fi
fi

if [ ! -f "$RELEASE_DIR/web/dist/index.html" ]; then
  echo "Missing web entry after preparation: $RELEASE_DIR/web/dist/index.html" >&2
  exit 1
fi

ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload
  systemctl restart "$SERVICE_NAME"
fi

for attempt in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$API_PORT/api/health" >/dev/null 2>&1; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    echo "API health check failed after restart." >&2
    if command -v journalctl >/dev/null 2>&1; then
      journalctl -u "$SERVICE_NAME" -n 80 --no-pager || true
    fi
    exit 1
  fi
  sleep 1
done

if command -v nginx >/dev/null 2>&1; then
  nginx -t
  if command -v systemctl >/dev/null 2>&1; then
    systemctl reload nginx || systemctl restart nginx
  fi
fi

find "$APP_ROOT/releases" -mindepth 1 -maxdepth 1 -type d | sort -r | tail -n +6 | xargs -r rm -rf

echo "Activated Starline release: $RELEASE_ID"

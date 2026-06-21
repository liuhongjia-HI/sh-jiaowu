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

if [ ! -x "$RELEASE_DIR/learning-api/learning-api" ]; then
  echo "Missing API binary: $RELEASE_DIR/learning-api/learning-api" >&2
  exit 1
fi

if [ ! -d "$RELEASE_DIR/web/dist" ]; then
  echo "Missing web dist: $RELEASE_DIR/web/dist" >&2
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

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

WEB_DIST="$RELEASE_DIR/web/dist"
WEB_ASSET_VALIDATOR="$RELEASE_DIR/deploy/production/validate-web-assets.sh"
if [ ! -f "$WEB_ASSET_VALIDATOR" ]; then
  echo "Missing web asset validator: $WEB_ASSET_VALIDATOR" >&2
  exit 1
fi

# 先验证发布包自身，避免入口和本次构建产物不一致时切换 current。
bash "$WEB_ASSET_VALIDATOR" "$WEB_DIST"

# hash 资源不会覆盖旧文件。把历史发布中的资源合并到当前 dist，保证已打开页面
# 仍能加载旧版本的 lazy chunk；清理旧 release 后这些资源仍由 current 提供。
TARGET_ASSETS="$WEB_DIST/assets"
mkdir -p "$TARGET_ASSETS"
shopt -s nullglob
for SOURCE_ASSETS in "$APP_ROOT"/releases/*/web/dist/assets; do
  if [ "$SOURCE_ASSETS" = "$TARGET_ASSETS" ] || [ ! -d "$SOURCE_ASSETS" ]; then
    continue
  fi
  for SOURCE_FILE in "$SOURCE_ASSETS"/*; do
    if [ ! -f "$SOURCE_FILE" ]; then
      continue
    fi
    ASSET_NAME="${SOURCE_FILE##*/}"
    if [ ! -e "$TARGET_ASSETS/$ASSET_NAME" ]; then
      cp -a "$SOURCE_FILE" "$TARGET_ASSETS/$ASSET_NAME"
    fi
  done
done
shopt -u nullglob

# 再验证合并后的 current 目录，确保 Nginx 切换后所有资源引用都可用。
bash "$WEB_ASSET_VALIDATOR" "$WEB_DIST"

ln -sfn "$RELEASE_DIR" "$CURRENT_LINK"

if [ ! -f "$CURRENT_LINK/web/dist/index.html" ]; then
  echo "Current web entry is unavailable after switch: $CURRENT_LINK/web/dist/index.html" >&2
  exit 1
fi

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

find "$APP_ROOT/releases" -mindepth 1 -maxdepth 1 -type d ! -samefile "$RELEASE_DIR" -printf '%T@ %p\n' |
  sort -rn |
  tail -n +5 |
  cut -d' ' -f2- |
  xargs -r rm -rf

if [ ! -f "$CURRENT_LINK/web/dist/index.html" ]; then
  echo "Current web entry is unavailable after cleanup: $CURRENT_LINK/web/dist/index.html" >&2
  exit 1
fi

echo "Activated Starline release: $RELEASE_ID"

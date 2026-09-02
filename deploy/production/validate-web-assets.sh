#!/usr/bin/env bash
set -euo pipefail

DIST_DIR="${1:-web/dist}"
MODE="${2:-all}"
INDEX_FILE="$DIST_DIR/index.html"

if [ "$MODE" != "all" ] && [ "$MODE" != "--entry-only" ]; then
  echo "Usage: validate-web-assets.sh [dist-dir] [--entry-only]" >&2
  exit 2
fi

if [ ! -f "$INDEX_FILE" ]; then
  echo "Missing web entry: $INDEX_FILE" >&2
  exit 1
fi

if [ ! -d "$DIST_DIR/assets" ]; then
  echo "Missing web assets directory: $DIST_DIR/assets" >&2
  exit 1
fi

missing=0
while IFS= read -r asset_name; do
  if [ ! -f "$DIST_DIR/assets/$asset_name" ]; then
    echo "Missing referenced web asset: $DIST_DIR/assets/$asset_name" >&2
    missing=1
  fi
done < <(
  if [ "$MODE" = "--entry-only" ]; then
    grep -hEo '[A-Za-z0-9_.-]+-[A-Za-z0-9_-]+\.(js|css)' "$INDEX_FILE" || true
  else
    find "$DIST_DIR" -maxdepth 2 -type f \( -name '*.html' -o -name '*.js' -o -name '*.css' \) -exec grep -hEo '[A-Za-z0-9_.-]+-[A-Za-z0-9_-]+\.(js|css)' {} + || true
  fi |
    sort -u
)

if [ "$missing" -ne 0 ]; then
  echo "Web asset validation failed." >&2
  exit 1
fi

echo "Validated web assets: $DIST_DIR"

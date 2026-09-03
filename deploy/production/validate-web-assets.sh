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
checked_files=""
files_to_check=("$INDEX_FILE")

while [ "${#files_to_check[@]}" -gt 0 ]; do
  source_file="${files_to_check[0]}"
  files_to_check=("${files_to_check[@]:1}")
  if printf '%s' "$checked_files" | grep -Fqx "$source_file"; then
    continue
  fi
  checked_files="${checked_files}${source_file}"$'\n'

  while IFS= read -r asset_name; do
    [ -n "$asset_name" ] || continue
    asset_file="$DIST_DIR/assets/$asset_name"
    if [ ! -f "$asset_file" ]; then
      echo "Missing referenced web asset: $asset_file" >&2
      missing=1
      continue
    fi
    if [ "$MODE" != "--entry-only" ]; then
      files_to_check+=("$asset_file")
    fi
  done < <(grep -hEo '[A-Za-z0-9_.-]+-[A-Za-z0-9_-]+\.(js|css)' "$source_file" || true)
done

if [ "$missing" -ne 0 ]; then
  echo "Web asset validation failed." >&2
  exit 1
fi

echo "Validated web assets: $DIST_DIR"

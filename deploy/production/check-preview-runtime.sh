#!/usr/bin/env bash
set -euo pipefail

missing=()
for command_name in soffice gs qpdf; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    missing+=("$command_name")
  fi
done

if command -v gs >/dev/null 2>&1 && ! gs -q -dBATCH -dNOPAUSE -dNODISPLAY \
  -c '/NotoSansCJKsc-Regular-UniGB-UCS2-H findfont pop quit' >/dev/null 2>&1; then
	missing+=("Noto CJK Ghostscript font")
fi

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Missing preview runtime commands: ${missing[*]}" >&2
	echo "Run deploy/production/provision-preview-runtime.sh as root or with passwordless sudo before activating this release." >&2
  exit 1
fi

soffice --headless --version >/dev/null
gs --version >/dev/null
qpdf --version >/dev/null

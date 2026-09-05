#!/usr/bin/env bash
set -euo pipefail

if ! command -v apt-get >/dev/null 2>&1; then
	echo "This provisioner currently supports Debian/Ubuntu servers with apt-get." >&2
	exit 1
fi

if [ "$(id -u)" -eq 0 ]; then
	APT_PREFIX=()
else
	if ! command -v sudo >/dev/null 2>&1; then
		echo "Preview runtime installation requires root or passwordless sudo." >&2
		exit 1
	fi
	if ! sudo -n true >/dev/null 2>&1; then
		echo "Preview runtime installation requires passwordless sudo." >&2
		exit 1
	fi
	APT_PREFIX=(sudo -n)
fi

export DEBIAN_FRONTEND=noninteractive
"${APT_PREFIX[@]}" apt-get update
"${APT_PREFIX[@]}" apt-get install -y libreoffice ghostscript qpdf fonts-noto-cjk

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"$SCRIPT_DIR/check-preview-runtime.sh"

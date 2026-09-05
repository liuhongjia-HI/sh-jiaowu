#!/usr/bin/env bash
set -euo pipefail

missing=()
for command_name in soffice gs qpdf; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    missing+=("$command_name")
  fi
done

watermark_font_path="/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
if command -v gs >/dev/null 2>&1; then
	if [ ! -f "$watermark_font_path" ]; then
		missing+=("$watermark_font_path")
	else
		watermark_cidfmap="$(mktemp)"
		printf '%s\n' '%!PS' "/StarlineNotoSansCJKsc << /FileType /TrueType /Path ($watermark_font_path) /SubfontID 2 /CSI [(Artifex) (Unicode) 0] >> ;" > "$watermark_cidfmap"
		watermark_cidmap_dir="$(mktemp -d)"
		mv "$watermark_cidfmap" "$watermark_cidmap_dir/cidfmap"
		watermark_cidfmap="$watermark_cidmap_dir/cidfmap"
		if ! gs -q -dBATCH -dNOPAUSE -dNODISPLAY \
			-I"$watermark_cidmap_dir" \
			--permit-file-read="$watermark_cidfmap" \
			--permit-file-read="$watermark_font_path" \
			-c '/StarlineNotoSansCJKsc-Identity-UTF16-H findfont pop quit' >/dev/null 2>&1; then
			missing+=("Noto CJK Unicode Ghostscript font map")
		fi
		rm -f "$watermark_cidfmap"
		rmdir "$watermark_cidmap_dir"
	fi
fi

if [ "${#missing[@]}" -gt 0 ]; then
  echo "Missing preview runtime commands: ${missing[*]}" >&2
	echo "Run deploy/production/provision-preview-runtime.sh as root or with passwordless sudo before activating this release." >&2
  exit 1
fi

soffice --headless --version >/dev/null
gs --version >/dev/null
qpdf --version >/dev/null

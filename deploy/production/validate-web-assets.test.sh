#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT

mkdir -p "$TEST_ROOT/dist/assets"
cat > "$TEST_ROOT/dist/index.html" <<'EOF'
<script type="module" src="/assets/index-current.js"></script>
EOF
cat > "$TEST_ROOT/dist/assets/index-current.js" <<'EOF'
import('./current-lazy.js');
EOF
cat > "$TEST_ROOT/dist/assets/current-lazy.js" <<'EOF'
export default true;
EOF
cat > "$TEST_ROOT/dist/assets/legacy.js" <<'EOF'
import('./removed-from-legacy-release.js');
EOF

if bash "$(dirname "$0")/validate-web-assets.sh" "$TEST_ROOT/dist" --entry-only >/dev/null 2>&1; then
  exit 0
fi

echo "entry-only validation should ignore orphaned legacy chunks" >&2
exit 1

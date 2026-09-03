#!/usr/bin/env bash
set -euo pipefail

TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT

mkdir -p "$TEST_ROOT/dist/assets"
cat > "$TEST_ROOT/dist/index.html" <<'EOF'
<script type="module" src="/assets/index-current-A1.js"></script>
EOF
cat > "$TEST_ROOT/dist/assets/index-current-A1.js" <<'EOF'
import('./current-lazy-B2.js');
EOF
cat > "$TEST_ROOT/dist/assets/current-lazy-B2.js" <<'EOF'
export default true;
EOF
cat > "$TEST_ROOT/dist/assets/legacy-C3.js" <<'EOF'
import('./removed-from-legacy-D4.js');
EOF

if bash "$(dirname "$0")/validate-web-assets.sh" "$TEST_ROOT/dist" >/dev/null 2>&1; then
  mv "$TEST_ROOT/dist/assets/current-lazy-B2.js" "$TEST_ROOT/dist/assets/current-lazy-B2.js.missing"
  if bash "$(dirname "$0")/validate-web-assets.sh" "$TEST_ROOT/dist" >/dev/null 2>&1; then
    echo "validation should reject a missing reachable chunk" >&2
    exit 1
  fi
  exit 0
fi

echo "entry-only validation should ignore orphaned legacy chunks" >&2
exit 1

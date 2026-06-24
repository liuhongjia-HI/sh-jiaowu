#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"

echo "[admin-test] running backend API tests"
(cd "$ROOT_DIR/learning-api" && "$GO_BIN" test ./...)

echo "[admin-test] checking web TypeScript"
(cd "$ROOT_DIR/web" && node node_modules/typescript/bin/tsc --noEmit --pretty false)

echo "[admin-test] checking Playwright config and specs"
(cd "$ROOT_DIR/web" && node node_modules/typescript/bin/tsc --noEmit --pretty false --target ES2020 --module ESNext --moduleResolution Node --skipLibCheck --jsx react-jsx --types node playwright.config.ts tests/admin.e2e.spec.ts)

echo "[admin-test] running browser E2E"
(cd "$ROOT_DIR/web" && node ./scripts/admin-smoke.mjs)

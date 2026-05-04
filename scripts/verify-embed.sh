#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail

cd "$(dirname "$0")/.."

echo "→ building frontend"
(cd frontend && npm run build)

echo "→ building Go binary"
go build -o /tmp/cairn-embed-test ./cmd/cairn

echo "→ starting server"
CAIRN_DB_PATH=/tmp/cairn-embed-test.db \
CAIRN_ENCRYPTION_KEY=dev-key-32-bytes-minimum-length-here \
/tmp/cairn-embed-test &
PID=$!
trap 'kill $PID 2>/dev/null || true; rm -f /tmp/cairn-embed-test.db' EXIT
sleep 2

echo "→ GET /login"
curl -fs http://localhost:8080/login | grep -qi '<title>' || { echo "FAIL /login"; exit 1; }

echo "→ GET /dashboard (deep route, expect SPA fallback)"
curl -fs http://localhost:8080/dashboard | grep -qi '<title>' || { echo "FAIL /dashboard"; exit 1; }

echo "→ GET /healthz"
curl -fs http://localhost:8080/healthz | grep -q 'ok' || { echo "FAIL /healthz"; exit 1; }

echo "embed verification passed"

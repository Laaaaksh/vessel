#!/usr/bin/env bash
# Smoke: unit tests + optional live Apple CLI lifecycle.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== unit =="
go test ./... -race -count=1

if command -v container >/dev/null && container list --all >/dev/null 2>&1; then
  echo "== live =="
  go test -tags=live ./internal/backend -run Live -count=1 -v
else
  echo "== live skipped (container CLI not available) =="
fi

echo "ok"

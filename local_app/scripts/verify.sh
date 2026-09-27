#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

UNFORMATTED="$(gofmt -l cmd internal)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "GOFMT_CHECK=FAIL"
  printf '%s\n' "$UNFORMATTED"
  exit 1
fi
echo "GOFMT_CHECK=PASS"

go test ./...
go vet ./...
node --test tests/*.test.cjs
go build ./cmd/llm-hub-local

echo "PHASE1_LOCAL_APP_VERIFY=PASS"

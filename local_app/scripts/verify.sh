#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

gofmt -w cmd internal
go test ./...
go vet ./...
node --test tests/*.test.cjs
go build ./cmd/llm-hub-local

echo "PHASE1_LOCAL_APP_VERIFY=PASS"

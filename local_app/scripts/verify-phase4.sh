#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

rm -f ./llm-hub-local
go test ./...
go test -race ./...
go vet ./...
node --test tests/*.test.cjs
go build -o ./llm-hub-local ./cmd/llm-hub-local
rm -f ./llm-hub-local
bash -n scripts/*.sh
python3 -m py_compile scripts/*.py
rm -rf scripts/__pycache__

echo "PHASE4_BUILD_VERIFY=PASS"

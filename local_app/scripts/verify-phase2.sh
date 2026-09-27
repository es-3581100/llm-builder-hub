#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

./scripts/verify.sh
rm -f ./llm-hub-local

go test -race ./...
go vet ./...

echo "PHASE2_LOCAL_REPOSITORY_VERIFY=PASS"

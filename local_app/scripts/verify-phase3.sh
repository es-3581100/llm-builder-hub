#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"

cd "$REPO_ROOT"

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "PHASE3_VERIFY_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

HEAD_BEFORE="$(git rev-parse HEAD)"
STATUS_BEFORE="$(git status --porcelain=v1 --untracked-files=all)"

cd "$APP_DIR"

./scripts/verify-phase2.sh

UNFORMATTED="$(gofmt -l cmd internal)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "PHASE3_GO_FORMAT=FAIL" >&2
  printf '%s\n' "$UNFORMATTED" >&2
  exit 1
fi

go test ./...
go test -race ./...
go vet ./...
node --test tests/*.test.cjs

cd "$REPO_ROOT"

git diff --check

HEAD_AFTER="$(git rev-parse HEAD)"
STATUS_AFTER="$(git status --porcelain=v1 --untracked-files=all)"

if [[ "$HEAD_AFTER" != "$HEAD_BEFORE" ]]; then
  echo "PHASE3_VERIFY_FAIL: verifier changed HEAD" >&2
  exit 1
fi

if [[ "$STATUS_AFTER" != "$STATUS_BEFORE" ]]; then
  echo "PHASE3_VERIFY_FAIL: verifier changed worktree state" >&2
  diff -u <(printf '%s\n' "$STATUS_BEFORE") <(printf '%s\n' "$STATUS_AFTER") >&2 || true
  exit 1
fi

echo "PHASE3_LOCAL_WRITE_VERIFY=PASS"

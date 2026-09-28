#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

# Preflight before any git call: REPO_ROOT resolution itself needs git.
for cmd in git go node sha256sum; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "PHASE4_VERIFY_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"
PACK_DIR="$REPO_ROOT/phase_4"

cd "$REPO_ROOT"

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "PHASE4_VERIFY_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

# The Phase-4 pack is sealed authority: the build must never edit it. Verify the
# recorded digests rather than regenerating them, so drift fails loudly.
for manifest in "$PACK_DIR/manifests/pack.files.sha256" "$PACK_DIR/manifests/PROMPT_PACK_SHA256.txt"; do
  [[ -f "$manifest" ]] || {
    echo "PHASE4_VERIFY_BLOCKED: missing sealed-pack manifest: $manifest" >&2
    exit 2
  }
done

# Pack paths in pack.files.sha256 are relative to phase_4/.
( cd "$PACK_DIR" && sha256sum -c manifests/pack.files.sha256 ) || {
  echo "PHASE4_VERIFY_FAIL: sealed Phase-4 pack contents do not match manifests/pack.files.sha256" >&2
  exit 1
}

# The declared pack digest must still describe the manifest file's own bytes.
( cd "$PACK_DIR/manifests" && sha256sum -c PROMPT_PACK_SHA256.txt ) || {
  echo "PHASE4_VERIFY_FAIL: manifests/pack.files.sha256 does not match the digest declared in manifests/PROMPT_PACK_SHA256.txt" >&2
  exit 1
}

HEAD_BEFORE="$(git rev-parse HEAD)"
STATUS_BEFORE="$(git status --porcelain=v1 --untracked-files=all)"

cd "$APP_DIR"

# Compose upward: verify-phase3.sh already runs verify-phase2.sh and verify.sh.
./scripts/verify-phase3.sh

UNFORMATTED="$(gofmt -l cmd internal)"
if [[ -n "$UNFORMATTED" ]]; then
  echo "PHASE4_GO_FORMAT=FAIL" >&2
  printf '%s\n' "$UNFORMATTED" >&2
  exit 1
fi

go build ./...
go test ./...
go test -race ./...
go vet ./...
node --test tests/*.test.cjs

cd "$REPO_ROOT"

git diff --check

HEAD_AFTER="$(git rev-parse HEAD)"
STATUS_AFTER="$(git status --porcelain=v1 --untracked-files=all)"

if [[ "$HEAD_AFTER" != "$HEAD_BEFORE" ]]; then
  echo "PHASE4_VERIFY_FAIL: verifier changed HEAD" >&2
  exit 1
fi

if [[ "$STATUS_AFTER" != "$STATUS_BEFORE" ]]; then
  echo "PHASE4_VERIFY_FAIL: verifier changed worktree state" >&2
  diff -u <(printf '%s\n' "$STATUS_BEFORE") <(printf '%s\n' "$STATUS_AFTER") >&2 || true
  exit 1
fi

echo "PHASE4_LOCAL_GIT_COMMIT_CONTROL_VERIFY=PASS"

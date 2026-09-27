#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"
BASELINE="860aab35775634c610edaf51191f154aa3afb6a7"
PORT="${PORT:-18766}"
ADDR="127.0.0.1:${PORT}"
URL="http://${ADDR}"

for cmd in git go curl python3 sha256sum grep sort; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

python3 - <<'PY' >/dev/null 2>&1 || {
from playwright.sync_api import sync_playwright
PY
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: Python Playwright is not installed in this python3 environment" >&2
  exit 2
}

cd "$REPO_ROOT"

git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: HEAD is not descended from Phase-3 UI baseline $BASELINE" >&2
  exit 2
}

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/phase3-host-acceptance/${STAMP}}"
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/llm-hub-phase3-auto.XXXXXX")"
SERVICE_PID=""
cleanup() {
  if [[ -n "$SERVICE_PID" ]]; then
    kill "$SERVICE_PID" 2>/dev/null || true
    wait "$SERVICE_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

TEST_REPO="$TMP/repository with spaces"
STATE="$TMP/workstation-state.json"
BIN="$TMP/llm-hub-local"
mkdir -p "$TEST_REPO"

git -C "$TEST_REPO" init -b main >/dev/null
git -C "$TEST_REPO" config user.name "Phase Three Automated Acceptance"
git -C "$TEST_REPO" config user.email "phase3-auto@example.invalid"
printf 'browser original\n' >"$TEST_REPO/source.txt"
printf 'conflict original\n' >"$TEST_REPO/conflict-ui.txt"
printf '# Phase 3 Automated Acceptance\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add source.txt conflict-ui.txt README.md
git -C "$TEST_REPO" commit -m "phase3 automated host baseline" >/dev/null

HEAD_BEFORE="$(git -C "$TEST_REPO" rev-parse HEAD)"
BRANCH_BEFORE="$(git -C "$TEST_REPO" branch --show-current)"
INDEX_SHA_BEFORE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
STATUS_BEFORE="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"
REMOTE_BEFORE="$(git -C "$TEST_REPO" remote)"

cd "$APP_DIR"
./scripts/verify-phase3.sh

go build -o "$BIN" ./cmd/llm-hub-local
"$BIN" -addr "$ADDR" -state "$STATE" -repo "$TEST_REPO" >"$EVIDENCE_DIR/service.log" 2>&1 &
SERVICE_PID=$!

READY=0
for _ in $(seq 1 100); do
  if kill -0 "$SERVICE_PID" 2>/dev/null     && curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-before.json" 2>/dev/null     && python3 - "$EVIDENCE_DIR/state-before.json" >/dev/null 2>&1 <<'PY'
import json, sys
json.load(open(sys.argv[1]))
PY
  then
    READY=1
    break
  fi
  sleep 0.1
done

if [[ "$READY" -ne 1 ]]; then
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: loopback service did not become ready at $URL" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
fi

REV_BEFORE="$(python3 - "$EVIDENCE_DIR/state-before.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["state"]["revision"])
PY
)"

python3 "$APP_DIR/scripts/host-accept-phase3.py" "$URL" "$TEST_REPO" "$EVIDENCE_DIR"

curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-after.json"
REV_AFTER="$(python3 - "$EVIDENCE_DIR/state-after.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["state"]["revision"])
PY
)"

HEAD_AFTER="$(git -C "$TEST_REPO" rev-parse HEAD)"
BRANCH_AFTER="$(git -C "$TEST_REPO" branch --show-current)"
INDEX_SHA_AFTER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
STAGED_AFTER="$(git -C "$TEST_REPO" diff --cached --name-only | sort)"
UNSTAGED_AFTER="$(git -C "$TEST_REPO" diff --name-only | sort)"
REMOTE_AFTER="$(git -C "$TEST_REPO" remote)"

printf 'phase3 browser write\nline two\n' >"$EVIDENCE_DIR/source-expected.txt"
printf 'external edit wins\n' >"$EVIDENCE_DIR/conflict-expected.txt"

cmp -s "$EVIDENCE_DIR/source-expected.txt" "$TEST_REPO/source.txt" || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: source.txt bytes mismatch" >&2
  exit 1
}
cmp -s "$EVIDENCE_DIR/conflict-expected.txt" "$TEST_REPO/conflict-ui.txt" || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: conflict-ui.txt external bytes were not preserved" >&2
  exit 1
}

[[ "$HEAD_AFTER" == "$HEAD_BEFORE" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: HEAD changed" >&2
  exit 1
}
[[ "$BRANCH_AFTER" == "$BRANCH_BEFORE" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: branch changed" >&2
  exit 1
}
[[ "$INDEX_SHA_AFTER" == "$INDEX_SHA_BEFORE" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: .git/index changed" >&2
  exit 1
}
[[ -z "$STAGED_AFTER" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: staged changes appeared: $STAGED_AFTER" >&2
  exit 1
}

EXPECTED_UNSTAGED="$(printf '%s\n' conflict-ui.txt source.txt | sort)"
[[ "$UNSTAGED_AFTER" == "$EXPECTED_UNSTAGED" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: unexpected unstaged paths" >&2
  printf 'expected:\n%s\nactual:\n%s\n' "$EXPECTED_UNSTAGED" "$UNSTAGED_AFTER" >&2
  exit 1
}

[[ "$REMOTE_AFTER" == "$REMOTE_BEFORE" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: Git remote configuration changed" >&2
  exit 1
}

(( REV_AFTER == REV_BEFORE + 1 )) || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: expected exactly one workstation SAVE revision advance ($REV_BEFORE -> $REV_AFTER)" >&2
  exit 1
}

grep -F 'POST /api/write-file status=200 code=OK path="source.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: successful source write log missing" >&2
  exit 1
}
grep -F 'POST /api/write-file status=409 code=CONTENT_CONFLICT path="conflict-ui.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: CONTENT_CONFLICT log missing" >&2
  exit 1
}

curl -fsS "$URL/api/export" >"$EVIDENCE_DIR/llm-hub-project.html"
for forbidden in '<textarea' 'WRITE FILE' 'EDIT SOURCE' 'CANCEL SOURCE EDIT' '/api/write-file'; do
  if grep -F "$forbidden" "$EVIDENCE_DIR/llm-hub-project.html" >/dev/null; then
    echo "PHASE3_HOST_ACCEPTANCE_FAIL: static export contains mutation surface: $forbidden" >&2
    exit 1
  fi
done

git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/final-status.txt"
git -C "$TEST_REPO" diff --no-color --no-ext-diff --no-textconv -- >"$EVIDENCE_DIR/final-working-tree.diff"

{
  echo "tested_head=$(git -C "$REPO_ROOT" rev-parse HEAD)"
  echo "url=$URL"
  echo "test_repo=$TEST_REPO"
  echo "head_before=$HEAD_BEFORE"
  echo "head_after=$HEAD_AFTER"
  echo "branch_before=$BRANCH_BEFORE"
  echo "branch_after=$BRANCH_AFTER"
  echo "index_sha_before=$INDEX_SHA_BEFORE"
  echo "index_sha_after=$INDEX_SHA_AFTER"
  echo "revision_before=$REV_BEFORE"
  echo "revision_after=$REV_AFTER"
  echo "status_before<<EOF"
  printf '%s\n' "$STATUS_BEFORE"
  echo "EOF"
  echo "unstaged_after<<EOF"
  printf '%s\n' "$UNSTAGED_AFTER"
  echo "EOF"
} >"$EVIDENCE_DIR/acceptance.txt"

{
  echo "PHASE3_LOCAL_VERIFY=PASS"
  echo "PHASE3_BROWSER_AUTOMATION=PASS"
  echo "PHASE3_SOURCE_WRITE_ACCEPTANCE=PASS"
  echo "PHASE3_CONFLICT_ACCEPTANCE=PASS"
  echo "PHASE3_SAVE_WRITE_SEPARATION=PASS"
  echo "PHASE3_HOST_ACCEPTANCE=PASS"
  echo "PHASE3_COMPLETE=PASS"
} | tee "$EVIDENCE_DIR/RESULT.txt"

echo "Evidence: $EVIDENCE_DIR"

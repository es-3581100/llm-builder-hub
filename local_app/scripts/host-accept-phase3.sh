#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="ec3f555bcaaff251d078a98b88ef0f98eb5e9f9f"
PORT="${PORT:-18766}"
ADDR="127.0.0.1:${PORT}"
URL="http://${ADDR}"

for cmd in git go curl python3 sha256sum grep sort; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "HOST_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel)"
cd "$REPO_ROOT"

git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "HOST_ACCEPTANCE_BLOCKED: current HEAD is not descended from Phase-3 UI checkpoint $BASELINE" >&2
  exit 2
}

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  echo "HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean before host acceptance" >&2
  git status --short >&2
  exit 2
fi

if [[ -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  echo "HOST_ACCEPTANCE_BLOCKED: no graphical DISPLAY/WAYLAND_DISPLAY detected" >&2
  exit 2
fi

BROWSER_CMD="${BROWSER_CMD:-}"
if [[ -z "$BROWSER_CMD" ]]; then
  for candidate in firefox firefox-esr thorium-browser chromium chromium-browser google-chrome; do
    if command -v "$candidate" >/dev/null 2>&1; then
      BROWSER_CMD="$candidate"
      break
    fi
  done
fi
if [[ -z "$BROWSER_CMD" ]]; then
  echo "HOST_ACCEPTANCE_BLOCKED: no supported browser found; set BROWSER_CMD explicitly" >&2
  exit 2
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/phase3-host-acceptance/${STAMP}}"
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/llm-hub-phase3-host.XXXXXX")"
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
git -C "$TEST_REPO" config user.name "Phase Three Host Acceptance"
git -C "$TEST_REPO" config user.email "phase3-host@example.invalid"

printf 'line one\nline two\n' >"$TEST_REPO/source.txt"
printf 'conflict baseline\n' >"$TEST_REPO/conflict.txt"
printf '# Phase 3 Host Acceptance\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add source.txt conflict.txt README.md
git -C "$TEST_REPO" commit -m "phase3 host acceptance baseline" >/dev/null

SOURCE_SHA_BEFORE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
CONFLICT_SHA_BEFORE="$(sha256sum "$TEST_REPO/conflict.txt" | awk '{print $1}')"
INDEX_SHA_BEFORE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
HEAD_BEFORE="$(git -C "$TEST_REPO" rev-parse HEAD)"
BRANCH_BEFORE="$(git -C "$TEST_REPO" branch --show-current)"
STATUS_BEFORE="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"

cd "$APP_DIR"
go build -o "$BIN" ./cmd/llm-hub-local

start_service() {
  if [[ -n "$SERVICE_PID" ]]; then
    kill "$SERVICE_PID" 2>/dev/null || true
    wait "$SERVICE_PID" 2>/dev/null || true
  fi

  printf '\n=== service_start utc=%s repo=%q ===\n'     "$(date -u +%Y%m%dT%H%M%SZ)" "$TEST_REPO" >>"$EVIDENCE_DIR/service.log"

  "$BIN" -addr "$ADDR" -state "$STATE" -repo "$TEST_REPO" >>"$EVIDENCE_DIR/service.log" 2>&1 &
  SERVICE_PID=$!

  for _ in $(seq 1 80); do
    if kill -0 "$SERVICE_PID" 2>/dev/null       && curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-current.json" 2>/dev/null       && python3 - "$EVIDENCE_DIR/state-current.json" >/dev/null 2>&1 <<'PY'
import json, sys
json.load(open(sys.argv[1]))
PY
    then
      return 0
    fi
    sleep 0.1
  done

  echo "HOST_ACCEPTANCE_BLOCKED: service did not become ready at $URL" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
}

json_value() {
  python3 - "$1" "$2" <<'PY'
import json, sys
obj = json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
    obj = obj[part]
print(obj)
PY
}

record_check() {
  local key="$1"
  local prompt="$2"
  local ans
  while true; do
    read -r -p "$prompt [y/n]: " ans
    case "${ans,,}" in
      y|yes)
        printf '%s=PASS\n' "$key" >>"$EVIDENCE_DIR/browser-checks.txt"
        return 0
        ;;
      n|no)
        printf '%s=FAIL\n' "$key" >>"$EVIDENCE_DIR/browser-checks.txt"
        return 1
        ;;
    esac
  done
}

PASS=1

start_service
cp "$EVIDENCE_DIR/state-current.json" "$EVIDENCE_DIR/state-before.json"
REV_BEFORE="$(json_value "$EVIDENCE_DIR/state-before.json" state.revision)"

{
  echo "PHASE3_HOST_BROWSER_ACCEPTANCE"
  echo "baseline=$BASELINE"
  echo "tested_head=$(git -C "$REPO_ROOT" rev-parse HEAD)"
  echo "browser=$BROWSER_CMD"
  echo "url=$URL"
  echo "test_repo=$TEST_REPO"
  echo "source_sha_before=$SOURCE_SHA_BEFORE"
  echo "conflict_sha_before=$CONFLICT_SHA_BEFORE"
  echo "index_sha_before=$INDEX_SHA_BEFORE"
  echo "head_before=$HEAD_BEFORE"
  echo "branch_before=$BRANCH_BEFORE"
  echo "status_before<<EOF"
  printf '%s\n' "$STATUS_BEFORE"
  echo "EOF"
} >"$EVIDENCE_DIR/acceptance.txt"

"$BROWSER_CMD" "$URL/" >/dev/null 2>&1 &

cat <<EOF

PHASE-3 REAL BROWSER ACCEPTANCE
===============================
The service is live at:
  $URL

A. Successful explicit source write
-----------------------------------
1. Confirm AUTHORITY: LOCAL and LOCAL GIT REPOSITORY are visible.
2. Find repository document source.txt.
3. Click EDIT SOURCE.
4. Append this exact line:
     PHASE3_BROWSER_WRITE
5. Confirm the document shows SOURCE DRAFT.
6. Click WRITE FILE.
7. Confirm the editor closes and source.txt appears as an unstaged Git change.
EOF

record_check browser_real_loopback   "Did the real browser load the real loopback service?" || PASS=0
record_check browser_source_write   "Did EDIT SOURCE → SOURCE DRAFT → WRITE FILE succeed for source.txt and show it unstaged?" || PASS=0

SOURCE_SHA_AFTER_WRITE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
INDEX_SHA_AFTER_WRITE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
HEAD_AFTER_WRITE="$(git -C "$TEST_REPO" rev-parse HEAD)"
BRANCH_AFTER_WRITE="$(git -C "$TEST_REPO" branch --show-current)"
STAGED_AFTER_WRITE="$(git -C "$TEST_REPO" diff --cached --name-only)"
UNSTAGED_AFTER_WRITE="$(git -C "$TEST_REPO" diff --name-only | sort)"

[[ "$SOURCE_SHA_AFTER_WRITE" != "$SOURCE_SHA_BEFORE" ]] || PASS=0
grep -Fx "PHASE3_BROWSER_WRITE" "$TEST_REPO/source.txt" >/dev/null || PASS=0
[[ "$INDEX_SHA_AFTER_WRITE" == "$INDEX_SHA_BEFORE" ]] || PASS=0
[[ "$HEAD_AFTER_WRITE" == "$HEAD_BEFORE" ]] || PASS=0
[[ "$BRANCH_AFTER_WRITE" == "$BRANCH_BEFORE" ]] || PASS=0
[[ -z "$STAGED_AFTER_WRITE" ]] || PASS=0
[[ "$UNSTAGED_AFTER_WRITE" == "source.txt" ]] || PASS=0

cat <<EOF

B. Cancel is browser-only
-------------------------
1. Find repository document conflict.txt.
2. Click EDIT SOURCE.
3. Append:
     CANCEL_SHOULD_NOT_WRITE
4. Click CANCEL SOURCE EDIT.
5. Confirm the document returns to GIT WORKTREE without writing that marker.
EOF

record_check browser_source_cancel   "Did CANCEL SOURCE EDIT discard the conflict.txt draft without a filesystem write?" || PASS=0

CONFLICT_SHA_AFTER_CANCEL="$(sha256sum "$TEST_REPO/conflict.txt" | awk '{print $1}')"
[[ "$CONFLICT_SHA_AFTER_CANCEL" == "$CONFLICT_SHA_BEFORE" ]] || PASS=0
if grep -F "CANCEL_SHOULD_NOT_WRITE" "$TEST_REPO/conflict.txt" >/dev/null; then
  PASS=0
fi

cat <<EOF

C. Prepare stale-content conflict
---------------------------------
1. On conflict.txt click EDIT SOURCE again.
2. Append:
     BROWSER_STALE_DRAFT
3. DO NOT click WRITE FILE yet.
4. Return to this terminal and answer yes.
EOF

record_check browser_conflict_draft_ready   "Is the dirty conflict.txt browser draft open and still unsaved?" || PASS=0

printf 'EXTERNAL_CONFLICT_WINNER\n' >>"$TEST_REPO/conflict.txt"
CONFLICT_EXTERNAL_SHA="$(sha256sum "$TEST_REPO/conflict.txt" | awk '{print $1}')"

cat <<EOF

An external process has now changed conflict.txt on disk.

Return to the browser:
1. Click WRITE FILE on the still-open stale conflict.txt draft.
2. Confirm the write is refused.
3. Confirm the UI visibly reports CONTENT_CONFLICT / WRITE BLOCKED.
4. Confirm the textarea still contains BROWSER_STALE_DRAFT.

Do not cancel the draft until after answering the next check.
EOF

record_check browser_content_conflict   "Did the browser refuse the stale write visibly and preserve the stale draft?" || PASS=0

CONFLICT_SHA_AFTER_REJECT="$(sha256sum "$TEST_REPO/conflict.txt" | awk '{print $1}')"
[[ "$CONFLICT_SHA_AFTER_REJECT" == "$CONFLICT_EXTERNAL_SHA" ]] || PASS=0
grep -F "EXTERNAL_CONFLICT_WINNER" "$TEST_REPO/conflict.txt" >/dev/null || PASS=0
if grep -F "BROWSER_STALE_DRAFT" "$TEST_REPO/conflict.txt" >/dev/null; then
  PASS=0
fi

curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-after.json"
REV_AFTER="$(json_value "$EVIDENCE_DIR/state-after.json" state.revision)"

INDEX_SHA_AFTER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
HEAD_AFTER="$(git -C "$TEST_REPO" rev-parse HEAD)"
BRANCH_AFTER="$(git -C "$TEST_REPO" branch --show-current)"
STAGED_AFTER="$(git -C "$TEST_REPO" diff --cached --name-only)"
UNSTAGED_AFTER="$(git -C "$TEST_REPO" diff --name-only | sort)"
EXPECTED_UNSTAGED="$(printf '%s\n' conflict.txt source.txt | sort)"

[[ "$REV_AFTER" == "$REV_BEFORE" ]] || PASS=0
[[ "$INDEX_SHA_AFTER" == "$INDEX_SHA_BEFORE" ]] || PASS=0
[[ "$HEAD_AFTER" == "$HEAD_BEFORE" ]] || PASS=0
[[ "$BRANCH_AFTER" == "$BRANCH_BEFORE" ]] || PASS=0
[[ -z "$STAGED_AFTER" ]] || PASS=0
[[ "$UNSTAGED_AFTER" == "$EXPECTED_UNSTAGED" ]] || PASS=0

grep -F 'POST /api/write-file status=200 code=OK path="source.txt"'   "$EVIDENCE_DIR/service.log" >/dev/null || PASS=0
grep -F 'POST /api/write-file status=409 code=CONTENT_CONFLICT path="conflict.txt"'   "$EVIDENCE_DIR/service.log" >/dev/null || PASS=0

curl -fsS "$URL/api/export" >"$EVIDENCE_DIR/llm-hub-project.html"
EXPORT_SHA="$(sha256sum "$EVIDENCE_DIR/llm-hub-project.html" | awk '{print $1}')"

for forbidden in '<textarea' 'WRITE FILE' 'EDIT SOURCE' '/api/write-file'; do
  if grep -F "$forbidden" "$EVIDENCE_DIR/llm-hub-project.html" >/dev/null; then
    echo "STATIC_EXPORT_MUTATION_SURFACE_FOUND=$forbidden" >>"$EVIDENCE_DIR/acceptance.txt"
    PASS=0
  fi
done

{
  echo "source_sha_after_write=$SOURCE_SHA_AFTER_WRITE"
  echo "conflict_sha_after_cancel=$CONFLICT_SHA_AFTER_CANCEL"
  echo "conflict_external_sha=$CONFLICT_EXTERNAL_SHA"
  echo "conflict_sha_after_reject=$CONFLICT_SHA_AFTER_REJECT"
  echo "index_sha_after=$INDEX_SHA_AFTER"
  echo "head_after=$HEAD_AFTER"
  echo "branch_after=$BRANCH_AFTER"
  echo "revision_before=$REV_BEFORE"
  echo "revision_after=$REV_AFTER"
  echo "export_sha256=$EXPORT_SHA"
  echo "staged_after<<EOF"
  printf '%s\n' "$STAGED_AFTER"
  echo "EOF"
  echo "unstaged_after<<EOF"
  printf '%s\n' "$UNSTAGED_AFTER"
  echo "EOF"
} >>"$EVIDENCE_DIR/acceptance.txt"

if [[ "$PASS" -eq 1 ]]; then
  {
    echo "PHASE3_IMPLEMENTATION=PASS"
    echo "HOST_BROWSER_SOURCE_WRITE_ACCEPTANCE=PASS"
    echo "PHASE3_COMPLETE=PASS"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 0
fi

{
  echo "PHASE3_IMPLEMENTATION=PASS"
  echo "HOST_BROWSER_SOURCE_WRITE_ACCEPTANCE=FAIL"
  echo "PHASE3_COMPLETE=BLOCKED"
} | tee "$EVIDENCE_DIR/RESULT.txt"
echo "Evidence: $EVIDENCE_DIR"
exit 1

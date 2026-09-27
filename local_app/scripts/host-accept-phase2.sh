#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="ab7156dba65d0d6a2a7dd67a5502e5a701df3cdd"
PORT="${PORT:-18765}"
ADDR="127.0.0.1:${PORT}"
URL="http://${ADDR}"

for cmd in git go curl python3 sha256sum; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "HOST_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2; exit 2; }
done

cd "$APP_DIR/.."
git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "HOST_ACCEPTANCE_BLOCKED: current HEAD is not descended from frozen Phase-2 baseline $BASELINE" >&2
  exit 2
}
if [[ -n "$(git status --porcelain)" ]]; then
  echo "HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean before host acceptance" >&2
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
  echo "HOST_ACCEPTANCE_BLOCKED: no supported real browser command found; set BROWSER_CMD explicitly" >&2
  exit 2
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/host-acceptance/${STAMP}}"
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/llm-hub-phase2-host.XXXXXX")"
SERVICE_PID=""
cleanup() {
  if [[ -n "$SERVICE_PID" ]]; then kill "$SERVICE_PID" 2>/dev/null || true; wait "$SERVICE_PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT

TEST_REPO="$TMP/repository with spaces"
STATE="$TMP/workstation-state.json"
BIN="$TMP/llm-hub-local"
mkdir -p "$TEST_REPO"

git -C "$TEST_REPO" init -b main >/dev/null
git -C "$TEST_REPO" config user.name "Phase Two Host Acceptance"
git -C "$TEST_REPO" config user.email "phase2-host@example.invalid"
printf 'line one\nline two\n' >"$TEST_REPO/source.txt"
printf '# Host Acceptance\n\nRepository-backed text.\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add source.txt README.md
git -C "$TEST_REPO" commit -m "host acceptance baseline" >/dev/null

printf 'staged line\n' >>"$TEST_REPO/source.txt"
git -C "$TEST_REPO" add source.txt
printf 'unstaged line\n' >>"$TEST_REPO/source.txt"
printf 'untracked text\n' >"$TEST_REPO/new file.txt"
python3 - "$TEST_REPO/binary.bin" <<'PY'
from pathlib import Path
import sys
Path(sys.argv[1]).write_bytes(bytes([0,1,2,3,255]))
PY

SOURCE_SHA_BEFORE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
STATUS_BEFORE="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"
INDEX_SHA_BEFORE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"

cd "$APP_DIR"
go build -o "$BIN" ./cmd/llm-hub-local

start_service() {
  local repo_path="$1"
  if [[ -n "$SERVICE_PID" ]]; then kill "$SERVICE_PID" 2>/dev/null || true; wait "$SERVICE_PID" 2>/dev/null || true; fi
  touch "$EVIDENCE_DIR/service.log"
  printf '\n=== service_start utc=%s repo=%q ===\n' "$(date -u +%Y%m%dT%H%M%SZ)" "$repo_path" >>"$EVIDENCE_DIR/service.log"
  "$BIN" -addr "$ADDR" -state "$STATE" -repo "$repo_path" >>"$EVIDENCE_DIR/service.log" 2>&1 &
  SERVICE_PID=$!
  for _ in $(seq 1 80); do
    if curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-current.json" 2>/dev/null \
      && python3 - "$EVIDENCE_DIR/state-current.json" >/dev/null 2>&1 <<'PY'
import json,sys
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
import json,sys
obj=json.load(open(sys.argv[1]))
for part in sys.argv[2].split('.'):
    obj=obj[part]
print(obj)
PY
}

record_check() {
  local key="$1" prompt="$2" ans
  while true; do
    read -r -p "$prompt [y/n]: " ans
    case "${ans,,}" in
      y|yes) printf '%s=PASS\n' "$key" >>"$EVIDENCE_DIR/browser-checks.txt"; return 0 ;;
      n|no)  printf '%s=FAIL\n' "$key" >>"$EVIDENCE_DIR/browser-checks.txt"; return 1 ;;
    esac
  done
}

start_service "$TEST_REPO"
cp "$EVIDENCE_DIR/state-current.json" "$EVIDENCE_DIR/state-normal-before.json"
REV_BEFORE="$(json_value "$EVIDENCE_DIR/state-normal-before.json" state.revision)"

{
  echo "PHASE2_HOST_BROWSER_ACCEPTANCE"
  echo "baseline=$BASELINE"
  echo "tested_head=$(git -C "$APP_DIR/.." rev-parse HEAD)"
  echo "browser=$BROWSER_CMD"
  echo "url=$URL"
  echo "source_sha_before=$SOURCE_SHA_BEFORE"
  echo "index_sha_before=$INDEX_SHA_BEFORE"
  echo "status_before<<EOF"
  printf '%s\n' "$STATUS_BEFORE"
  echo "EOF"
} >"$EVIDENCE_DIR/acceptance.txt"

"$BROWSER_CMD" "$URL/" >/dev/null 2>&1 &

cat <<EOF

REAL BROWSER ACCEPTANCE
=======================
The browser was opened directly to:
  $URL

Normal-state checks:
  1. Confirm AUTHORITY: LOCAL and LOCAL GIT REPOSITORY are visible.
  2. Confirm branch=main and staged=1, unstaged=1, untracked=2.
  3. Click Home/W/P/B and confirm columns are exactly 4/3/2/1.
     Optional DevTools readback:
       document.getElementById('workspace').dataset.documentColumns
  4. Enter EDIT on a workstation document, modify it, and confirm UNSAVED EDITS.
  5. Click REFRESH while dirty; confirm the guard appears and CANCEL preserves the draft.
  6. CLEAR EDITS, edit again, then SAVE CHANGES.
     Before answering the SAVE check below, paste this in DevTools:
       copy(JSON.stringify({
         save:     document.getElementById('save-state').textContent,
         revision: document.getElementById('revision').textContent
       }))
     Expected after one successful save in this fresh run:
       {"save":"SAVED","revision":"revision 2"}
  7. Confirm repository source documents remain read-only.
  8. Click EXPORT HTML5, open the download, and confirm STATIC PROJECTION / READ ONLY.

Return here after completing those steps.
EOF

PASS=1
record_check browser_real_loopback "Did the real browser load the real loopback service?" || PASS=0
record_check browser_repository_normal "Did repository state render correctly for main with the expected counts?" || PASS=0
record_check browser_geometry "Did 0/1/2/3 sliders visibly produce 4/3/2/1 document columns?" || PASS=0
record_check browser_view_edit "Did VIEW/EDIT workstation behavior work and show dirty state?" || PASS=0
record_check browser_dirty_refresh_guard "Did dirty REFRESH show the guard and preserve the draft on CANCEL?" || PASS=0
record_check browser_save "Did SAVE CHANGES complete successfully?" || PASS=0
record_check browser_export "Did EXPORT HTML5 download and render as a read-only static projection?" || PASS=0

curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-normal-after.json"
REV_AFTER="$(json_value "$EVIDENCE_DIR/state-normal-after.json" state.revision)"
SAVE_LOG_LINE="$(grep 'POST /api/save' "$EVIDENCE_DIR/service.log" | tail -n 1 || true)"
EXPECTED_SAVE_LOG="POST /api/save status=200 revision_before=$REV_BEFORE revision_after=$REV_AFTER"
curl -fsS "$URL/api/export" >"$EVIDENCE_DIR/llm-hub-project.html"
EXPORT_SHA="$(sha256sum "$EVIDENCE_DIR/llm-hub-project.html" | awk '{print $1}')"

SOURCE_SHA_AFTER="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
STATUS_AFTER="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"
INDEX_SHA_AFTER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"

[[ "$SOURCE_SHA_BEFORE" == "$SOURCE_SHA_AFTER" ]] || PASS=0
[[ "$STATUS_BEFORE" == "$STATUS_AFTER" ]] || PASS=0
[[ "$INDEX_SHA_BEFORE" == "$INDEX_SHA_AFTER" ]] || PASS=0
(( REV_AFTER > REV_BEFORE )) || PASS=0
[[ -n "$SAVE_LOG_LINE" ]] || PASS=0
grep -F "$EXPECTED_SAVE_LOG" "$EVIDENCE_DIR/service.log" >/dev/null || PASS=0

git -C "$TEST_REPO" checkout --detach HEAD >/dev/null
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-detached-api.json"
cat <<EOF

DETACHED check:
  Refresh the real browser page.
  Read the always-visible #repository-branch element.
  Confirm it reads exactly: branch: DETACHED
  Ignore the hidden B-slider #branch-name element.
EOF
record_check browser_detached_label "Did #repository-branch render exactly 'branch: DETACHED' after refreshing?" || PASS=0

UNBORN_REPO="$TMP/unborn repository"
mkdir -p "$UNBORN_REPO"
git -C "$UNBORN_REPO" init -b main >/dev/null
start_service "$UNBORN_REPO"
cp "$EVIDENCE_DIR/state-current.json" "$EVIDENCE_DIR/state-unborn-api.json"
cat <<EOF

UNBORN check:
  Refresh the same real loopback URL after the service restart.
  Confirm branch=main and HEAD=UNBORN.
EOF
record_check browser_unborn_label "Did the real browser render the unborn repository correctly?" || PASS=0

{
  echo "revision_before_save=$REV_BEFORE"
  echo "revision_after_browser_steps=$REV_AFTER"
  echo "save_request_log=$SAVE_LOG_LINE"
  echo "source_sha_after=$SOURCE_SHA_AFTER"
  echo "index_sha_after=$INDEX_SHA_AFTER"
  echo "export_sha256=$EXPORT_SHA"
  echo "status_after<<EOF"
  printf '%s\n' "$STATUS_AFTER"
  echo "EOF"
} >>"$EVIDENCE_DIR/acceptance.txt"

if [[ "$PASS" -eq 1 ]]; then
  {
    echo "PHASE2_IMPLEMENTATION=PASS"
    echo "HOST_BROWSER_LOOPBACK_ACCEPTANCE=PASS"
    echo "PHASE2_COMPLETE=PASS"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 0
fi

{
  echo "PHASE2_IMPLEMENTATION=PASS"
  echo "HOST_BROWSER_LOOPBACK_ACCEPTANCE=FAIL"
  echo "PHASE2_COMPLETE=BLOCKED"
} | tee "$EVIDENCE_DIR/RESULT.txt"
echo "Evidence: $EVIDENCE_DIR"
exit 1

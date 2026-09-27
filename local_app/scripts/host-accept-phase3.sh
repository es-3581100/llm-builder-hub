#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="b662b28bba127c7493db15fcad2af65710eb923e"
PORT="${PORT:-18766}"
ADDR="127.0.0.1:${PORT}"
URL="http://${ADDR}"

for cmd in git go curl python3 sha256sum cmp; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

cd "$APP_DIR/.."
git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: HEAD is not descended from Phase-3 acceptance baseline $BASELINE" >&2
  exit 2
}
if [[ -n "$(git status --porcelain)" ]]; then
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

if [[ -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: no graphical DISPLAY/WAYLAND_DISPLAY detected" >&2
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
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: no supported graphical browser found; set BROWSER_CMD" >&2
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
printf 'browser original\n' >"$TEST_REPO/source.txt"
printf 'conflict original\n' >"$TEST_REPO/conflict-ui.txt"
printf 'mechanical original\n' >"$TEST_REPO/mechanical.txt"
printf '# Phase 3 Host Acceptance\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add source.txt conflict-ui.txt mechanical.txt README.md
git -C "$TEST_REPO" commit -m "phase3 host baseline" >/dev/null

cd "$APP_DIR"
go test ./...
go vet ./...
node --test tests/*.test.cjs
go build -o "$BIN" ./cmd/llm-hub-local

"$BIN" -addr "$ADDR" -state "$STATE" -repo "$TEST_REPO" >"$EVIDENCE_DIR/service.log" 2>&1 &
SERVICE_PID=$!
READY=0
for _ in $(seq 1 80); do
  if curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-initial.json" 2>/dev/null \
    && python3 - "$EVIDENCE_DIR/state-initial.json" >/dev/null 2>&1 <<'PY'
import json,sys
json.load(open(sys.argv[1]))
PY
  then
    READY=1
    break
  fi
  sleep 0.1
done
if [[ "$READY" -ne 1 ]]; then
  echo "PHASE3_HOST_ACCEPTANCE_BLOCKED: service did not become ready at $URL" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
fi

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

record_enter() {
  local prompt="$1" ignored
  read -r -p "$prompt [press Enter]: " ignored
}

make_write_request() {
  local state_file="$1" path="$2" replacement="$3" output="$4"
  python3 - "$state_file" "$path" "$replacement" "$output" <<'PY'
import json,sys
state_path,path,replacement,out=sys.argv[1:]
env=json.load(open(state_path))
repo=env["repository"]
doc=next(d for d in repo["documents"] if d["path"] == path)
json.dump({
  "repository_id": repo["repository_id"],
  "document_id": doc["id"],
  "path": path,
  "expected_content_sha256": doc["content_sha256"],
  "content": replacement,
}, open(out,"w"), separators=(",",":"))
PY
}

INITIAL_REV="$(json_value "$EVIDENCE_DIR/state-initial.json" state.revision)"
INITIAL_INDEX_SHA="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
INITIAL_HEAD="$(git -C "$TEST_REPO" rev-parse HEAD)"
SOURCE_INITIAL_SHA="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"

make_write_request "$EVIDENCE_DIR/state-initial.json" "mechanical.txt" $'mechanical written through API\n' "$EVIDENCE_DIR/mechanical-write-request.json"
MECH_HTTP="$(curl -sS -o "$EVIDENCE_DIR/mechanical-write-response.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  --data-binary @"$EVIDENCE_DIR/mechanical-write-request.json" \
  "$URL/api/write-file")"
[[ "$MECH_HTTP" == "200" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: mechanical write HTTP=$MECH_HTTP" >&2
  exit 1
}
printf 'mechanical written through API\n' >"$EVIDENCE_DIR/mechanical-expected.txt"
cmp -s "$EVIDENCE_DIR/mechanical-expected.txt" "$TEST_REPO/mechanical.txt" || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: mechanical write bytes mismatch" >&2
  exit 1
}
[[ "$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')" == "$INITIAL_INDEX_SHA" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: WRITE FILE mutated .git/index" >&2
  exit 1
}

python3 - "$EVIDENCE_DIR/state-initial.json" "$EVIDENCE_DIR/save-request.json" <<'PY'
import json,sys
env=json.load(open(sys.argv[1]))
state=env["state"]
state["project"]["name"]="phase3-save-separation-proof"
json.dump(state,open(sys.argv[2],"w"),separators=(",",":"))
PY
SOURCE_BEFORE_SAVE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
SAVE_HTTP="$(curl -sS -o "$EVIDENCE_DIR/save-response.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  --data-binary @"$EVIDENCE_DIR/save-request.json" \
  "$URL/api/save")"
[[ "$SAVE_HTTP" == "200" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: SAVE separation request HTTP=$SAVE_HTTP" >&2
  exit 1
}
SOURCE_AFTER_SAVE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
[[ "$SOURCE_BEFORE_SAVE" == "$SOURCE_AFTER_SAVE" ]] || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: SAVE CHANGES path mutated source.txt" >&2
  exit 1
}
SAVE_REV="$(json_value "$EVIDENCE_DIR/save-response.json" state.revision)"
(( SAVE_REV > INITIAL_REV )) || {
  echo "PHASE3_HOST_ACCEPTANCE_FAIL: workstation revision did not advance on SAVE" >&2
  exit 1
}

curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-before-browser.json"
BROWSER_REV_BEFORE="$(json_value "$EVIDENCE_DIR/state-before-browser.json" state.revision)"
INDEX_BEFORE_BROWSER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"

{
  echo "PHASE3_HOST_BROWSER_ACCEPTANCE"
  echo "baseline=$BASELINE"
  echo "tested_head=$(git -C "$APP_DIR/.." rev-parse HEAD)"
  echo "browser=$BROWSER_CMD"
  echo "url=$URL"
  echo "initial_head=$INITIAL_HEAD"
  echo "initial_revision=$INITIAL_REV"
  echo "revision_after_mechanical_save=$SAVE_REV"
  echo "initial_index_sha256=$INITIAL_INDEX_SHA"
  echo "source_initial_sha256=$SOURCE_INITIAL_SHA"
  echo "mechanical_write_http=$MECH_HTTP"
  echo "save_separation_http=$SAVE_HTTP"
} >"$EVIDENCE_DIR/acceptance.txt"

"$BROWSER_CMD" "$URL/" >/dev/null 2>&1 &

cat <<'EOF'

PHASE 3 — REAL BROWSER WRITE ACCEPTANCE
========================================

SUCCESSFUL SOURCE WRITE:
  1. Find repository document source.txt.
  2. Click EDIT SOURCE.
  3. Replace the entire textarea with exactly:

phase3 browser write
line two

  Include the final newline after "line two".
  4. Confirm SOURCE DRAFT is visible.
  5. While source.txt is still dirty, try EDIT SOURCE on conflict-ui.txt.
     Confirm the app blocks the switch and source.txt's exact draft remains.
  6. Click REFRESH and confirm it is BLOCKED and the source draft remains intact.
  7. Click WRITE FILE.
  8. Confirm source edit closes and source.txt now displays the exact replacement.
  9. Confirm the repository working-tree view shows source.txt modified.

Return to this terminal only after those steps.
EOF

PASS=1
record_check browser_source_edit "Did EDIT SOURCE create a separate editable repository source draft?" || PASS=0
record_check browser_source_switch_block "Did a dirty source draft block EDIT SOURCE on the other repository document?" || PASS=0
record_check browser_source_refresh_block "Did REFRESH block while SOURCE DRAFT existed and preserve the draft?" || PASS=0
record_check browser_write_success "Did WRITE FILE succeed and close source edit mode?" || PASS=0
record_check browser_fresh_git_diff "Did the fresh repository view show source.txt as an unstaged working-tree modification?" || PASS=0

printf 'phase3 browser write\nline two\n' >"$EVIDENCE_DIR/source-browser-expected.txt"
if ! cmp -s "$EVIDENCE_DIR/source-browser-expected.txt" "$TEST_REPO/source.txt"; then
  echo "browser_source_bytes=FAIL" >>"$EVIDENCE_DIR/browser-checks.txt"
  PASS=0
else
  echo "browser_source_bytes=PASS" >>"$EVIDENCE_DIR/browser-checks.txt"
fi
INDEX_AFTER_BROWSER_WRITE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
[[ "$INDEX_AFTER_BROWSER_WRITE" == "$INDEX_BEFORE_BROWSER" ]] || PASS=0
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-after-browser-write.json"
BROWSER_REV_AFTER_WRITE="$(json_value "$EVIDENCE_DIR/state-after-browser-write.json" state.revision)"
[[ "$BROWSER_REV_AFTER_WRITE" == "$BROWSER_REV_BEFORE" ]] || PASS=0
grep -F 'POST /api/write-file status=200 code=OK path="source.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || PASS=0

cat <<'EOF'

STALE-CONTENT CONFLICT:
  1. Find repository document conflict-ui.txt.
  2. Click EDIT SOURCE.
  3. Replace its textarea with exactly:

browser stale draft

  Include the final newline.
  4. DO NOT click WRITE FILE yet.
  5. Return to this terminal and press Enter.
EOF
record_enter "Ready for the harness to create an external edit to conflict-ui.txt?"

printf 'external edit wins\n' >"$TEST_REPO/conflict-ui.txt"
EXTERNAL_CONFLICT_SHA="$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')"

cat <<'EOF'

  The external edit now exists on disk.

  6. In the browser, click REFRESH.
     Confirm REFRESH is blocked and your "browser stale draft" remains visible.
  7. Click WRITE FILE.
  8. Confirm WRITE FILE is blocked with CONTENT_CONFLICT.
  9. Confirm the browser textarea still contains exactly "browser stale draft"
     and source edit mode remains open.

Return here after those checks.
EOF
record_check browser_conflict_refresh_block "Did REFRESH preserve the stale browser source draft?" || PASS=0
record_check browser_content_conflict "Did WRITE FILE report CONTENT_CONFLICT?" || PASS=0
record_check browser_conflict_draft_preserved "Did the exact browser stale draft remain editable after the conflict?" || PASS=0

[[ "$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')" == "$EXTERNAL_CONFLICT_SHA" ]] || PASS=0
printf 'external edit wins\n' >"$EVIDENCE_DIR/conflict-external-expected.txt"
cmp -s "$EVIDENCE_DIR/conflict-external-expected.txt" "$TEST_REPO/conflict-ui.txt" || PASS=0
grep -F 'POST /api/write-file status=409 code=CONTENT_CONFLICT path="conflict-ui.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || PASS=0

cat <<'EOF'

SAVE CHANGES SEPARATION:
  1. Click CANCEL SOURCE EDIT on conflict-ui.txt.
  2. Edit any workstation-authority document (not a repository source document).
  3. Click SAVE CHANGES.
  4. Confirm workstation revision advances.
  5. Confirm neither source.txt nor conflict-ui.txt changes because of SAVE CHANGES.

Return here after those steps.
EOF
SOURCE_BEFORE_BROWSER_SAVE="$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
CONFLICT_BEFORE_BROWSER_SAVE="$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')"
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-before-browser-save.json"
REV_BEFORE_BROWSER_SAVE="$(json_value "$EVIDENCE_DIR/state-before-browser-save.json" state.revision)"
record_check browser_save_separation "Did workstation SAVE CHANGES succeed without acting as WRITE FILE?" || PASS=0
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-after-browser-save.json"
REV_AFTER_BROWSER_SAVE="$(json_value "$EVIDENCE_DIR/state-after-browser-save.json" state.revision)"
(( REV_AFTER_BROWSER_SAVE > REV_BEFORE_BROWSER_SAVE )) || PASS=0
[[ "$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')" == "$SOURCE_BEFORE_BROWSER_SAVE" ]] || PASS=0
[[ "$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')" == "$CONFLICT_BEFORE_BROWSER_SAVE" ]] || PASS=0
FINAL_INDEX_SHA="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
[[ "$FINAL_INDEX_SHA" == "$INITIAL_INDEX_SHA" ]] || PASS=0

git -C "$TEST_REPO" diff --no-color --no-ext-diff --no-textconv -- >"$EVIDENCE_DIR/final-working-tree.diff"
git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/final-status.txt"
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-final.json"

{
  echo "browser_revision_before_write=$BROWSER_REV_BEFORE"
  echo "browser_revision_after_write=$BROWSER_REV_AFTER_WRITE"
  echo "browser_save_revision_before=$REV_BEFORE_BROWSER_SAVE"
  echo "browser_save_revision_after=$REV_AFTER_BROWSER_SAVE"
  echo "index_sha256_after=$FINAL_INDEX_SHA"
  echo "source_sha256_after=$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
  echo "conflict_sha256_after=$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')"
} >>"$EVIDENCE_DIR/acceptance.txt"

if [[ "$PASS" -eq 1 ]]; then
  {
    echo "PHASE3_BUILD_GATES=PASS"
    echo "PHASE3_BROWSER_WRITE_ACCEPTANCE=PASS"
    echo "PHASE3_CONFLICT_ACCEPTANCE=PASS"
    echo "PHASE3_SAVE_WRITE_SEPARATION=PASS"
    echo "PHASE3_HOST_ACCEPTANCE=PASS"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 0
fi

{
  echo "PHASE3_BUILD_GATES=PASS"
  echo "PHASE3_HOST_ACCEPTANCE=FAIL"
  echo "PHASE3_COMPLETE=BLOCKED"
} | tee "$EVIDENCE_DIR/RESULT.txt"
echo "Evidence: $EVIDENCE_DIR"
exit 1

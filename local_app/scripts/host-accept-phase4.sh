#!/usr/bin/env bash
# Phase 4 automated host acceptance.
#
# Builds a disposable fixture repository, starts the real loopback service,
# drives the real browser UI through the full local Git control chain, and then
# verifies the resulting Git and HTTP facts independently of the browser.
#
# Exit codes: 0 = every asserted property held, 2 = environment/infra problem
# (PHASE4_HOST_ACCEPTANCE_BLOCKED), 1 = an assertion failed
# (PHASE4_HOST_ACCEPTANCE_FAIL). The two are never conflated.
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(git -C "$APP_DIR" rev-parse --show-toplevel 2>/dev/null || true)"
BASELINE="f42531ddc654b6357c19bda1ffa507d560bdb523"
PORT="${PORT:-18767}"
ADDR="127.0.0.1:${PORT}"
URL="http://${ADDR}"

blocked() {
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: $*" >&2
  exit 2
}

failed() {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: $*" >&2
  exit 1
}

# Exact bytes and strings shared with the browser half. No trailing newlines are
# stripped: $'...' quoting preserves them, so the on-disk comparison is byte
# exact on both sides.
SOURCE_BASELINE=$'phase4 acceptance original\nsecond line\n'
EXPECTED_SOURCE=$'phase4 acceptance browser write\nline two\n'
UNRELATED_BASELINE=$'phase4 unrelated baseline\nkeep me unstaged\n'
EXTERNAL_REWRITE=$'phase4 external rewrite wins\n'
COMMIT_MESSAGE='phase4 acceptance commit 9f3a1c'

# Phase 4 forbids silent normalization, so a commit message that exists nowhere
# in the server log can be detected. It must exist somewhere in the run, or the
# absence assertion would be vacuous.
for cmd in git go node curl sha256sum grep sort awk mktemp date cmp; do
  command -v "$cmd" >/dev/null 2>&1 || blocked "missing command: $cmd"
done

PYTHON_BIN="${PHASE4_PYTHON:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  CANDIDATES=()
  if [[ -n "${CONDA_PREFIX:-}" ]]; then
    CANDIDATES+=("$CONDA_PREFIX/bin/python")
  fi
  CANDIDATES+=(python3 python)

  for candidate in "${CANDIDATES[@]}"; do
    if [[ "$candidate" == */* ]]; then
      [[ -x "$candidate" ]] || continue
      resolved="$candidate"
    else
      resolved="$(command -v "$candidate" 2>/dev/null || true)"
      [[ -n "$resolved" ]] || continue
    fi
    if "$resolved" -c 'from playwright.sync_api import sync_playwright' >/dev/null 2>&1; then
      PYTHON_BIN="$resolved"
      break
    fi
  done
fi

if [[ -z "$PYTHON_BIN" ]]; then
  echo "Tried CONDA_PREFIX/bin/python, python3, and python. Override with PHASE4_PYTHON=/path/to/python." >&2
  blocked "no Python interpreter with Playwright found"
fi

if ! "$PYTHON_BIN" -c 'from playwright.sync_api import sync_playwright' >/dev/null 2>&1; then
  blocked "$PYTHON_BIN cannot import Playwright"
fi

echo "PHASE4_PYTHON=$PYTHON_BIN"

[[ -n "$REPO_ROOT" ]] || blocked "could not resolve the Builder-Hub repository root"

cd "$REPO_ROOT"

git merge-base --is-ancestor "$BASELINE" HEAD || blocked "HEAD is not descended from the Phase-3 completion baseline $BASELINE"

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
  blocked "Builder-Hub worktree must be clean"
  git status --short >&2
fi

# Property 13: the Builder-Hub repository is observed, never touched.
HUB_HEAD_BEFORE="$(git rev-parse HEAD)"
HUB_STATUS_BEFORE="$(git status --porcelain=v1 --untracked-files=all)"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${PHASE4_EVIDENCE_DIR:-${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/phase4-host-acceptance/${STAMP}}}"
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/llm-hub-phase4-auto.XXXXXX")"
SERVICE_PID=""
cleanup() {
  if [[ -n "$SERVICE_PID" ]]; then
    kill "$SERVICE_PID" 2>/dev/null || true
    wait "$SERVICE_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# A read-only Git observer. GIT_OPTIONAL_LOCKS=0 keeps `git status` from
# refreshing and rewriting .git/index, so an observed index identity only
# changes when a real index mutation happened.
git_ro() {
  GIT_OPTIONAL_LOCKS=0 git -C "$TEST_REPO" "$@"
}

ORIGIN="$TMP/origin.git"
TEST_REPO="$TMP/repository with spaces"
STATE="$TMP/workstation-state.json"
BIN="$TMP/llm-hub-local"

git init --bare -b main "$ORIGIN" >/dev/null
git init -b main "$TEST_REPO" >/dev/null
git -C "$TEST_REPO" config user.name "Phase Four Automated Acceptance"
git -C "$TEST_REPO" config user.email "phase4-auto@example.invalid"
printf '%s' "$SOURCE_BASELINE" >"$TEST_REPO/source.txt"
printf '%s' "$UNRELATED_BASELINE" >"$TEST_REPO/unrelated.txt"
printf '# Phase 4 Automated Acceptance\n\nDisposable fixture for local Git commit control.\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add -A
git -C "$TEST_REPO" commit -m "phase4 automated host baseline" >/dev/null
git -C "$TEST_REPO" remote add origin "$ORIGIN"
git -C "$TEST_REPO" push -q origin main

# The unrelated change is worktree-only: it is never staged and never committed.
printf '%s' "$UNRELATED_BASELINE$EXTERNAL_REWRITE" >"$TEST_REPO/unrelated.txt"

git_ro status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/fixture-status-before.txt"
git_ro diff --name-only >"$EVIDENCE_DIR/fixture-unstaged-before.txt"

HEAD_BEFORE="$(git_ro rev-parse HEAD)"
BRANCH_BEFORE="$(git_ro branch --show-current)"
BRANCH_REF_BEFORE="$(git_ro symbolic-ref --quiet HEAD)"
INDEX_SHA_BEFORE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
REMOTE_V_BEFORE="$(git_ro remote -v)"
REMOTE_REFS_BEFORE="$(git_ro for-each-ref --format='%(objectname) %(refname)' refs/remotes)"
ORIGIN_REFS_BEFORE="$(git_ro --git-dir="$ORIGIN" for-each-ref --format='%(objectname) %(refname)')"

# A non-vacuous remote assertion needs a real remote-tracking ref to exist, and
# it must still point at the pre-commit HEAD after a successful local commit.
[[ "$REMOTE_REFS_BEFORE" == "$HEAD_BEFORE refs/remotes/origin/main" ]] || failed "unexpected remote-tracking ref state: $REMOTE_REFS_BEFORE"
[[ "$ORIGIN_REFS_BEFORE" == "$HEAD_BEFORE refs/heads/main" ]] || failed "unexpected bare origin ref state: $ORIGIN_REFS_BEFORE"

[[ "$REMOTE_V_BEFORE" == *"$(basename "$ORIGIN")"* ]] || failed "fixture origin remote is not configured as expected"
[[ "$(git_ro diff --name-only)" == "unrelated.txt" ]] || failed "fixture did not start with exactly one unrelated unstaged change"
[[ -z "$(git_ro diff --cached --name-only)" ]] || failed "fixture did not start with a clean index"

cd "$APP_DIR"
./scripts/verify-phase4.sh

go build -o "$BIN" ./cmd/llm-hub-local
"$BIN" -addr "$ADDR" -state "$STATE" -repo "$TEST_REPO" >"$EVIDENCE_DIR/service.log" 2>&1 &
SERVICE_PID=$!

READY=0
for _ in $(seq 1 100); do
  if kill -0 "$SERVICE_PID" 2>/dev/null && curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-before.json" 2>/dev/null && "$PYTHON_BIN" - "$EVIDENCE_DIR/state-before.json" >/dev/null 2>&1 <<'PY'
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
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: loopback service did not become ready at $URL" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
fi

set +e
PHASE4_EXPECTED_SOURCE="$EXPECTED_SOURCE" \
  PHASE4_SOURCE_BASELINE="$SOURCE_BASELINE" \
  PHASE4_UNRELATED_BASELINE="$UNRELATED_BASELINE" \
  PHASE4_EXTERNAL_REWRITE="$EXTERNAL_REWRITE" \
  PHASE4_COMMIT_MESSAGE="$COMMIT_MESSAGE" \
  "$PYTHON_BIN" "$APP_DIR/scripts/host-accept-phase4.py" "$URL" "$TEST_REPO" "$EVIDENCE_DIR"
PYTHON_STATUS=$?
set -e

OBS="$EVIDENCE_DIR/browser-automation.json"
if [[ "$PYTHON_STATUS" -eq 2 ]]; then
  blocked "browser automation environment problem (see $OBS)"
fi
if [[ "$PYTHON_STATUS" -ne 0 ]]; then
  failed "browser automation assertions failed (see $OBS)"
fi
[[ -s "$OBS" ]] || blocked "browser automation produced no evidence file"

json_get() {
  "$PYTHON_BIN" - "$OBS" "$1" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
for key in sys.argv[2].split("."):
    try:
        data = data[key]
    except (KeyError, IndexError, TypeError) as exc:
        available = sorted(data) if isinstance(data, dict) else type(data).__name__
        print(
            f"json_get: path {sys.argv[2]!r} has no key {key!r} in {sys.argv[1]}: {exc!r}; available={available}",
            file=sys.stderr,
        )
        raise SystemExit(1)
if isinstance(data, bool):
    print(str(data).lower())
elif isinstance(data, (dict, list)):
    print(json.dumps(data, sort_keys=True))
else:
    print(data)
PY
}

# A browser check is only honored when it is present and explicitly true.
assert_browser_check() {
  local name="$1"
  "$PYTHON_BIN" - "$OBS" "$name" <<'PY' || failed "browser check not proven: $name"
import json, sys
data = json.load(open(sys.argv[1]))
entry = (data.get("checks") or {}).get(sys.argv[2])
if not entry or entry.get("pass") is not True:
    print(f"browser check {sys.argv[2]!r} is missing or not passing: {entry!r}", file=sys.stderr)
    raise SystemExit(1)
PY
}

RESULTS=()
pass_property() {
  RESULTS+=("$1=PASS")
  echo "$1=PASS"
}

pass_property PHASE4_LOCAL_VERIFY
pass_property PHASE4_BROWSER_AUTOMATION

# ---- Independent shell-side Git assertions --------------------------------
# The index identity is read before any other Git call, so only a real index
# mutation can have changed it.
INDEX_SHA_AFTER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
HEAD_AFTER="$(git_ro rev-parse HEAD)"
BRANCH_AFTER="$(git_ro branch --show-current)"
BRANCH_REF_AFTER="$(git_ro symbolic-ref --quiet HEAD)"
STAGED_AFTER="$(git_ro diff --cached --name-only)"
UNSTAGED_AFTER="$(git_ro diff --name-only)"
REMOTE_V_AFTER="$(git_ro remote -v)"
REMOTE_REFS_AFTER="$(git_ro for-each-ref --format='%(objectname) %(refname)' refs/remotes)"
ORIGIN_REFS_AFTER="$(git_ro --git-dir="$ORIGIN" for-each-ref --format='%(objectname) %(refname)')"

{
  echo "head_before=$HEAD_BEFORE"
  echo "head_after=$HEAD_AFTER"
  echo "branch_before=$BRANCH_BEFORE"
  echo "branch_after=$BRANCH_AFTER"
  echo "branch_ref_before=$BRANCH_REF_BEFORE"
  echo "branch_ref_after=$BRANCH_REF_AFTER"
  echo "index_sha_before=$INDEX_SHA_BEFORE"
  echo "index_sha_after=$INDEX_SHA_AFTER"
  echo "staged_after<<EOF"
  printf '%s\n' "$STAGED_AFTER"
  echo "EOF"
  echo "unstaged_after<<EOF"
  printf '%s\n' "$UNSTAGED_AFTER"
  echo "EOF"
  echo "remote_v_before<<EOF"
  printf '%s\n' "$REMOTE_V_BEFORE"
  echo "EOF"
  echo "remote_v_after<<EOF"
  printf '%s\n' "$REMOTE_V_AFTER"
  echo "EOF"
  echo "remote_refs_before<<EOF"
  printf '%s\n' "$REMOTE_REFS_BEFORE"
  echo "EOF"
  echo "remote_refs_after<<EOF"
  printf '%s\n' "$REMOTE_REFS_AFTER"
  echo "EOF"
  echo "origin_refs_before<<EOF"
  printf '%s\n' "$ORIGIN_REFS_BEFORE"
  echo "EOF"
  echo "origin_refs_after<<EOF"
  printf '%s\n' "$ORIGIN_REFS_AFTER"
  echo "EOF"
} >"$EVIDENCE_DIR/git-observations.txt"

# Property 1: worktree bytes after WRITE FILE, compared byte for byte.
printf '%s' "$EXPECTED_SOURCE" >"$EVIDENCE_DIR/source-expected.txt"
cmp -s "$EVIDENCE_DIR/source-expected.txt" "$TEST_REPO/source.txt" || failed "source.txt bytes do not match the expected WRITE FILE content"
assert_browser_check worktree_bytes_after_write_file
pass_property PHASE4_WRITE_FILE_ACCEPTANCE

# Property 2: index SHA-256 changed on stage and again on unstage, and the
# values the API reported match the index on disk.
STAGE1_INDEX="$(json_get stage.after_index_sha256)"
UNSTAGE_INDEX="$(json_get unstage.after_index_sha256)"
RESTAGE_INDEX="$(json_get restage.after_index_sha256)"
COMMIT_INDEX="$(json_get commit.after_index_sha256)"
[[ "$STAGE1_INDEX" != "$INDEX_SHA_BEFORE" ]] || failed "index identity did not change on STAGE FILE"
[[ "$UNSTAGE_INDEX" != "$STAGE1_INDEX" ]] || failed "index identity did not change on UNSTAGE FILE"
[[ "$RESTAGE_INDEX" != "$UNSTAGE_INDEX" ]] || failed "index identity did not change on the second STAGE FILE"
[[ "$RESTAGE_INDEX" == "$COMMIT_INDEX" && "$COMMIT_INDEX" == "$INDEX_SHA_AFTER" ]] || failed "index identity drifted outside index mutations: staged=$RESTAGE_INDEX commit=$COMMIT_INDEX disk=$INDEX_SHA_AFTER"
assert_browser_check stage_index_sha_matches_disk
assert_browser_check unstage_index_sha_matches_disk
pass_property PHASE4_INDEX_IDENTITY

# Property 3 and 4: HEAD does not move through stage or unstage, and moves only
# on commit, to the exact commit the API reported, with exactly one parent that
# is the pre-commit HEAD.
API_NEW_HEAD="$(json_get commit.new_head_commit)"
API_PARENT="$(json_get commit.parent_commit)"
PRE_COMMIT_HEAD="$(json_get pre_commit_head)"
[[ "$HEAD_AFTER" == "$API_NEW_HEAD" ]] || failed "HEAD $HEAD_AFTER does not equal the API new_head_commit $API_NEW_HEAD"
read -r -a PARENT_FIELDS <<<"$(git_ro rev-list --parents -n 1 "$HEAD_AFTER")"
[[ "${#PARENT_FIELDS[@]}" -eq 2 ]] || failed "new commit is not a one-parent commit: ${PARENT_FIELDS[*]}"
[[ "${PARENT_FIELDS[1]}" == "$PRE_COMMIT_HEAD" ]] || failed "new commit parent ${PARENT_FIELDS[1]} is not the pre-commit HEAD $PRE_COMMIT_HEAD"
[[ "$API_PARENT" == "$PRE_COMMIT_HEAD" ]] || failed "API parent $API_PARENT is not the pre-commit HEAD $PRE_COMMIT_HEAD"
[[ "$HEAD_AFTER" != "$HEAD_BEFORE" ]] || failed "HEAD did not change across a successful commit"
assert_browser_check commit_one_parent_is_previous_head
assert_browser_check commit_object_has_exactly_one_parent
pass_property PHASE4_HEAD_IDENTITY

# The committed snapshot really is the staged snapshot, and the unrelated
# worktree change is really absent from it.
git_ro show "$HEAD_AFTER:source.txt" >"$EVIDENCE_DIR/committed-source.txt"
cmp -s "$EVIDENCE_DIR/source-expected.txt" "$EVIDENCE_DIR/committed-source.txt" || failed "the commit does not contain the bytes that were staged"
git_ro show "$HEAD_AFTER:unrelated.txt" >"$EVIDENCE_DIR/committed-unrelated.txt"
cmp -s <(printf '%s' "$UNRELATED_BASELINE") "$EVIDENCE_DIR/committed-unrelated.txt" || failed "the commit contains the unrelated file, which was never staged"
[[ "$(git_ro show --format=%s --no-patch "$HEAD_AFTER")" == "$COMMIT_MESSAGE" ]] || failed "commit subject is not the exact supplied message"
assert_browser_check commit_advances_head_only
assert_browser_check commit_reports_settled_index_identity
pass_property PHASE4_STAGE_ACCEPTANCE
pass_property PHASE4_UNSTAGE_ACCEPTANCE
pass_property PHASE4_COMMIT_ACCEPTANCE

# Property 5: the branch is the same before and after the whole run.
[[ "$BRANCH_AFTER" == "$BRANCH_BEFORE" ]] || failed "branch changed: $BRANCH_BEFORE -> $BRANCH_AFTER"
[[ "$BRANCH_REF_AFTER" == "$BRANCH_REF_BEFORE" ]] || failed "branch ref changed: $BRANCH_REF_BEFORE -> $BRANCH_REF_AFTER"
[[ "$BRANCH_AFTER" == "main" ]] || failed "fixture branch is not main"
assert_browser_check commit_keeps_branch
pass_property PHASE4_BRANCH_STABLE

# Property 6: no remote mutation. The fixture has a real bare origin with a real
# remote-tracking ref, so all three comparisons are non-vacuous.
[[ "$REMOTE_V_AFTER" == "$REMOTE_V_BEFORE" ]] || failed "git remote -v changed"
[[ "$REMOTE_REFS_AFTER" == "$REMOTE_REFS_BEFORE" ]] || failed "refs/remotes changed: before=$REMOTE_REFS_BEFORE after=$REMOTE_REFS_AFTER"
[[ "$ORIGIN_REFS_AFTER" == "$ORIGIN_REFS_BEFORE" ]] || failed "bare origin refs changed"
[[ "$REMOTE_REFS_AFTER" == *"$HEAD_BEFORE"* ]] || failed "the remote no longer points at the pre-commit HEAD $HEAD_BEFORE, so the new local commit was pushed: $REMOTE_REFS_AFTER"
assert_browser_check no_push_control
pass_property PHASE4_REMOTE_IMMUTABLE

# Property 7: the committed snapshot consumed the index; the unrelated unstaged
# change survived it.
[[ -z "$STAGED_AFTER" ]] || failed "staged changes remain after the commit: $STAGED_AFTER"
[[ "$UNSTAGED_AFTER" == "unrelated.txt" ]] || failed "unexpected unstaged paths after the commit: $UNSTAGED_AFTER"
cmp -s <(printf '%s' "$EXTERNAL_REWRITE") "$TEST_REPO/unrelated.txt" || failed "the unrelated unstaged change did not survive the commit"
assert_browser_check commit_consumes_the_staged_snapshot
assert_browser_check staged_count_zero_after_commit
assert_browser_check unrelated_unstaged_survives_commit
pass_property PHASE4_COMMITTED_SNAPSHOT_CONSUMED

# Property 8: SAVE CHANGES, WRITE FILE, STAGE FILE, and the commit textarea are
# four separate authority layers.
for name in \
  save_changes_leaves_git_untouched \
  save_changes_left_source_baseline \
  commit_message_keeps_workstation_saved \
  commit_message_kept_after_typing \
  commit_message_keeps_workstation_saved_again \
  commit_message_kept_verbatim \
  commit_never_dirties_workstation \
  write_file_does_not_stage \
  stage_does_not_commit \
  commit_button_disabled_again; do
  assert_browser_check "$name"
done
[[ "$(json_get commit.expected_head_commit)" == "$PRE_COMMIT_HEAD" ]] || failed "the commit was not bound to the pre-commit HEAD"
pass_property PHASE4_AUTHORITY_SEPARATION
pass_property PHASE4_SAVE_WRITE_SEPARATION

# Property 9: a dirty source draft blocks stage, unstage, and commit with a
# visible dialog, issues no request, and mutates no Git state.
for name in \
  dialog_stage_blocked_no_request \
  dialog_unstage_blocked_no_request \
  dialog_commit_blocked_no_request \
  draft_blocks_stage_without_mutation \
  draft_blocks_unstage_without_mutation \
  draft_blocks_commit_without_mutation \
  draft_block_short_circuits_before_request \
  draft_block_unstage_no_error_and_no_request \
  draft_block_commit_no_error_and_no_request \
  commit_message_survives_blocked_commit \
  cancel_draft_writes_nothing; do
  assert_browser_check "$name"
done
BLOCK_DIALOGS="$(json_get dialogs)"
for fragment in 'SOURCE DRAFT exists' 'before STAGE FILE' 'before UNSTAGE FILE' 'before COMMIT STAGED'; do
  case "$BLOCK_DIALOGS" in
    *"$fragment"*) ;;
    *) failed "no dialog carried the expected fragment: $fragment" ;;
  esac
done
pass_property PHASE4_DRAFT_BLOCK

# Property 10: the 409 stays visible, is not a dialog, and does not clobber.
[[ "$(json_get conflict.code)" == "WORKTREE_CONFLICT" ]] || failed "the staged conflict did not return WORKTREE_CONFLICT"
for name in \
  conflict_http_409 \
  conflict_code_visible \
  conflict_status_visible \
  conflict_action_visible \
  conflict_api_code \
  conflict_does_not_replace_local_snapshot \
  conflict_preserves_external_bytes \
  conflict_performs_no_git_mutation; do
  assert_browser_check "$name"
done
CONFLICT_DIALOG_COUNT="$("$PYTHON_BIN" - "$OBS" <<'PY'
import json, sys
data = json.load(open(sys.argv[1]))
conflict = [d for d in data.get("dialogs", []) if "CONFLICT" in d or "conflict" in d]
print(len(conflict))
PY
)"
[[ "$CONFLICT_DIALOG_COUNT" == "0" ]] || failed "a 409 conflict raised a dialog; the design is a visible #commit-error only"
pass_property PHASE4_CONFLICT_VISIBILITY

# Property 11: the service log carries identities and status, and neither file
# contents nor the commit message.
grep -F 'POST /api/stage-file status=200 code=OK path="source.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || failed "successful stage log line missing"
grep -F 'POST /api/unstage-file status=200 code=OK path="source.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || failed "successful unstage log line missing"
grep -F 'POST /api/commit status=200 code=OK' "$EVIDENCE_DIR/service.log" >/dev/null || failed "successful commit log line missing"
grep -F 'POST /api/stage-file status=409 code=WORKTREE_CONFLICT path="unrelated.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || failed "worktree conflict log line missing"
[[ "$(grep -cF 'POST /api/stage-file status=200 code=OK path="source.txt"' "$EVIDENCE_DIR/service.log")" == "2" ]] || failed "expected exactly two successful stage log lines for source.txt"
for leaked in 'phase4 acceptance browser write' 'line two' 'phase4 external rewrite wins' "$COMMIT_MESSAGE" 'keep me unstaged'; do
  if grep -Fq "$leaked" "$EVIDENCE_DIR/service.log"; then
    failed "service log leaked user content: $leaked"
  fi
done
# The absence assertions above are only meaningful if the content existed.
grep -Fq 'phase4 acceptance browser write' "$EVIDENCE_DIR/committed-source.txt" || failed "the committed blob does not contain the staged content, so the log-leak check would be vacuous"
# Only the local Git control log lines carry these four identities. The
# POST /api/write-file success line also carries 'status=200 code=OK' but
# names before_sha256/after_sha256 instead, so it is filtered out here.
LOG_FIELD_LINES=0
while IFS= read -r line; do
  LOG_FIELD_LINES=$((LOG_FIELD_LINES + 1))
  for field in before_index_sha256 after_index_sha256 expected_head_commit new_head_commit; do
    # Tolerated only so the assertion below can report the missing field by
    # name; the assertion itself has no escape hatch.
    value="$(printf '%s\n' "$line" | grep -oE "[[:space:]]${field}=[^[:space:]]+" | head -n1 | sed "s/^[[:space:]]*${field}=//" || true)"
    case "$field" in
      expected_head_commit | new_head_commit)
        [[ "$value" =~ ^[0-9a-f]{40}$ || "$value" =~ ^[0-9a-f]{64}$ ]] || failed "log field $field is not a hex object id: '$value' in: $line"
        ;;
      *)
        [[ "$value" =~ ^[0-9a-f]{64}$ ]] || failed "log field $field is not a sha256: '$value' in: $line"
        ;;
    esac
  done
done < <(grep -F 'status=200 code=OK' "$EVIDENCE_DIR/service.log" | grep -F 'before_index_sha256=')
[[ "$LOG_FIELD_LINES" -gt 0 ]] || failed "no successful local Git control log line carried index and head identities"
pass_property PHASE4_LOG_REDACTION

# Property 12: the static export is still read-only.
curl -fsS "$URL/api/export" >"$EVIDENCE_DIR/llm-hub-project.html"
for forbidden in 'STAGE FILE' 'UNSTAGE FILE' 'COMMIT STAGED' '/api/stage-file' '/api/unstage-file' '/api/commit' '<textarea' 'WRITE FILE' 'EDIT SOURCE' 'CANCEL SOURCE EDIT' '/api/write-file'; do
  if grep -Fq "$forbidden" "$EVIDENCE_DIR/llm-hub-project.html"; then
    failed "static export contains mutation surface: $forbidden"
  fi
done
grep -Fq 'data-static-projection="true"' "$EVIDENCE_DIR/llm-hub-project.html" || failed "static export is not marked read-only"
grep -Fq 'READ ONLY' "$EVIDENCE_DIR/llm-hub-project.html" || failed "static export lost its read-only badge"
pass_property PHASE4_EXPORT_READ_ONLY

# ---- Evidence and the final Builder-Hub check ------------------------------
git_ro status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/final-status.txt"
git_ro diff --no-color --no-ext-diff --no-textconv -- >"$EVIDENCE_DIR/final-working-tree.diff"
git_ro diff --cached --no-color --no-ext-diff --no-textconv -- >"$EVIDENCE_DIR/final-staged.diff"
git_ro log --format='%H %P %an %s' -n 3 >"$EVIDENCE_DIR/final-history.txt"

mkdir -p "$EVIDENCE_DIR/content"
printf '%s' "$SOURCE_BASELINE" >"$EVIDENCE_DIR/content/source-baseline.txt"
printf '%s' "$EXPECTED_SOURCE" >"$EVIDENCE_DIR/content/expected-source.txt"
printf '%s' "$UNRELATED_BASELINE" >"$EVIDENCE_DIR/content/unrelated-baseline.txt"
printf '%s' "$EXTERNAL_REWRITE" >"$EVIDENCE_DIR/content/external-rewrite.txt"
printf '%s' "$COMMIT_MESSAGE" >"$EVIDENCE_DIR/content/commit-message.txt"

{
  echo "tested_head=$HUB_HEAD_BEFORE"
  echo "tested_branch=$(git rev-parse --abbrev-ref HEAD)"
  echo "baseline_ancestor=$BASELINE"
  echo "url=$URL"
  echo "test_repo=$TEST_REPO"
  echo "origin=$ORIGIN"
  echo "python=$PYTHON_BIN"
  echo "source_baseline_file=content/source-baseline.txt"
  echo "expected_source_file=content/expected-source.txt"
  echo "unrelated_baseline_file=content/unrelated-baseline.txt"
  echo "external_rewrite_file=content/external-rewrite.txt"
  echo "commit_message_file=content/commit-message.txt"
  echo "stage1_index_sha256=$STAGE1_INDEX"
  echo "unstage_index_sha256=$UNSTAGE_INDEX"
  echo "restage_index_sha256=$RESTAGE_INDEX"
  echo "commit_index_sha256=$COMMIT_INDEX"
  echo "new_head_commit=$API_NEW_HEAD"
  echo "pre_commit_head=$PRE_COMMIT_HEAD"
} >"$EVIDENCE_DIR/acceptance.txt"

# Property 13: the Builder-Hub repository is byte-identical after the run.
HUB_HEAD_AFTER="$(git rev-parse HEAD)"
HUB_STATUS_AFTER="$(git status --porcelain=v1 --untracked-files=all)"
[[ "$HUB_HEAD_AFTER" == "$HUB_HEAD_BEFORE" ]] || failed "the acceptance changed the Builder-Hub HEAD: $HUB_HEAD_BEFORE -> $HUB_HEAD_AFTER"
[[ "$HUB_STATUS_AFTER" == "$HUB_STATUS_BEFORE" ]] || failed "the acceptance changed the Builder-Hub worktree"
pass_property PHASE4_HUB_REPO_UNTOUCHED

pass_property PHASE4_HOST_ACCEPTANCE
pass_property PHASE4_COMPLETE

{
  printf '%s\n' "${RESULTS[@]}"
} >"$EVIDENCE_DIR/RESULT.txt"
cat "$EVIDENCE_DIR/RESULT.txt"

echo "Evidence: $EVIDENCE_DIR"

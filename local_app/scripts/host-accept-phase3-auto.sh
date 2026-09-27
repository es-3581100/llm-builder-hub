#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT="$(cd "$APP_DIR/.." && pwd)"
BASELINE="a01b77ace6ff972b9077a764c0b069ffd54bf6e9"
PORT="${PORT:-18767}"
URL="http://127.0.0.1:${PORT}"
PYTHON="${PYTHON:-python3}"

for cmd in git go curl sha256sum cmp "$PYTHON"; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

cd "$ROOT"
git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: HEAD is not descended from $BASELINE" >&2
  exit 2
}
if [[ -n "$(git status --porcelain)" ]]; then
  echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

BROWSER_EXECUTABLE="${PLAYWRIGHT_CHROMIUM_EXECUTABLE:-}"
if [[ -z "$BROWSER_EXECUTABLE" ]]; then
  for candidate in thorium-browser chromium chromium-browser google-chrome google-chrome-stable; do
    if command -v "$candidate" >/dev/null 2>&1; then
      BROWSER_EXECUTABLE="$(command -v "$candidate")"
      break
    fi
  done
fi
if [[ -z "$BROWSER_EXECUTABLE" ]]; then
  echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: no Chromium-family executable found" >&2
  exit 2
fi
if ! "$PYTHON" -c 'import playwright.sync_api' >/dev/null 2>&1; then
  echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: python Playwright unavailable for $PYTHON" >&2
  exit 2
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/phase3-auto-acceptance/${STAMP}}"
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"
case "$(cd "$EVIDENCE_DIR" && pwd -P)/" in
  "$(cd "$ROOT" && pwd -P)/"*)
    echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: evidence directory must live outside the repository: $EVIDENCE_DIR" >&2
    exit 2
    ;;
esac

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

TEST_REPO="$TMP/repository"
STATE="$TMP/workstation-state.json"
BIN="$TMP/llm-hub-local"
mkdir -p "$TEST_REPO"

git -C "$TEST_REPO" init -b main >/dev/null
git -C "$TEST_REPO" config user.name "Phase Three Autonomous Acceptance"
git -C "$TEST_REPO" config user.email "phase3-auto@example.invalid"
printf 'browser original\n' >"$TEST_REPO/source.txt"
printf 'conflict original\n' >"$TEST_REPO/conflict-ui.txt"
printf 'mechanical original\n' >"$TEST_REPO/mechanical.txt"
printf '# Autonomous acceptance\n' >"$TEST_REPO/README.md"
git -C "$TEST_REPO" add source.txt conflict-ui.txt mechanical.txt README.md
git -C "$TEST_REPO" commit -m "acceptance baseline" >/dev/null

INITIAL_INDEX_SHA="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"

cd "$APP_DIR"
./scripts/verify-phase3.sh
go build -o "$BIN" ./cmd/llm-hub-local

"$BIN" -addr "127.0.0.1:${PORT}" -state "$STATE" -repo "$TEST_REPO" >"$EVIDENCE_DIR/service.log" 2>&1 &
SERVICE_PID=$!
READY=0
for _ in $(seq 1 100); do
  if curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-initial.json" 2>/dev/null; then
    READY=1
    break
  fi
  sleep 0.1
done
if [[ "$READY" -ne 1 ]]; then
  echo "PHASE3_AUTO_ACCEPTANCE_BLOCKED: service did not become ready" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
fi

set +e
"$PYTHON" "$APP_DIR/scripts/phase3-browser-accept.py" \
  --url "$URL/" \
  --repo "$TEST_REPO" \
  --evidence-dir "$EVIDENCE_DIR" \
  --browser-executable "$BROWSER_EXECUTABLE"
DRIVER_RC=$?
set -e

if [[ "$DRIVER_RC" -eq 2 ]]; then
  {
    echo "PHASE3_BUILD_GATES=PASS"
    echo "PHASE3_AUTONOMOUS_HOST_ACCEPTANCE=BLOCKED"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 2
fi
if [[ "$DRIVER_RC" -ne 0 ]]; then
  {
    echo "PHASE3_BUILD_GATES=PASS"
    echo "PHASE3_AUTONOMOUS_HOST_ACCEPTANCE=FAIL"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 1
fi

FINAL_INDEX_SHA="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
[[ "$FINAL_INDEX_SHA" == "$INITIAL_INDEX_SHA" ]] || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: .git/index changed" >&2
  exit 1
}

printf 'phase3 browser write\nline two\n' >"$EVIDENCE_DIR/source-expected.txt"
printf 'external edit wins\n' >"$EVIDENCE_DIR/conflict-expected.txt"
cmp -s "$EVIDENCE_DIR/source-expected.txt" "$TEST_REPO/source.txt" || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: source.txt final bytes mismatch" >&2
  exit 1
}
cmp -s "$EVIDENCE_DIR/conflict-expected.txt" "$TEST_REPO/conflict-ui.txt" || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: conflict-ui.txt final bytes mismatch" >&2
  exit 1
}

grep -E 'POST /api/write-file status=200 code=OK path="source\.txt" before_sha256=[0-9a-f]{64} after_sha256=[0-9a-f]{64}' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: missing successful source write log with before/after content hashes" >&2
  exit 1
}
grep -F 'POST /api/write-file status=409 code=CONTENT_CONFLICT path="conflict-ui.txt"' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: missing CONTENT_CONFLICT log" >&2
  exit 1
}
grep -F 'POST /api/save status=200' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: missing workstation SAVE log" >&2
  exit 1
}

for leaked in 'phase3 browser write' 'line two' 'browser stale draft' 'external edit wins'; do
  if grep -Fq "$leaked" "$EVIDENCE_DIR/service.log"; then
    echo "PHASE3_AUTO_ACCEPTANCE_FAIL: service log leaked replacement content: $leaked" >&2
    exit 1
  fi
done

git -C "$TEST_REPO" diff --no-color --no-ext-diff --no-textconv -- >"$EVIDENCE_DIR/final-working-tree.diff"
git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/final-status.txt"
curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-final.json"

"$PYTHON" - "$EVIDENCE_DIR/browser-report.json" <<'PY'
import json,sys
report=json.load(open(sys.argv[1]))
required=[
  "source_edit",
  "source_switch_block",
  "source_refresh_block",
  "write_success",
  "write_revision_unchanged",
  "content_conflict",
  "conflict_draft_preserved",
  "cancel_nonmutating",
  "save_write_separation",
]
missing=[key for key in required if report.get("checks",{}).get(key) is not True]
if report.get("result") != "PASS" or missing:
    raise SystemExit(f"browser report incomplete: result={report.get('result')} missing={missing}")
PY

if [[ -n "$(git -C "$ROOT" status --porcelain)" ]]; then
  echo "PHASE3_AUTO_ACCEPTANCE_FAIL: acceptance left the repository worktree dirty" >&2
  git -C "$ROOT" status --short >&2
  exit 1
fi

{
  echo "tested_head=$(git -C "$ROOT" rev-parse HEAD)"
  echo "browser_executable=$BROWSER_EXECUTABLE"
  echo "python=$PYTHON"
  echo "index_sha256_before=$INITIAL_INDEX_SHA"
  echo "index_sha256_after=$FINAL_INDEX_SHA"
  echo "source_sha256=$(sha256sum "$TEST_REPO/source.txt" | awk '{print $1}')"
  echo "conflict_sha256=$(sha256sum "$TEST_REPO/conflict-ui.txt" | awk '{print $1}')"
} >"$EVIDENCE_DIR/acceptance.txt"

cat >"$EVIDENCE_DIR/RESULT.txt" <<'EOF'
PHASE3_BUILD_GATES=PASS
PHASE3_BROWSER_WRITE_ACCEPTANCE=PASS
PHASE3_CONFLICT_ACCEPTANCE=PASS
PHASE3_SAVE_WRITE_SEPARATION=PASS
PHASE3_AUTONOMOUS_HOST_ACCEPTANCE=PASS
EOF
cat "$EVIDENCE_DIR/RESULT.txt"
echo "Evidence: $EVIDENCE_DIR"

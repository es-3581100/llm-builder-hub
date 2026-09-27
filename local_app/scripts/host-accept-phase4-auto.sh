#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT="$(cd "$APP_DIR/.." && pwd)"
BASELINE="f94cdb9c66eac6b3fd545ce58937d94e3f803a2a"
PORT="${PORT:-18768}"
URL="http://127.0.0.1:${PORT}"
PYTHON="${PYTHON:-python3}"
MODEL="${PHASE4_MODEL:-opencode/space-bunny-free}"
VARIANT="${PHASE4_VARIANT:-max}"
TIMEOUT_SECONDS="${PHASE4_TIMEOUT_SECONDS:-300}"

for cmd in git go curl sha256sum cmp stat "$PYTHON" opencode; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: missing command: $cmd" >&2
    exit 2
  }
done

cd "$ROOT"
git merge-base --is-ancestor "$BASELINE" HEAD || {
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: HEAD is not descended from accepted Phase-3 baseline $BASELINE" >&2
  exit 2
}
if [[ -n "$(git status --porcelain --untracked-files=all)" ]]; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean" >&2
  git status --short >&2
  exit 2
fi

if ! opencode run --help 2>&1 | grep -F -- '--model' >/dev/null; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: installed opencode run --help lacks --model" >&2
  exit 2
fi
if ! opencode run --help 2>&1 | grep -F -- '--dir' >/dev/null; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: installed opencode run --help lacks --dir" >&2
  exit 2
fi
if [[ -n "$VARIANT" ]] && ! opencode run --help 2>&1 | grep -F -- '--variant' >/dev/null; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: installed opencode run --help lacks --variant" >&2
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
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: no Chromium-family browser found" >&2
  exit 2
fi
if ! "$PYTHON" -c 'import playwright.sync_api' >/dev/null 2>&1; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: python Playwright unavailable for $PYTHON" >&2
  exit 2
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
EVIDENCE_DIR="${HOST_EVIDENCE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/llm-hub/phase4-auto-acceptance/${STAMP}}"
EXECUTION_ROOT="$EVIDENCE_DIR/executions"
mkdir -p "$EXECUTION_ROOT"
chmod 700 "$EVIDENCE_DIR" "$EXECUTION_ROOT"
case "$(cd "$EVIDENCE_DIR" && pwd -P)/" in
  "$(cd "$ROOT" && pwd -P)/"*)
    echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: evidence directory must live outside Builder-Hub repository" >&2
    exit 2
    ;;
esac

PROMPT_FILE="$EVIDENCE_DIR/expected-prompt.txt"
cat >"$PROMPT_FILE" <<'EOF'
Return exactly PHASE4_EXECUTION_ACK in your final response. Do not use tools. Do not create, edit, delete, stage, commit, or otherwise change any file.
EOF
chmod 600 "$PROMPT_FILE"
PROMPT_SHA="$(sha256sum "$PROMPT_FILE" | awk '{print $1}')"

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

TEST_REPO="$TMP/repository"
STATE="$TMP/workstation-state.json"
BIN="$TMP/llm-hub-local"
mkdir -p "$TEST_REPO"

git -C "$TEST_REPO" init -b main >/dev/null
git -C "$TEST_REPO" config user.name "Phase Four Acceptance"
git -C "$TEST_REPO" config user.email "phase4@example.invalid"
printf '# Phase 4 disposable repository\n' >"$TEST_REPO/README.md"
printf 'unchanged baseline\n' >"$TEST_REPO/source.txt"
git -C "$TEST_REPO" add README.md source.txt
git -C "$TEST_REPO" commit -m "phase4 baseline" >/dev/null

HEAD_BEFORE="$(git -C "$TEST_REPO" rev-parse HEAD)"
INDEX_BEFORE="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
STATUS_BEFORE="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"

cd "$APP_DIR"
./scripts/verify-phase4.sh
go build -o "$BIN" ./cmd/llm-hub-local

"$BIN" \
  -addr "127.0.0.1:${PORT}" \
  -state "$STATE" \
  -repo "$TEST_REPO" \
  -execution-evidence "$EXECUTION_ROOT" \
  >"$EVIDENCE_DIR/service.log" 2>&1 &
SERVICE_PID=$!

READY=0
for _ in $(seq 1 100); do
  if curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-before.json" 2>/dev/null; then
    READY=1
    break
  fi
  sleep 0.1
done
if [[ "$READY" -ne 1 ]]; then
  echo "PHASE4_HOST_ACCEPTANCE_BLOCKED: service did not become ready" >&2
  cat "$EVIDENCE_DIR/service.log" >&2 || true
  exit 2
fi

set +e
"$PYTHON" "$APP_DIR/scripts/phase4-browser-accept.py" \
  --url "$URL/" \
  --repo "$TEST_REPO" \
  --evidence-dir "$EVIDENCE_DIR" \
  --browser-executable "$BROWSER_EXECUTABLE" \
  --prompt-file "$PROMPT_FILE" \
  --model "$MODEL" \
  --variant "$VARIANT" \
  --timeout-seconds "$TIMEOUT_SECONDS"
DRIVER_RC=$?
set -e

if [[ "$DRIVER_RC" -eq 2 ]]; then
  {
    echo "PHASE4_BUILD_GATES=PASS"
    echo "PHASE4_AUTONOMOUS_HOST_ACCEPTANCE=BLOCKED"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 2
fi
if [[ "$DRIVER_RC" -ne 0 ]]; then
  {
    echo "PHASE4_BUILD_GATES=PASS"
    echo "PHASE4_AUTONOMOUS_HOST_ACCEPTANCE=FAIL"
  } | tee "$EVIDENCE_DIR/RESULT.txt"
  echo "Evidence: $EVIDENCE_DIR"
  exit 1
fi

mapfile -t RUN_DIRS < <(find "$EXECUTION_ROOT" -mindepth 1 -maxdepth 1 -type d -print)
if [[ "${#RUN_DIRS[@]}" -ne 1 ]]; then
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: expected exactly one execution evidence directory, got ${#RUN_DIRS[@]}" >&2
  exit 1
fi
RUN_DIR="${RUN_DIRS[0]}"

for name in prompt.txt opencode-version.txt opencode-run-help.txt transcript.log execution.json; do
  [[ -f "$RUN_DIR/$name" ]] || {
    echo "PHASE4_HOST_ACCEPTANCE_FAIL: missing execution evidence $name" >&2
    exit 1
  }
  [[ "$(stat -c '%a' "$RUN_DIR/$name")" == "600" ]] || {
    echo "PHASE4_HOST_ACCEPTANCE_FAIL: $name is not mode 600" >&2
    exit 1
  }
done
[[ "$(stat -c '%a' "$RUN_DIR")" == "700" ]] || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: run evidence directory is not mode 700" >&2
  exit 1
}
cmp -s "$PROMPT_FILE" "$RUN_DIR/prompt.txt" || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: prompt evidence bytes mismatch" >&2
  exit 1
}

grep -F 'PHASE4_EXECUTION_ACK' "$RUN_DIR/transcript.log" >/dev/null || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: real OpenCode transcript missing acknowledgment" >&2
  exit 1
}

"$PYTHON" - "$RUN_DIR/execution.json" "$PROMPT_SHA" "$MODEL" "$VARIANT" "$HEAD_BEFORE" "$RUN_DIR" <<'PY'
import json,sys
path,prompt_sha,model,variant,head,evidence_dir=sys.argv[1:]
record=json.load(open(path))
checks={
    "status": record.get("status") == "PASS",
    "exit_code": record.get("exit_code") == 0,
    "timed_out": record.get("timed_out") is False,
    "prompt_sha": record.get("prompt_sha256") == prompt_sha,
    "model": record.get("model") == model,
    "variant": record.get("variant","") == variant,
    "auto": record.get("auto_approve") is False,
    "head": record.get("starting_head_commit") == head,
    "evidence_dir": record.get("evidence_dir") == evidence_dir,
    "final_clean": record.get("repository",{}).get("clean") is True,
    "help_hash": len(record.get("opencode_run_help_sha256","")) == 64,
}
command=" ".join(record.get("command",[]))
checks["command_redacted"] = "PHASE4_EXECUTION_ACK" not in command and prompt_sha in command
bad=[name for name,ok in checks.items() if not ok]
if bad:
    raise SystemExit(f"execution.json failed checks: {bad}")
PY

HEAD_AFTER="$(git -C "$TEST_REPO" rev-parse HEAD)"
INDEX_AFTER="$(sha256sum "$TEST_REPO/.git/index" | awk '{print $1}')"
STATUS_AFTER="$(git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all)"
[[ "$HEAD_AFTER" == "$HEAD_BEFORE" ]] || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: real no-op execution changed HEAD" >&2
  exit 1
}
[[ "$INDEX_AFTER" == "$INDEX_BEFORE" ]] || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: real no-op execution changed .git/index" >&2
  exit 1
}
[[ "$STATUS_AFTER" == "$STATUS_BEFORE" ]] || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: real no-op execution changed working tree" >&2
  printf 'before=%q\nafter=%q\n' "$STATUS_BEFORE" "$STATUS_AFTER" >&2
  exit 1
}

grep -E 'POST /api/run status=200 code=OK .*prompt_sha256=[0-9a-f]{64} .*run_id=' "$EVIDENCE_DIR/service.log" >/dev/null || {
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: service log missing successful execution identity" >&2
  exit 1
}
if grep -Fq 'Return exactly PHASE4_EXECUTION_ACK' "$EVIDENCE_DIR/service.log"; then
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: service log leaked prompt content" >&2
  exit 1
fi

curl -fsS "$URL/api/state" >"$EVIDENCE_DIR/state-after.json"
git -C "$TEST_REPO" status --porcelain=v1 --untracked-files=all >"$EVIDENCE_DIR/final-status.txt"

if [[ -n "$(git -C "$ROOT" status --porcelain --untracked-files=all)" ]]; then
  echo "PHASE4_HOST_ACCEPTANCE_FAIL: acceptance left Builder-Hub worktree dirty" >&2
  git -C "$ROOT" status --short >&2
  exit 1
fi

{
  echo "tested_head=$(git -C "$ROOT" rev-parse HEAD)"
  echo "model=$MODEL"
  echo "variant=$VARIANT"
  echo "browser_executable=$BROWSER_EXECUTABLE"
  echo "python=$PYTHON"
  echo "prompt_sha256=$PROMPT_SHA"
  echo "repository_head_before=$HEAD_BEFORE"
  echo "repository_head_after=$HEAD_AFTER"
  echo "index_sha256_before=$INDEX_BEFORE"
  echo "index_sha256_after=$INDEX_AFTER"
  echo "execution_evidence_dir=$RUN_DIR"
} >"$EVIDENCE_DIR/acceptance.txt"

cat >"$EVIDENCE_DIR/RESULT.txt" <<'EOF'
PHASE4_BUILD_GATES=PASS
PHASE4_REAL_OPENCODE_EXECUTION=PASS
PHASE4_BROWSER_RUN_ACCEPTANCE=PASS
PHASE4_EVIDENCE_CUSTODY=PASS
PHASE4_NOOP_REPOSITORY_PRESERVATION=PASS
PHASE4_AUTONOMOUS_HOST_ACCEPTANCE=PASS
EOF
cat "$EVIDENCE_DIR/RESULT.txt"
echo "Evidence: $EVIDENCE_DIR"

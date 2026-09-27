#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd opencode
require_cmd sha256sum
require_cmd git
require_inputs_file

[[ $# -ge 2 && $# -le 3 ]] || fail "usage: $0 EXPECTED_HUB_COMMIT EXPECTED_PROMPT_PACK_SHA256 [--auto]"
expected_hub="$1"
expected_pack="$2"
auto_mode=0
if [[ ${3:-} == --auto ]]; then auto_mode=1; elif [[ $# -eq 3 ]]; then fail "unknown third argument: $3"; fi

verify_args=("$expected_hub" "$expected_pack")
if [[ -n "${SOURCE_INPUT:-}" ]]; then verify_args+=("$SOURCE_INPUT"); fi
"$COMMAND_DIR/verify-inputs.sh" "${verify_args[@]}"

source_name="$(yaml_value source_name)"
model="$(yaml_value model)"
variant="$(yaml_value variant)"
prompt_rel="$(yaml_value builder_prompt_path)"
BUILD_ROOT="${BUILD_ROOT:-$HOME/repos/build-dev}"
BUILD_DIR="${BUILD_DIR:-$BUILD_ROOT/$source_name}"
PROMPT="$PHASE_DIR/$prompt_rel"

mkdir -p "$PHASE_DIR/runtime" "$PHASE_DIR/reports"
version_file="$PHASE_DIR/runtime/opencode-version.txt"
help_file="$PHASE_DIR/runtime/opencode-run-help.txt"

opencode --version > "$version_file" 2>&1 || fail "opencode --version failed"
opencode run --help > "$help_file" 2>&1 || fail "opencode run --help failed"

require_flag() {
  local flag="$1"
  grep -F -- "$flag" "$help_file" >/dev/null || fail "installed 'opencode run --help' does not advertise required flag: $flag"
}
require_flag --model
require_flag --dir
if [[ -n "$variant" && "$variant" != NONE && "$variant" != null ]]; then require_flag --variant; fi
if (( auto_mode == 1 )); then
  require_flag --auto
  info "WARNING: --auto explicitly enabled; configured denies still matter. Preserve the runtime help snapshot for audit."
fi

help_sha="$(sha256sum "$help_file" | awk '{print $1}')"
info "opencode_version=$(tr '\n' ' ' < "$version_file" | sed 's/[[:space:]]\+$//')"
info "opencode_run_help_sha256=$help_sha"

cmd=(opencode run --model "$model" --dir "$BUILD_DIR")
if [[ -n "$variant" && "$variant" != NONE && "$variant" != null ]]; then cmd+=(--variant "$variant"); fi
if (( auto_mode == 1 )); then cmd+=(--auto); fi
cmd+=("$(cat "$PROMPT")")

transcript="$PHASE_DIR/reports/opencode-transcript.log"
started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
set +e
(
  cd "$BUILD_DIR"
  "${cmd[@]}"
) 2>&1 | tee "$transcript"
rc=${PIPESTATUS[0]}
set -e
completed="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

{
  printf 'started_at=%s\n' "$started"
  printf 'completed_at=%s\n' "$completed"
  printf 'exit_code=%s\n' "$rc"
  printf 'opencode_version=%s\n' "$(tr '\n' ' ' < "$version_file" | sed 's/[[:space:]]\+$//')"
  printf 'opencode_run_help_sha256=%s\n' "$help_sha"
  printf 'model=%s\nvariant=%s\nauto_approve=%s\n' "$model" "$variant" "$auto_mode"
} > "$PHASE_DIR/runtime/execution.env"

"$COMMAND_DIR/collect-build-evidence.sh" >/dev/null || true
info "OpenCode exit_code=$rc"
exit "$rc"

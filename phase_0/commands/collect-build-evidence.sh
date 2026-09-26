#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd git
require_inputs_file
source_name="$(yaml_value source_name)"
BUILD_ROOT="${BUILD_ROOT:-$HOME/repos/build-dev}"
BUILD_DIR="${BUILD_DIR:-$BUILD_ROOT/$source_name}"
[[ -d "$BUILD_DIR/.git" ]] || fail "not a Git worktree: $BUILD_DIR"

report="$PHASE_DIR/reports/mechanical-evidence.md"
mkdir -p "$PHASE_DIR/reports"

head_sha="$(git -C "$BUILD_DIR" rev-parse HEAD)"
status="$(git -C "$BUILD_DIR" status --porcelain)"
if [[ -z "$status" ]]; then clean_state=clean; else clean_state=dirty; fi

source_mode="$(yaml_value source_mode)"
if [[ "$source_mode" == git ]]; then
  base_sha="$(yaml_value base_source_commit_sha)"
else
  base_sha="$(git -C "$BUILD_DIR" rev-list --max-parents=0 HEAD | tail -n 1)"
fi

{
  printf '# Mechanical Build Evidence\n\n'
  printf -- '- generated_at_utc: `%s`\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf -- '- build_dir: `%s`\n' "$BUILD_DIR"
  printf -- '- base_commit: `%s`\n' "$base_sha"
  printf -- '- result_commit: `%s`\n' "$head_sha"
  printf -- '- working_tree: `%s`\n\n' "$clean_state"

  printf '## Changed files from base to result\n\n```text\n'
  git -C "$BUILD_DIR" diff --name-status "$base_sha..$head_sha" || true
  printf '```\n\n## Uncommitted status\n\n```text\n%s\n```\n\n' "${status:-<clean>}"

  printf '## Diff stat from base to result\n\n```text\n'
  git -C "$BUILD_DIR" diff --stat "$base_sha..$head_sha" || true
  printf '```\n\n## Recent commits\n\n```text\n'
  git -C "$BUILD_DIR" log --oneline --decorate -5 || true
  printf '```\n'
} > "$report"

info "wrote $report"
printf '%s\n' "$report"

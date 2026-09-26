#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd git
require_cmd sha256sum
require_inputs_file

[[ $# -ge 2 && $# -le 3 ]] || fail "usage: $0 EXPECTED_HUB_COMMIT EXPECTED_PROMPT_PACK_SHA256 [SOURCE_ARCHIVE]"
expected_hub="$1"
expected_pack="$2"
source_archive="${3:-${SOURCE_INPUT:-}}"

[[ -d "$HUB_ROOT/.git" ]] || fail "hub root is not a Git checkout: $HUB_ROOT"
observed_hub="$(git -C "$HUB_ROOT" rev-parse HEAD)"
[[ "$observed_hub" == "$expected_hub" ]] || fail "hub commit mismatch: got $observed_hub expected $expected_hub"

hub_dirty="$(git -C "$HUB_ROOT" status --porcelain)"
[[ -z "$hub_dirty" ]] || fail "builder-hub working tree is dirty before execution"

manifest="$PHASE_DIR/manifests/pack.files.sha256"
[[ -f "$manifest" ]] || fail "prompt pack is not sealed: missing $manifest"
(
  cd "$PHASE_DIR"
  sha256sum -c manifests/pack.files.sha256
)
observed_pack="$(sha256sum "$manifest" | awk '{print $1}')"
[[ "$observed_pack" == "$expected_pack" ]] || fail "prompt pack digest mismatch: got $observed_pack expected $expected_pack"

prompt_rel="$(yaml_value builder_prompt_path)"
expected_prompt="$(yaml_value builder_prompt_sha256)"
[[ -f "$PHASE_DIR/$prompt_rel" ]] || fail "builder prompt missing: $PHASE_DIR/$prompt_rel"
observed_prompt="$(sha256sum "$PHASE_DIR/$prompt_rel" | awk '{print $1}')"
[[ "$observed_prompt" == "$expected_prompt" ]] || fail "builder prompt SHA-256 mismatch"

source_name="$(yaml_value source_name)"
source_mode="$(yaml_value source_mode)"
BUILD_ROOT="${BUILD_ROOT:-$HOME/repos/build-dev}"
BUILD_DIR="${BUILD_DIR:-$BUILD_ROOT/$source_name}"
[[ -d "$BUILD_DIR/.git" ]] || fail "prepared source is not a Git worktree: $BUILD_DIR"

case "$source_mode" in
  git)
    expected_base="$(yaml_value base_source_commit_sha)"
    observed_base="$(git -C "$BUILD_DIR" rev-parse HEAD)"
    [[ "$observed_base" == "$expected_base" ]] || fail "source commit mismatch: got $observed_base expected $expected_base"
    ;;
  archive)
    expected_source_sha="$(yaml_value source_archive_sha256)"
    [[ -n "$source_archive" ]] || fail "archive mode verification requires SOURCE_INPUT or third argument"
    [[ -f "$source_archive" ]] || fail "source archive not found: $source_archive"
    observed_source_sha="$(sha256sum "$source_archive" | awk '{print $1}')"
    [[ "$observed_source_sha" == "$expected_source_sha" ]] || fail "source archive SHA-256 mismatch"
    ;;
  *) fail "invalid source_mode: $source_mode" ;;
esac

source_dirty="$(git -C "$BUILD_DIR" status --porcelain)"
[[ -z "$source_dirty" ]] || fail "source working tree is not clean before build"

printf 'IDENTITY_CHECK=PASS\n'
printf 'HUB_COMMIT_SHA=%s\n' "$observed_hub"
printf 'PROMPT_PACK_SHA256=%s\n' "$observed_pack"
printf 'BUILDER_PROMPT_SHA256=%s\n' "$observed_prompt"
printf 'SOURCE_MODE=%s\n' "$source_mode"
printf 'LOCAL_BASELINE_COMMIT_SHA=%s\n' "$(git -C "$BUILD_DIR" rev-parse HEAD)"

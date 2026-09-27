#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd git
require_cmd sha256sum
require_inputs_file

replace=0
source_input="${SOURCE_INPUT:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source) [[ $# -ge 2 ]] || fail "--source requires a path"; source_input="$2"; shift 2 ;;
    --replace) replace=1; shift ;;
    *) fail "unknown argument: $1" ;;
  esac
done

source_name="$(yaml_value source_name)"
source_mode="$(yaml_value source_mode)"
[[ -n "$source_name" && "$source_name" != REPLACE_ME ]] || fail "set source_name in manifests/build-inputs.yaml"

STAGING_ROOT="${STAGING_ROOT:-$HOME/Downloads/${source_name}-dev-workspace}"
BUILD_ROOT="${BUILD_ROOT:-$HOME/repos/build-dev}"
BUILD_DIR="${BUILD_DIR:-$BUILD_ROOT/$source_name}"

mkdir -p "$STAGING_ROOT" "$BUILD_ROOT"
if [[ -e "$BUILD_DIR" ]]; then
  (( replace == 1 )) || fail "build directory already exists: $BUILD_DIR (use --replace only when intentional)"
  case "$BUILD_DIR" in
    "$BUILD_ROOT"/*) rm -rf -- "$BUILD_DIR" ;;
    *) fail "refusing to remove build directory outside BUILD_ROOT: $BUILD_DIR" ;;
  esac
fi

case "$source_mode" in
  git)
    source_url="$(yaml_value source_git_url)"
    base_commit="$(yaml_value base_source_commit_sha)"
    [[ -n "$source_url" && "$source_url" != REPLACE_ME ]] || fail "set source_git_url"
    [[ -n "$base_commit" && "$base_commit" != REPLACE_ME ]] || fail "set base_source_commit_sha"
    git clone "$source_url" "$BUILD_DIR"
    git -C "$BUILD_DIR" checkout --detach "$base_commit"
    actual="$(git -C "$BUILD_DIR" rev-parse HEAD)"
    [[ "$actual" == "$base_commit" ]] || fail "checked out $actual, expected $base_commit"
    ;;

  archive)
    require_cmd unzip
    expected_sha="$(yaml_value source_archive_sha256)"
    [[ -n "$source_input" ]] || fail "archive mode requires --source /path/to/source.zip or SOURCE_INPUT"
    [[ -f "$source_input" ]] || fail "source archive not found: $source_input"
    [[ -n "$expected_sha" && "$expected_sha" != NONE && "$expected_sha" != REPLACE_ME ]] || fail "set source_archive_sha256"
    actual_sha="$(sha256sum "$source_input" | awk '{print $1}')"
    [[ "$actual_sha" == "$expected_sha" ]] || fail "source archive SHA-256 mismatch: got $actual_sha expected $expected_sha"

    unpack="$STAGING_ROOT/source-unpacked"
    rm -rf -- "$unpack"
    mkdir -p "$unpack" "$BUILD_DIR"
    unzip -q "$source_input" -d "$unpack"

    mapfile -t top < <(find "$unpack" -mindepth 1 -maxdepth 1 -print)
    if [[ ${#top[@]} -eq 1 && -d "${top[0]}" ]]; then
      cp -a "${top[0]}/." "$BUILD_DIR/"
    else
      cp -a "$unpack/." "$BUILD_DIR/"
    fi

    if [[ ! -d "$BUILD_DIR/.git" ]]; then
      git config user.name >/dev/null || fail "git user.name is not configured; configure it before importing an archive"
      git config user.email >/dev/null || fail "git user.email is not configured; configure it before importing an archive"
      git -C "$BUILD_DIR" init
      git -C "$BUILD_DIR" add -A
      git -C "$BUILD_DIR" commit -m "chore: import verified source baseline"
    fi
    ;;

  *) fail "source_mode must be git or archive; got: $source_mode" ;;
esac

status="$(git -C "$BUILD_DIR" status --porcelain)"
[[ -z "$status" ]] || fail "prepared source working tree is not clean"

info "prepared BUILD_DIR=$BUILD_DIR"
info "source_mode=$source_mode"
info "local_baseline_commit_sha=$(git -C "$BUILD_DIR" rev-parse HEAD)"

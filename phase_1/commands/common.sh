#!/usr/bin/env bash
set -euo pipefail

COMMAND_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PHASE_DIR="$(cd -- "$COMMAND_DIR/.." && pwd)"
HUB_ROOT="$(cd -- "$PHASE_DIR/.." && pwd)"
INPUTS_FILE="$PHASE_DIR/manifests/build-inputs.yaml"

fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
info() { printf '[builder-hub] %s\n' "$*" >&2; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

yaml_value() {
  local key="$1"
  awk -v k="$key" '
    $0 ~ "^[[:space:]]*" k ":[[:space:]]*" {
      sub("^[[:space:]]*" k ":[[:space:]]*", "", $0)
      gsub(/^\"|\"$/, "", $0)
      print $0
      exit
    }
  ' "$INPUTS_FILE"
}

require_inputs_file() {
  [[ -f "$INPUTS_FILE" ]] || fail "missing $INPUTS_FILE"
}

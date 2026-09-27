#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd sha256sum
require_cmd markitdown

[[ $# -eq 2 ]] || fail "usage: $0 ORIGINAL_FILE OUTPUT.md"
original="$1"
derived="$2"
[[ -f "$original" ]] || fail "input file not found: $original"
mkdir -p "$(dirname -- "$derived")"

original_sha="$(sha256sum "$original" | awk '{print $1}')"
markitdown "$original" -o "$derived"
derived_sha="$(sha256sum "$derived" | awk '{print $1}')"

printf 'original_path=%s\noriginal_sha256=%s\nderived_path=%s\nderived_sha256=%s\n' \
  "$original" "$original_sha" "$derived" "$derived_sha"

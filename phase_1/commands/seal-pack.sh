#!/usr/bin/env bash
set -euo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

require_cmd sha256sum
require_cmd find
require_cmd sort
require_inputs_file

PROMPT="$PHASE_DIR/prompts/builder.md"
[[ -f "$PROMPT" ]] || fail "builder prompt missing: $PROMPT"

prompt_sha="$(sha256sum "$PROMPT" | awk '{print $1}')"
# Flat fixed-key YAML is intentional here so sealing does not require a YAML runtime.
sed -i -E "s|^builder_prompt_sha256:.*$|builder_prompt_sha256: $prompt_sha|" "$INPUTS_FILE"

manifest="$PHASE_DIR/manifests/pack.files.sha256"
digest_file="$PHASE_DIR/manifests/PROMPT_PACK_SHA256.txt"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

cd "$PHASE_DIR"
find prompts commands schemas audit manifests -type f \
  ! -path 'manifests/pack.files.sha256' \
  ! -path 'manifests/PROMPT_PACK_SHA256.txt' \
  -print | LC_ALL=C sort > "$tmp"

: > "$manifest"
while IFS= read -r file; do
  sha256sum "$file" >> "$manifest"
done < "$tmp"

pack_sha="$(sha256sum "$manifest" | awk '{print $1}')"
printf '%s  %s\n' "$pack_sha" 'manifests/pack.files.sha256' > "$digest_file"

info "builder_prompt_sha256=$prompt_sha"
info "prompt_pack_sha256=$pack_sha"
info "sealed $(wc -l < "$manifest") immutable files"

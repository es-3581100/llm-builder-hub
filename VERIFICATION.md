# Bootstrap Verification

Date: 2026-09-26

## Passed in this build environment

- `bash -n` on every Phase-0 shell command.
- `seal-pack.sh` generated a deterministic immutable-file manifest.
- `sha256sum -c phase_0/manifests/pack.files.sha256` passed for all 12 sealed files.
- Embedded JSON in `index.html` parsed successfully and exposed the expected Phase-0 run contract.
- Git-source fixture: `prepare-workspace.sh` cloned an exact commit and `verify-inputs.sh` returned `IDENTITY_CHECK=PASS`.
- Archive-source fixture: source ZIP hash was checked before extraction, a local Git baseline was created, and `verify-inputs.sh` returned `IDENTITY_CHECK=PASS`.

## Current sealed identity

- Builder prompt SHA-256: `ae39fefef6fdeca86510b8661f23b03e7ab12faa0e0aa58f83c93f78075ee81f`
- Prompt-pack SHA-256: `68823bfaf9a5b8acd9bcc59ab24c724fe0c233cd03661b7f3ab778bd9a0f7d89`

## Publication state

- GitHub publication was completed from ChatGPT after the repository was made public and the `es-3581100` write-capable GitHub connection was selected.
- The GitHub commit history is the authoritative publication record.

## Not claimed as verified here

- A real `opencode run` was not executed because OpenCode is not installed in this sandbox. The first real-repository loop remains a runtime acceptance step on the user's machine.
- `normalize-input.sh` was syntax-checked but not executed because `markitdown` is not installed in this sandbox. The user's supplied local CLI output confirms `markitdown` is available on their machine.

# Phase 0 Pack

Immutable/sealed inputs:

- `prompts/`
- `commands/`
- `schemas/`
- `audit/`
- `manifests/build-inputs.yaml`

Mutable execution evidence:

- `build_ledger.yaml`
- `reports/`
- `runtime/`

Seal with:

```bash
./phase_0/commands/seal-pack.sh
```

The resulting prompt-pack identity is the SHA-256 of `manifests/pack.files.sha256`. The manifest does not hash itself, the digest file, the mutable ledger, or runtime/report output.

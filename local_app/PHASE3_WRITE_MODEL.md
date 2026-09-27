# Phase 3 — Explicit Working-Tree Edit Semantics

Phase 3 begins from the accepted Phase-2 head `2b7459351fecd97b699d4b0a2cb413573d40a264`.

The authority distinction is fixed:

```text
SAVE CHANGES = persist workstation state
WRITE FILE   = explicit repository working-tree mutation
```

## Guarded write primitive

A write request is bound to:

- repository identity;
- repository-document identity;
- canonical repository-relative path;
- SHA-256 of the exact source bytes observed when editing began;
- replacement UTF-8 text.

Before replacement, the writer rejects stale repository/document identity, deleted or renamed sources, stale content, path traversal, symlinks, non-regular files, binary content, oversized replacement content, and source type/mode changes.

Successful writes use a same-directory temporary file, sync, atomic rename, immediate byte/hash readback, and a fresh Git repository inspection. The resulting Git working-tree diff is evidence of the mutation. The Git index is not modified.

## HTTP boundary

The guarded writer is exposed only at:

```text
POST /api/write-file
```

This endpoint is distinct from `POST /api/save`. A successful repository write does not advance workstation-state revision.

Success returns the write result plus the fresh repository snapshot. Errors are JSON with a stable `code`:

- `INVALID_REQUEST` → 400
- `REPOSITORY_CONFLICT` → 409
- `DOCUMENT_CONFLICT` → 409
- `CONTENT_CONFLICT` → 409
- `UNSUPPORTED_TARGET` → 422
- `READBACK_MISMATCH` → 500
- `INTERNAL_ERROR` → 500
- `WRITE_NOT_CONFIGURED` → 404

Server logs record status, code, path, and before/after content hashes, never replacement content.

## Browser integration

Repository-backed text documents expose a separate source-edit surface:

```text
EDIT SOURCE
  ↓
browser-only source draft
  ↓
WRITE FILE
  ↓
POST /api/write-file
```

This source draft is not part of workstation state. `SAVE CHANGES` continues to persist only workstation state.

A dirty source draft blocks REFRESH rather than being silently discarded. HTTP conflicts retain the browser source draft and its original repository/document/path/content-hash binding. Only a successful `WRITE FILE` adopts the returned fresh repository snapshot and exits source-edit mode.

`CANCEL SOURCE EDIT` discards only the browser source draft and performs no filesystem mutation.

During an in-flight `WRITE FILE`, the source textarea is disabled so edits cannot race the request and then be lost on success. An authoritative refresh is allowed only when there is no dirty source draft and clears any non-dirty source edit session.\n\nA dirty repository source draft is exclusive: attempting `EDIT SOURCE` on another repository document is blocked until the existing source draft is written or explicitly cancelled. This prevents cross-document source-draft loss.

## Verification discipline

Verification is non-mutating. Formatting drift is a failure condition, not something a verifier silently repairs.

Phase-3 mechanical completion is checked by:

```text
scripts/verify-phase3.sh
```

Real browser behavior and destination-host invariants are checked by:

```text
scripts/host-accept-phase3.sh
```

Phase 3 is complete only when both pass.

## Deferred authority

Phase 3 does not add:

```text
new-file creation
delete / rename
Git add / reset / restore
commit creation
branch switching
merge / rebase
fetch / pull / push
remote synchronization
OpenCode execution
actors
SEAL
RUN
PUBLISH
GitHub mutation
```

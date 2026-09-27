# Phase 3 — Explicit Working-Tree Edit Semantics

Phase 3 begins from the accepted Phase-2 head `2b7459351fecd97b699d4b0a2cb413573d40a264`.

The authority distinction is fixed:

```text
SAVE CHANGES = persist workstation state
WRITE FILE   = explicit repository working-tree mutation
```

The first checkpoint intentionally implements only the guarded filesystem write primitive. It does **not** expose an HTTP mutation endpoint or browser control yet.

## Write contract

A write request is bound to:

- repository identity;
- repository-document identity;
- canonical repository-relative path;
- SHA-256 of the exact source bytes observed when editing began;
- replacement UTF-8 text.

Before replacement, the writer rejects stale repository/document identity, deleted or renamed sources, stale content, path traversal, symlinks, non-regular files, binary content, oversized replacement content, and source type/mode changes.

Successful writes use a same-directory temporary file, sync, atomic rename, immediate byte/hash readback, and a fresh Git repository inspection. The resulting Git working-tree diff is evidence of the mutation. The Git index is not modified.

This checkpoint does not add Git add/restore/reset/commit, branch operations, fetch/pull/push, merge/rebase, OpenCode execution, actors, RUN, SEAL, or PUBLISH.


## HTTP boundary checkpoint

The next checkpoint exposes the already-guarded writer at:

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


## Browser integration checkpoint

Repository-backed text documents now expose a separate source-edit surface:

```text
EDIT SOURCE
  ↓
browser-only source draft
  ↓
WRITE FILE
  ↓
POST /api/write-file
```

This source draft is not part of workstation state. `SAVE CHANGES` continues to persist only workstation state. A dirty source draft blocks REFRESH rather than being silently discarded. HTTP conflicts retain the browser source draft and its original repository/document/path/content-hash binding. Only a successful `WRITE FILE` adopts the returned fresh repository snapshot and exits source edit mode.

`CANCEL SOURCE EDIT` discards only the browser source draft and performs no filesystem mutation.


During an in-flight `WRITE FILE`, the source textarea is disabled so edits cannot race the request and then be lost on success. An authoritative refresh is allowed only when there is no dirty source draft and clears any non-dirty source edit session.


A dirty source draft is exclusive: attempting to enter `EDIT SOURCE` on a different repository document is blocked until the existing draft is written or explicitly cancelled. This prevents cross-document navigation from silently discarding source bytes.

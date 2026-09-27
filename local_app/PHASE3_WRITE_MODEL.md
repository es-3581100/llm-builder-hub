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

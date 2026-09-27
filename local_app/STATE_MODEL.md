# Local Workstation State Model — Phases 1–3

## Workstation state

The workstation separates authoritative persisted state from the browser's saved and draft copies:

```text
AUTHORITATIVE LOCAL STATE
    persisted JSON owned by the local service

SAVED UI / PROJECT STATE
    last state returned by the local service

DRAFT UI STATE
    browser-side editable copy
```

Typing in an ordinary workstation document or project/tool control changes only the workstation draft.

```text
DRAFT != SAVED
→ UNSAVED EDITS
```

`SAVE CHANGES` POSTs the complete workstation draft with its current revision. The service validates the shape, advances the revision, and atomically replaces the authoritative state file.

A stale workstation revision returns HTTP 409 `CONFLICTED`.

`CLEAR EDITS` restores the workstation draft from the last saved state.

## Repository observation

Repository state is a separate authority surface derived from the configured local Git worktree.

The server observes repository identity, HEAD/branch state, staged/unstaged/untracked changes, diffs, history, source-file relationships, and eligible text documents. Repository observation does not advance workstation revision and does not contact a remote.

## Phase-3 source draft

Phase 3 adds a third browser-side state:

```text
REPOSITORY SOURCE DRAFT
```

It is created only after explicit `EDIT SOURCE` on an eligible repository text document.

The source draft binds:

```text
repository_id
document_id
path
expected_content_sha256
original content
draft content
```

It is not embedded in workstation JSON and therefore is not persisted by `SAVE CHANGES`.

```text
SAVE CHANGES
→ workstation state only

WRITE FILE
→ selected repository source file only
```

A dirty source draft blocks authoritative REFRESH. `CANCEL SOURCE EDIT` discards only that source draft.

On `WRITE FILE`, stale content or identity returns a typed conflict. The external file is preserved and the browser keeps the source draft for inspection/recovery.

Only a successful source write replaces the worktree file, adopts the returned fresh repository snapshot, and exits source-edit mode.

## Persistence

The default authoritative workstation-state path is outside the inspected repository:

```text
$XDG_STATE_HOME/llm-hub/workstation-state.json
or
~/.local/state/llm-hub/workstation-state.json
```

The state file is written atomically with restrictive permissions.

Repository source writes are separate atomic replacements in the configured worktree and do not modify workstation-state revision.

## SAVE != WRITE FILE != SEAL != RUN != PUBLISH

```text
SAVE CHANGES
→ mutable authoritative workstation state

WRITE FILE
→ explicit existing-file worktree mutation

SEAL
→ deferred

RUN
→ deferred

PUBLISH
→ deferred
```

No later-phase authority is implied by Phase 3.

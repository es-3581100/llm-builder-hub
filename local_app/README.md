# LLM-Hub Local Workstation — Phases 1–3

This subtree contains the local-first workstation vertical slice for LLM Builder Hub.

Phase 1 established the workstation shell, explicit draft/saved state, unusual slider geometry, semantic HTML, and static export. Phase 2 added real local Git repository observation. Phase 3 adds one narrow mutation surface: explicit replacement of an already-materialized eligible repository text file in the local working tree.

The Hub chain-of-custody boundary remains unchanged: project-specific decisions remain payload; the workstation is a local authority and evidence surface.

## Run

```bash
cd local_app
go run ./cmd/llm-hub-local -repo /absolute/or/relative/path/to/a/git/repository
```

Defaults:

```text
url:        http://127.0.0.1:8765/
repo:       .
state:      $XDG_STATE_HOME/llm-hub/workstation-state.json
            or ~/.local/state/llm-hub/workstation-state.json
```

The default workstation-state path is outside the inspected repository so merely starting the workstation does not dirty the project checkout. An explicitly supplied `-state` path is still honored.

Use a disposable state path when testing:

```bash
go run ./cmd/llm-hub-local \
  -addr 127.0.0.1:8765 \
  -state /tmp/llm-hub-workstation.json \
  -repo "/tmp/repository with spaces"
```

The server binds to loopback by default. Repository inspection and working-tree writes do not contact a Git remote.

## Verify

Phase-1 suite:

```bash
./scripts/verify.sh
```

Phase-2 composed suite:

```bash
./scripts/verify-phase2.sh
```

Phase-3 composed suite:

```bash
./scripts/verify-phase3.sh
```

The verifiers are non-mutating gates. Formatting drift is detected with `gofmt -l`; verification must not silently rewrite repository-controlled source.

Real-browser Phase-3 acceptance:

```bash
./scripts/host-accept-phase3.sh
```

The host harness uses a disposable Git repository and records evidence under:

```text
$XDG_STATE_HOME/llm-hub/phase3-host-acceptance/<UTC>/
or
~/.local/state/llm-hub/phase3-host-acceptance/<UTC>/
```

## Authority model

Three authority streams remain distinct.

```text
WORKSTATION STATE
browser draft
  → SAVE CHANGES
  → validated persisted workstation JSON
```

```text
REPOSITORY OBSERVATION
local Git worktree
  → read-only Git inspection
  → RepositorySnapshot
  → browser + static projection
```

```text
EXPLICIT SOURCE MUTATION
eligible repository document
  → EDIT SOURCE
  → browser-only source draft
  → WRITE FILE
  → identity/hash validation
  → atomic worktree replacement
  → fresh RepositorySnapshot
```

`SAVE CHANGES` never writes repository source. `WRITE FILE` never advances workstation-state revision and never stages or commits the resulting change.

## Phase-3 source write contract

A source write is allowed only for an existing materialized text document and is bound to:

```text
repository identity
document identity
canonical repository-relative path
expected SHA-256 of observed source bytes
replacement UTF-8 text
```

The server rejects stale repository/document identity, stale content, deleted or renamed source, path traversal, symlinks, non-regular files, binary content, and oversized content.

A successful write uses a same-directory temporary file plus atomic rename, preserves file mode, reads the result back, and refreshes repository state. The Git index, HEAD, branch, and workstation revision remain unchanged.

HTTP stale-content conflicts return `409 CONTENT_CONFLICT`; the browser preserves the source draft instead of overwriting the external edit.

## Browser behavior

Repository-backed text documents are read-only by default.

```text
GIT WORKTREE
  → EDIT SOURCE
  → SOURCE EDIT / SOURCE DRAFT
  → WRITE FILE
```

`CANCEL SOURCE EDIT` discards only the browser source draft. A dirty source draft blocks REFRESH so it cannot be silently discarded.

The static HTML5 export remains mutation-free and visibly read-only.

## Still deferred

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
OAuth
plugin architecture
multi-user collaboration
graduation automation
```

See `STATE_MODEL.md`, `UI_CONTRACT.md`, `PHASE2_REPOSITORY_MODEL.md`, and `PHASE3_WRITE_MODEL.md`.

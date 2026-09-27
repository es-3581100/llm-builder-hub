# LLM-Hub Local Workstation — Phases 1–2

This subtree contains the local-first workstation vertical slice for LLM Builder Hub.

Phase 1 established the workstation shell, explicit draft/saved state, unusual slider geometry, semantic HTML, and static export. Phase 2 adds **read-only real local Git repository semantics** without adding execution, commit, publication, or remote authority.

The Hub chain-of-custody boundary remains unchanged: project-specific decisions remain payload; the workstation is a local authority and evidence surface.

## Run Phase 2

```bash
cd local_app
go run ./cmd/llm-hub-local   -repo /absolute/or/relative/path/to/a/git/repository
```

Defaults:

```text
url:        http://127.0.0.1:8765/
repo:       .
state:      $XDG_STATE_HOME/llm-hub/workstation-state.json
            or ~/.local/state/llm-hub/workstation-state.json
```

The Phase-2 default workstation-state path is intentionally outside the inspected repository. Phase-2 testing exposed that the former in-tree default could make the repository dirty merely by starting the workstation. An explicitly supplied `-state` path is still honored.

Use a disposable state path when testing:

```bash
go run ./cmd/llm-hub-local   -addr 127.0.0.1:8765   -state /tmp/llm-hub-workstation.json   -repo "/tmp/repository with spaces"
```

The server binds to loopback by default. Git inspection never contacts a remote.

## Verify

Frozen Phase-1 suite:

```bash
./scripts/verify.sh
```

Phase-2 complete suite:

```bash
./scripts/verify-phase2.sh
```

`verify-phase2.sh` runs the full Phase-1 verifier first, removes its temporary local build output, then runs the Phase-2/race tests.

## Authority model

```text
LOCAL GIT REPOSITORY
        ↓ read-only Git inspection
repository-derived snapshot
        ↓
workstation/API projection
        ↓
browser + static HTML5
```

Separately:

```text
browser draft
        ↓ SAVE CHANGES
local service validation
        ↓ atomic write
persisted workstation state
```

These authority streams do not collapse into one another.

`SAVE CHANGES` still saves **workstation state only**. It does not write source files, stage changes, create commits, change branches, or mutate Git configuration.

## Phase-2 repository state

The workstation now exposes:

```text
canonical repository root
repository-scoped identity
HEAD commit
current symbolic branch
DETACHED state
UNBORN/new-repository state
CLEAN / DIRTY working tree
staged changes
unstaged changes
untracked files
staged diff
unstaged diff
recent local history
source-file relationships
read-only Git-backed text documents
binary / oversized / symlink presence without unsafe content loading
```

Git-backed document IDs are derived from repository identity + path, so ordinary content edits and refreshes preserve document identity. A rename is represented explicitly as a source relationship and may receive a new path-derived document ID after the rename becomes canonical.

## Phase-1 shell preserved

The Phase-1 shell remains:

```text
AUTHORITY: LOCAL
revision conflict protection
browser saved/draft distinction
SAVE CHANGES
CLEAR EDITS
safe REFRESH guard
persistent far-left quick rail
3 hierarchical left sliders
exact 0:4 / 1:3 / 2:2 / 3:1 desktop document geometry
integrated 5-slot tool spine per slider
tool customization under explicit save semantics
one ordered Markdown-fence document stream
in-place VIEW ↔ EDIT
active document identity preservation
right overlay drawer that does not change document geometry
single-file static HTML5 export
semantic data-* attributes for agent inspection
compact-mode fallback below 900px
```

## Still deferred

```text
source-file mutation
Git add/reset/restore
commit creation
branch switching
merge/rebase
fetch/pull/push
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

See `STATE_MODEL.md`, `UI_CONTRACT.md`, and `PHASE2_REPOSITORY_MODEL.md`.

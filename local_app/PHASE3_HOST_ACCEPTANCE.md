# Phase-3 Host Acceptance

Phase 3 introduces explicit repository working-tree mutation, so completion requires a destination-host proof in a real graphical browser in addition to the mechanical Go/Node suite.

Run from a clean checkout of the Phase-3 candidate:

```bash
cd local_app
./scripts/verify-phase3.sh
./scripts/host-accept-phase3.sh
```

The host harness creates a disposable Git repository and a disposable workstation-state file. It never uses the Builder Hub checkout as the mutation target.

The browser checks cover:

```text
real browser → real loopback service
EDIT SOURCE
SOURCE DRAFT
WRITE FILE success
unstaged worktree visibility
CANCEL SOURCE EDIT with no write
stale-content conflict
visible CONTENT_CONFLICT
stale browser draft preservation
```

The mechanical tail proves:

```text
successful source bytes changed
cancelled source bytes did not change
external conflict bytes survived
browser stale draft did not overwrite the external edit
.git/index SHA-256 unchanged
HEAD unchanged
branch unchanged
no staged changes added
only expected paths are unstaged
workstation revision unchanged
successful and rejected write attempts logged
static export contains no mutation surface
```

Evidence is stored under:

```text
$XDG_STATE_HOME/llm-hub/phase3-host-acceptance/<UTC>/
or
~/.local/state/llm-hub/phase3-host-acceptance/<UTC>/
```

Success terminates with:

```text
PHASE3_IMPLEMENTATION=PASS
HOST_BROWSER_SOURCE_WRITE_ACCEPTANCE=PASS
PHASE3_COMPLETE=PASS
```

A failed browser or mechanical invariant terminates with:

```text
PHASE3_IMPLEMENTATION=PASS
HOST_BROWSER_SOURCE_WRITE_ACCEPTANCE=FAIL
PHASE3_COMPLETE=BLOCKED
```

Do not classify Phase 3 as complete until the real host acceptance passes.

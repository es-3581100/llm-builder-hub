# Phase-2 Recovery Manifest

phase: PHASE 2 — REAL LOCAL REPOSITORY SEMANTICS
baseline_sha: 73f94e963d824fd22187cb0edff9f70816ac3ca8
branch: phase2/local-repository-semantics
last_completed_checkpoint_sha: 1ffa00885c1a12f2fddaddfb47de7ba8df9581b9

The containing commit is the current branch checkpoint. The manifest records the previous completed checkpoint SHA rather than attempting a self-referential commit hash.

## Recovery status

The interrupted run left the Phase-2 branch at the Phase-1 baseline but preserved substantial Phase-2 work as GitHub blob objects. Checkpoint 1 attached the known-good implementation blobs to branch history.

The orphaned Phase-2 `app.js` blob was then inspected. It contained one transport corruption:

```text
document.getElementById("refresh-guard").hiddentrue;
```

Recovered form:

```text
document.getElementById("refresh-guard").hidden=true;
```

No UI redesign was performed.

## Fresh recovery-runtime verification

Current reconstructed backend tests:

```text
go test ./internal/workstation     PASS — 16 tests
go test ./...                      PASS
go test -race ./...                PASS
go vet ./...                       PASS
```

Adversarial states freshly exercised with real temporary Git repositories:

```text
clean repository
unstaged modification
staged modification
staged + unstaged same file
untracked file
detached HEAD
unborn/new repository
file deletion
rename + previous-path relationship
binary file presence
repository path containing spaces
stable Git-backed document identity across refresh
repository-scoped identity
inspection leaves .git/index unchanged
non-repository rejection
recent local history
```

Authority-boundary tests freshly exercised:

```text
external Git change appears on /api/state refresh
repository refresh does not advance workstation revision
SAVE changes workstation state only
SAVE leaves source bytes unchanged
SAVE leaves Git status unchanged
saved project-root metadata cannot retarget inspector
/api/repository is read-only
static export contains repository provenance
static export remains read-only
```

Recovered browser script was syntax-checked and its pure helpers were executed:

```text
0/1/2/3 sliders -> 4/3/2/1 columns
compact -> 1 column
structural dirty comparison
branch / DETACHED / UNBORN / UNKNOWN labels
```

## Durable implementation

```text
read-only real local Git repository inspection
repository-scoped identity
HEAD / branch / detached / unborn states
staged / unstaged / untracked separation
staged + unstaged diffs
recent local history
source-file relationships
bounded Git-backed text document loading
binary / oversized / symlink-safe representation
repository snapshot in API responses
read-only repository endpoint
static export with repository provenance
default workstation state outside inspected repository
semantic repository UI surface
```

SAVE remains workstation-state SAVE only. No Git mutation, execution, remote contact, publication, or commit creation is introduced.

## Next smallest action

Attach the recovered browser script, adversarial tests, authority-boundary tests, and Phase-2 verifier to the branch; remote-read the checkpoint; then run the complete Phase-1 + Phase-2 verification against the durable branch state and finalize the Phase-2 evidence report.

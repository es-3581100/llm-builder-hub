# Phase-2 Build Report — Real Local Repository Semantics

## Baseline

```text
frozen Phase-1 branch: phase1/local-workstation-shell
baseline commit: 73f94e963d824fd22187cb0edff9f70816ac3ca8
Phase-2 branch: phase2/local-repository-semantics
```

Phase 1 was not merged to `main` and was not refactored during Phase 2.

Recovery checkpoints created after the interrupted stream:

```text
1ffa00885c1a12f2fddaddfb47de7ba8df9581b9
  checkpoint(phase2): preserve repository semantics implementation

f48238a232ebe4651762ea8e045f1a5df7c603fd
  checkpoint(phase2): recover tests and browser projection
```

The final Phase-2 result is the commit containing this report; its SHA is intentionally reported by the outer Git handoff rather than embedded self-referentially.

## Implemented

- real local Git repository inspection using the installed `git` binary;
- canonical repository root and repository-scoped identity;
- HEAD commit, symbolic branch, detached HEAD, and unborn repository states;
- clean/dirty state with staged, unstaged, and untracked categories distinct;
- staged and unstaged diffs;
- recent local history;
- source-file relationships including rename/deletion/binary/symlink/size metadata;
- bounded read-only Git-backed text document loading;
- repository snapshot hash;
- `/api/repository` read-only endpoint;
- repository snapshot in `/api/state` and save responses;
- semantic live-HTML repository status surface;
- read-only repository documents alongside workstation documents;
- static export containing repository provenance and Git-backed document context;
- default workstation-state location moved outside the inspected repository after testing proved the former in-tree default could self-contaminate Git status.

## Authority

```text
LOCAL GIT REPOSITORY
        ↓ read-only inspection
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
persisted workstation state
```

No browser-supplied repository snapshot is accepted as authority.

## SAVE / CLEAR / REFRESH / STATIC EXPORT

### SAVE CHANGES

No semantic expansion. SAVE still writes workstation state only.

It does **not**:

```text
write source files
git add
git reset
git restore
git commit
checkout/switch
change Git config
contact a remote
```

Fresh recovery tests verify source bytes and Git status remain unchanged across SAVE.

### CLEAR EDITS

No semantic change. CLEAR restores the last saved workstation draft only and does not mutate repository state.

### REFRESH

Narrow Phase-2 extension: once the existing dirty-draft guard permits refresh, the workstation reloads both persisted workstation state and a fresh repository snapshot.

### STATIC EXPORT

Narrow Phase-2 extension: export still uses persisted workstation state, while also capturing a fresh read-only repository snapshot and bounded Git-backed text documents.

The export contains no mutation controls.

## Preserved boundaries

No OpenCode execution, actors, RUN, SEAL, PUBLISH, Git mutation, remote synchronization, OAuth, merge/rebase, or collaboration behavior was added.

The Phase-1 browser→loopback destination-host limitation remains exactly that: a destination-host acceptance item, not a reason to redesign the workstation shell.

## Disposition

```text
PHASE2_IMPLEMENTATION: PASS
HOST_BROWSER_LOOPBACK_ACCEPTANCE: PENDING
MAIN_MERGE: NOT_PERFORMED
```

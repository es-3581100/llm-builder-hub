# Phase-2 Build Report — Real Local Repository Semantics

## Baseline

```text
branch source: phase1/local-workstation-shell
baseline commit: 73f94e963d824fd22187cb0edff9f70816ac3ca8
Phase-1 baseline verification: PASS
```

Phase 2 was implemented on the dedicated branch:

```text
phase2/local-repository-semantics
```

The result commit is the Git commit containing this report; its SHA is recorded by the outer Hub/Git handoff after commit creation rather than self-referentially embedded here.

## Implemented

- real local Git repository inspection using the installed `git` binary;
- canonical repository root and repository-scoped identity;
- HEAD commit, symbolic branch, detached HEAD, and unborn repository states;
- clean/dirty state with staged, unstaged, and untracked categories kept distinct;
- staged and unstaged diffs;
- recent local history;
- source-file relationships including rename/deletion/binary/symlink/size metadata;
- bounded read-only Git-backed text document loading;
- repository snapshot hash;
- `/api/repository` read-only endpoint;
- repository snapshot in `/api/state` and save responses;
- live semantic HTML repository status surface;
- read-only repository documents alongside workstation documents;
- static export containing repository provenance and Git-backed document context;
- default workstation-state location moved outside the inspected repository after Phase-2 testing proved the old in-tree default could self-contaminate Git status.

## Authority

```text
Git repository     → repository-derived state
workstation JSON   → saved workstation state
browser            → draft projection
static HTML        → read-only projection
```

No browser-supplied repository snapshot is accepted as authority.

## SAVE / CLEAR / REFRESH / EXPORT

### SAVE

No authority expansion. SAVE still writes only workstation JSON. Automated and live HTTP tests verify that source bytes and Git status remain unchanged after SAVE.

### CLEAR EDITS

No semantic change. CLEAR still restores the last saved workstation draft. Repository state is not mutated.

### REFRESH

Narrow semantic extension: a permitted refresh now reloads the fresh repository snapshot in addition to persisted workstation state. The Phase-1 dirty-draft guard is unchanged.

### STATIC EXPORT

Narrow semantic extension: export still uses persisted workstation state, but now also adds a fresh read-only Git snapshot and bounded Git-backed documents. It still contains no mutation controls.

## Explicitly deferred

No Git mutation, execution, publication, remote contact, OAuth, actor runtime, or collaboration behavior was added.

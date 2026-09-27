# Phase 4 — Local Git Commit Control Contract

## Boundary

Phase 4 begins from the sealed Phase-3 completion commit:

```text
f42531ddc654b6357c19bda1ffa507d560bdb523
```

Phase 3 already permits one explicit source mutation:

```text
EDIT SOURCE
→ WRITE FILE
→ unstaged working-tree change
```

Phase 4 adds a narrow local Git checkpoint path:

```text
unstaged tracked modification
→ STAGE FILE
→ staged modification
→ COMMIT STAGED
→ new local commit
```

It also adds:

```text
staged tracked modification
→ UNSTAGE FILE
→ working-tree modification remains
```

This phase does not add remote synchronization, push/pull/fetch, branch switching, restore/reset of worktree content, amend, merge/rebase, or OpenCode execution from the application.

## Authority separation

The UI and server must preserve these distinct operations:

```text
SAVE CHANGES
    workstation JSON only

WRITE FILE
    existing eligible repository source file only

STAGE FILE
    Git index only

UNSTAGE FILE
    Git index only

COMMIT STAGED
    local commit object + current branch ref only
```

None implies another. In particular:

```text
WRITE FILE != STAGE FILE
STAGE FILE != COMMIT STAGED
COMMIT STAGED != PUSH
```

## Initial supported repository state

Phase 4 local Git mutation is supported only when all of these are true:

- repository has an existing HEAD commit;
- HEAD is attached to a local branch;
- no merge, rebase, cherry-pick, revert, bisect, or sequencer operation is in progress;
- target path is already tracked in HEAD;
- target path exists at the same path;
- target is a regular non-symlink file;
- target change is a modification, not add/delete/rename/typechange;
- no custom Git `filter` attribute applies to the target;
- no `working-tree-encoding` attribute applies to the target.

Unsupported states are rejected with typed errors. They are not silently normalized.

## Hidden-execution constraint

Do not implement Phase 4 staging/commit through loose porcelain commands such as:

```text
git add
git commit
git reset
git restore
```

The implementation must avoid repository hooks, editors, signing prompts, and custom clean filters as hidden execution paths.

Use explicit Git plumbing with argument arrays, not shell concatenation.

### Stage

For an eligible tracked modified path:

1. inspect and bind current repository identity, HEAD, branch, index identity, path, and current worktree content SHA-256;
2. reject unsupported Git operation state;
3. reject a custom `filter` attribute and `working-tree-encoding`;
4. re-check the exact worktree bytes and index identity immediately before mutation;
5. create the candidate blob with `git hash-object -w --path=<path> <file>` after the custom-filter/encoding rejection;
6. update exactly that index entry using `git update-index --cacheinfo`;
7. preserve the existing index mode; Phase 4 does not stage executable-bit/type changes;
8. perform fresh repository inspection and return it.

`hash-object --path` is intentional so built-in Git text/EOL conversion semantics remain consistent with Git check-in behavior. Custom filters are rejected before this call.

### Unstage

For an eligible staged modification:

1. bind repository identity, current HEAD, branch, index identity, and path;
2. resolve the exact HEAD tree entry for that path;
3. restore only that index entry to the HEAD blob/mode with `git update-index --cacheinfo`;
4. do not change worktree bytes;
5. fresh-inspect and return repository state.

Deletion/rename/typechange cases remain out of scope.

### Commit

For an eligible staged snapshot:

1. bind repository identity, expected HEAD commit, expected attached branch ref, and expected index identity;
2. reject empty staged state;
3. reject any staged entry outside the Phase-4 supported modification-only shape;
4. validate commit message: UTF-8 text, non-empty after rejecting NUL, bounded size, no invented suffix;
5. re-check repository operation state, HEAD, branch, and index immediately before commit creation;
6. create the tree with `git write-tree`;
7. create a one-parent commit with `git commit-tree <tree> -p <expected-head>` and the exact supplied message;
8. do not pass signing flags and do not invoke an editor or hooks;
9. atomically advance only the current branch with compare-and-swap:
   `git update-ref <branch-ref> <new-commit> <expected-head>`;
10. fresh-inspect and return the new HEAD.

If compare-and-swap fails, report a typed conflict. An unreachable commit object created before a failed ref update is acceptable evidence of a race; never force the ref.

Run mutating Git plumbing with `core.fsmonitor=false` and without `GIT_OPTIONAL_LOCKS=0`.

## Request identity

Each mutating request must be typed and bind enough observed state to reject stale browser actions.

### Stage request

Minimum fields:

```text
repository_id
path
expected_head_commit
expected_branch
expected_index_sha256
expected_worktree_sha256
```

### Unstage request

Minimum fields:

```text
repository_id
path
expected_head_commit
expected_branch
expected_index_sha256
```

### Commit request

Minimum fields:

```text
repository_id
expected_head_commit
expected_branch
expected_index_sha256
commit_message
```

The server, not the browser, is authority for validation.

## Index identity

Expose a deterministic current index identity in `RepositorySnapshot`, at minimum SHA-256 of `.git/index` bytes when the index exists.

The value must change when the index changes and remain unchanged across worktree-only writes.

If a repository form makes an index path unavailable or ambiguous, Phase 4 mutation may reject it rather than guess.

## Typed HTTP boundary

Add distinct endpoints:

```text
POST /api/stage-file
POST /api/unstage-file
POST /api/commit
```

Stable error codes should distinguish at least:

```text
INVALID_REQUEST
REPOSITORY_CONFLICT
HEAD_CONFLICT
BRANCH_CONFLICT
INDEX_CONFLICT
WORKTREE_CONFLICT
UNSUPPORTED_TARGET
UNSUPPORTED_GIT_STATE
UNSAFE_GIT_ATTRIBUTES
NOTHING_TO_COMMIT
COMMIT_MESSAGE_INVALID
REF_UPDATE_CONFLICT
INTERNAL_ERROR
```

Use 409 for stale/conflict conditions, 400 for invalid requests/messages, 422 for unsupported target/state/attributes, and 500 only for genuine internal failures.

Logs must record action, status/code, path where applicable, before/after index identity, expected/new HEAD where applicable, and never file contents or secrets.

## Browser contract

Add explicit local-Git controls without collapsing authority layers.

For eligible unstaged tracked modifications:

```text
STAGE FILE
```

For eligible staged tracked modifications:

```text
UNSTAGE FILE
```

Add a local commit panel containing:

- current branch and HEAD;
- exact staged-file summary;
- commit-message textarea;
- `COMMIT STAGED` button.

Rules:

- no auto-stage after `WRITE FILE`;
- no auto-commit after `STAGE FILE`;
- a dirty repository source draft blocks stage/unstage/commit until written or cancelled;
- action requests bind the repository snapshot identity described above;
- 409 conflicts remain visible and do not silently refresh away user state;
- successful actions consume the returned fresh repository snapshot;
- after commit, staged count is zero for the committed staged snapshot while unrelated unstaged changes remain;
- no PUSH control exists.

## Static export

Static HTML export remains read-only. It must not contain:

```text
STAGE FILE
UNSTAGE FILE
COMMIT STAGED
/api/stage-file
/api/unstage-file
/api/commit
```

## Required tests

At minimum add repository/server tests proving:

- stage changes index only and leaves worktree/HEAD unchanged;
- stage rejects stale index/worktree/HEAD/branch identity;
- stage rejects custom filter and working-tree-encoding;
- stage does not stage mode/type changes;
- unstage restores only the index entry and leaves worktree bytes unchanged;
- commit creates exactly one local one-parent commit from the bound staged snapshot;
- commit advances only the bound current branch via CAS;
- commit runs no hooks/editor/signing path;
- commit rejects detached/unborn/in-progress Git operations;
- commit rejects stale index/HEAD/branch;
- commit preserves unrelated unstaged changes;
- no remote configuration or remote ref changes occur;
- logs contain identities/status but not file contents;
- static export contains no Phase-4 mutation surface.

Add browser/helper tests and automated Playwright host acceptance for:

```text
WRITE FILE
→ STAGE FILE
→ UNSTAGE FILE
→ STAGE FILE
→ COMMIT STAGED
```

The automated acceptance must independently prove:

- worktree bytes after WRITE FILE;
- index SHA changes on stage/unstage as expected;
- HEAD unchanged through stage/unstage;
- HEAD changes only on commit;
- branch remains the same;
- no remote refs/remotes mutate;
- no human y/n or manual browser steps;
- static export remains read-only.

## Deferred after Phase 4

```text
new-file creation
file deletion / rename
worktree restore/discard
mode/type staging
branch creation/switching/deletion
merge / rebase / cherry-pick / revert
amend
tags
fetch / pull / push
remote synchronization
OpenCode execution from the Hub
SEAL / RUN / PUBLISH lifecycle controls
```

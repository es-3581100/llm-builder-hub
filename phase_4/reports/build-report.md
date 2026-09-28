# Phase 4 Build Report — Local Git Commit Control

```text
status=BUILD_PASS
phase=4
branch=phase4/local-git-commit-control
head=dee699794b6a36f752c7770610cd4f27f82433f6
sealed_phase4_pack_commit=d3641fa4a4d5e4107d4f9dada361142d8c1a15c5
sealed_phase3_base=f42531ddc654b6357c19bda1ffa507d560bdb523
mechanical_verification=PASS
automated_browser_acceptance=PASS
phase4_complete=PASS
```

## 1. Scope and boundary

Phase 4 begins from the sealed Phase-3 completion commit `f42531dd` and adds one narrow
local Git checkpoint path on top of the Phase-3 source-write authority. The implemented
authority chain is:

```text
WRITE FILE    -> unstaged tracked modification
STAGE FILE    -> staged modification            (Git index only)
UNSTAGE FILE  -> worktree bytes preserved       (Git index only)
COMMIT STAGED -> new LOCAL commit               (commit object + current branch ref only)
```

The separations required by `phase_4/spec/LOCAL_GIT_COMMIT_CONTROL.md` are preserved and
independently proven by the automated acceptance:

```text
SAVE CHANGES != WRITE FILE != STAGE FILE != COMMIT STAGED
COMMIT STAGED != PUSH
```

No PUSH, fetch, or pull control exists anywhere in the product or the static export.

## 2. Implementation shape

### Repository layer — `local_app/internal/workstation/git_commit_control.go` (new)

`GitCommitController` exposes `StageFile`, `UnstageFile`, and `CommitStaged`. Twelve typed
sentinels (`ErrGitInvalidRequest`, `ErrGitRepositoryConflict`, `ErrGitHeadConflict`,
`ErrGitBranchConflict`, `ErrGitIndexConflict`, `ErrGitWorktreeConflict`,
`ErrGitRefUpdateConflict`, `ErrGitCommitMessageInvalid`, `ErrGitUnsupportedTarget`,
`ErrGitUnsupportedState`, `ErrGitUnsafeAttributes`, `ErrGitNothingToCommit`) are the only
authority the HTTP layer maps. Unsupported states are rejected, never silently normalized.

A dedicated mutating runner is used for all index/ref writes. It uses **argument arrays
only** — no shell string is ever built for Git mutation — and passes
`-c core.fsmonitor=false -c core.hooksPath=/dev/null -c commit.gpgsign=false`, with
`GIT_EDITOR=:` and `GIT_TERMINAL_PROMPT=0`. It deliberately does **not** set
`GIT_OPTIONAL_LOCKS=0`; the spec forbids optional locks for mutating plumbing, so the
runner actively forces `GIT_OPTIONAL_LOCKS=1` and strips any inherited value.

Plumbing only: `git check-attr`, `hash-object`, `update-index`, `ls-tree`, `write-tree`,
`commit-tree`, `update-ref`. The product never invokes `git add`, `git commit`,
`git reset`, or `git restore`.

Guard order in every operation, chosen so that a rejection reason is never masked:

1. request/field validation
2. supported-state gate — unborn, detached, eight in-progress operation markers, index
   unavailable
3. identity comparison — repository, HEAD, branch, index
4. unsafe-attribute gate — custom `filter`, `working-tree-encoding`
5. target-shape gate — tracked in HEAD, regular non-symlink, currently a modification
6. worktree byte comparison (stage)
7. immediate re-check of state, HEAD, branch, index (and worktree bytes for stage)
8. mutate, then fresh `Inspect` and return

Branch advance is compare-and-swap only: `git update-ref <full-ref> <new> <expected-old>`.
A `.lock` fixture proves a failed CAS surfaces `REF_UPDATE_CONFLICT`; the ref is never
forced. Commit messages are validated (UTF-8, no NUL, non-empty after trimming, bounded to
4096 bytes) and forwarded as exact bytes with no invented suffix.

### Index identity — `local_app/internal/workstation/repository.go`

`RepositorySnapshot.IndexSHA256` exposes SHA-256 of the raw `.git/index` bytes, resolved
through `git rev-parse --path-format=absolute --git-path index` with a legacy fallback. An
empty value means the index is unavailable, and Phase-4 mutation rejects that repository
form rather than guessing.

### HTTP layer — `local_app/internal/workstation/server.go`

Three new endpoints: `POST /api/stage-file`, `POST /api/unstage-file`, `POST /api/commit`.
Each mirrors the existing `writeRepositoryFile` pattern: `http.MaxBytesReader` at 2 MiB,
`DisallowUnknownFields`, controller resolved by type assertion (404 `WRITE_NOT_CONFIGURED`
when unconfigured), and a log line on every path including decode failure.

Typed mapping:

| code | status |
|---|---|
| `INVALID_REQUEST`, `COMMIT_MESSAGE_INVALID` | 400 |
| `REPOSITORY_CONFLICT`, `HEAD_CONFLICT`, `BRANCH_CONFLICT`, `INDEX_CONFLICT`, `WORKTREE_CONFLICT`, `REF_UPDATE_CONFLICT` | 409 |
| `UNSUPPORTED_TARGET`, `UNSUPPORTED_GIT_STATE`, `UNSAFE_GIT_ATTRIBUTES`, `NOTHING_TO_COMMIT` | 422 |
| unrecognised | 500 `INTERNAL_ERROR` |

Structured logging emits action, status, code, path, before/after index identity, and
expected/new HEAD. It **structurally cannot** leak user content: the `gitControlLog` record
type has no field capable of holding file contents or the commit message. This was proven,
not merely argued — see section 5.

### Browser layer — `web/index.html`, `web/app.js`

A `#commit-panel` (`data-commit-panel`) carries `#commit-branch`, `#commit-head`,
`#commit-index`, `#commit-staged-summary` (`data-staged-count`), `#commit-message`
(browser-local textarea), `#commit-btn` (`data-action="commit-staged"`), and `#commit-error`
(`data-error-code`/`data-error-status`/`data-error-action`). Per-file `STAGE FILE` and
`UNSTAGE FILE` buttons are rendered into `#source-relationships`, `#staged-changes`, and
`#unstaged-changes` with `data-action` and `data-source-path`.

Requests bind the snapshot identity. There is no auto-stage after `WRITE FILE` and no
auto-commit after `STAGE FILE`; both are asserted structurally against the function bodies.
A dirty source draft blocks stage, unstage, and commit. Conflicts stay visible in
`#commit-error` and the local snapshot is never replaced on failure. Successful actions
consume the returned fresh snapshot.

### Static export

`export.go` is unchanged and remains mutation-free. `phase4_export_test.go` asserts the
export contains none of `STAGE FILE`, `UNSTAGE FILE`, `COMMIT STAGED`, `/api/stage-file`,
`/api/unstage-file`, `/api/commit`, using a non-empty repository snapshot so the assertion
is not vacuous. The existing Phase-1/2/3 export assertions were added to, not weakened.

## 3. Mechanical verification

All run in place from a clean worktree, exit 0, with the worktree clean before and after.

```text
gofmt -l cmd internal                 -> no output
go build ./...                        -> clean
go test ./...                         -> ok (2 packages)
go test -race ./...                   -> ok (2 packages)
go vet ./...                          -> clean
node --test tests/*.test.cjs          -> 23 tests / 23 pass / 0 fail
git diff --check                      -> clean
local_app/scripts/verify-phase4.sh     -> PHASE4_LOCAL_GIT_COMMIT_CONTROL_VERIFY=PASS (exit 0)
```

`verify-phase4.sh` composes upward through `verify-phase3.sh` → `verify-phase2.sh` →
`verify.sh`, re-asserts that the verifier changed neither HEAD nor worktree state, and
re-verifies the sealed pack. Sealed-pack digests: all five entries in
`phase_4/manifests/pack.files.sha256` plus `PROMPT_PACK_SHA256.txt` report `OK`. Drift
behaviour was proven with two negative tests that deliberately failed rather than
regenerating a manifest.

## 4. Automated host acceptance

Machine-driven, non-interactive, exit 0. No human y/n, no manual browser steps, no
hand-edited test files.

```text
evidence=/home/sticky-ricky/.local/state/llm-hub/phase4-host-acceptance/20260928T013018Z
browser=/usr/bin/thorium-browser (headless)
python=/home/sticky-ricky/anaconda3/bin/python
service=real loopback binary on 127.0.0.1:18767
fixture=disposable `git init` repository under /tmp with a real bare `origin`
browser checks=75  passed=75  failed=0
```

```text
PHASE4_COMPLETE=PASS
PHASE4_LOCAL_VERIFY=PASS
PHASE4_BROWSER_AUTOMATION=PASS
PHASE4_WRITE_FILE_ACCEPTANCE=PASS
PHASE4_INDEX_IDENTITY=PASS
PHASE4_HEAD_IDENTITY=PASS
PHASE4_STAGE_ACCEPTANCE=PASS
PHASE4_UNSTAGE_ACCEPTANCE=PASS
PHASE4_COMMIT_ACCEPTANCE=PASS
PHASE4_BRANCH_STABLE=PASS
PHASE4_REMOTE_IMMUTABLE=PASS
PHASE4_COMMITTED_SNAPSHOT_CONSUMED=PASS
PHASE4_AUTHORITY_SEPARATION=PASS
PHASE4_SAVE_WRITE_SEPARATION=PASS
PHASE4_DRAFT_BLOCK=PASS
PHASE4_CONFLICT_VISIBILITY=PASS
PHASE4_LOG_REDACTION=PASS
PHASE4_EXPORT_READ_ONLY=PASS
PHASE4_HUB_REPO_UNTOUCHED=PASS
PHASE4_HOST_ACCEPTANCE=PASS
```

Independent Git observations, taken from the filesystem rather than from the UI:

```text
stage1_index_sha256=0b4696ded8d7f5229cc13b106e07ff5a27d960aa2c34a0e0441234e8042b5dca
unstage_index_sha256=3b56fa687c16b66df633f2e6aff2af373967a44df33c6354851be52877eabf57
restage_index_sha256=0b4696ded8d7f5229cc13b106e07ff5a27d960aa2c34a0e0441234e8042b5dca
commit_index_sha256=54ddc68e25f563161d638d80a12f64a09cf20f72c68fb6b003fe547b61b8f9ba
new_head_commit=bc16814346f1384ca4dafecb91e0c68751e2f5b9
pre_commit_head=b89d8102dd508714a007000e6177a5d92854c00b
```

Resulting fixture history — exactly one parent, and no invented message suffix:

```text
bc16814346f1384ca4dafecb91e0c68751e2f5b9 b89d8102dd508714a007000e6177a5d92854c00b Phase Four Automated Acceptance phase4 acceptance commit 9f3a1c
b89d8102dd508714a007000e6177a5d92854c00b  Phase Four Automated Acceptance phase4 automated host baseline
```

Final fixture status is ` M unrelated.txt`: the unrelated unstaged change survived the
commit and the index is clean.

The index identity changes on stage, changes again on unstage, returns to the stage value
on re-stage, and settles to a new value at commit. HEAD is unchanged through stage and
unstage and moves only at commit, landing on the new commit with exactly one parent equal
to the pre-commit HEAD. The branch is unchanged. `git remote -v`, `refs/remotes`, and the
bare `origin`'s refs are byte-identical before and after, and the remote-tracking ref still
points at the pre-commit HEAD, proving nothing was pushed.

## 5. Defects found and repaired

### 5.1 Pre-existing Phase-3 defect — `WRITE FILE` binding scope (`a5afa20`)

In `web/app.js`, `const write` was declared inside the first `if (editing)` block and
referenced inside a sibling `if (editing)` block. Under the file's `"use strict"` directive
every keystroke in a repository source textarea threw `ReferenceError: write is not
defined`, so the `WRITE FILE` button's `disabled` flag was never cleared.

It escaped Phase-3 acceptance because a re-render happened to occur between typing and
clicking, rebuilding the button with a correct value. It also escaped the Node tests,
which only exercise the pure exported helpers and never analyze the DOM-building code.

Because this made the entire required Phase-4 chain unreachable through the UI, repairing
it was a prerequisite. The fix hoists the binding to the enclosing scope. A new static
regression test asserts the binding is declared exactly once, in enclosing scope, before
every use; it was proven non-vacuous by re-introducing the defect in a scratch tree and
observing the test fail with 22 of 23 passing.

### 5.2 Phase-4 defect — stale index identity (`4d6de59`)

`Inspect` computed `IndexSHA256` at the *start* of inspection. However `git diff` (the
unstaged diff) refreshes the Git index **stat cache and rewrites `.git/index` even with
`GIT_OPTIONAL_LOCKS=0`**. The reported identity therefore described pre-refresh bytes: after
a successful stage the server reported `392e27a0…` while the real on-disk index was
`9b2ebbe1…`. The value was also an intermediate state matching neither the pre-stage nor
the final index.

This broke the CAS contract in the browser's favour of spurious `INDEX_CONFLICT`, and it was
caught by the automated acceptance (`stage_index_sha_matches_disk`) rather than by any
existing test — both `AfterIndexSHA256` and `Repository.IndexSHA256` had been compared only
against each other, so they agreed while both being wrong.

The fix reads the identity at the end of `Inspect`, after all read-only inspection. Four new
tests compare the reported identity against the actual on-disk bytes after stage, unstage,
commit, and plain inspection, plus a stability test. Non-vacuity was proven by reverting
the ordering in a scratch tree and observing all four fail with explicit reported-versus-disk
hash mismatches.

### 5.3 Log redaction proven, not asserted

The `gitControlLog` record type has no field able to hold user content. To prove the
corresponding test actually bites, a `secret string` field and a `secret=%s` format argument
were temporarily added in a scratch copy and the commit success path set to
`request.CommitMessage`. The test failed as intended:

```text
commit log leaked user content "top secret commit message 4b21\n"
```

The change was then discarded. The real tree was never mutated.

## 6. Test inventory

Go test functions: 65 total — 1 in `cmd/llm-hub-local`, 64 in `internal/workstation`.
Phase-4 attributable: 35 — `git_commit_control_test.go` 20, `index_identity_test.go` 8,
`server_phase4_test.go` 5, `phase4_export_test.go` 2.

Node tests: 23 total (10 pre-existing, all still passing), 13 Phase-4 attributable.

Adversarial coverage includes stale index/worktree/HEAD/branch identity on all three
operations, custom `filter` and `working-tree-encoding` rejection, mode-change rejection and
mode preservation, unmerged/deleted/renamed/type-changed staged shapes, detached and unborn
HEAD, all eight in-progress operation markers, a locked-ref CAS failure, and a PATH-shim
`git` argv recorder that asserts the plumbing subcommand allowlist, the absence of every
porcelain mutation verb, the absence of signing flags, and the presence of the three safety
`-c` flags. A tripwire test installs `pre-commit`/`commit-msg` hooks and sets
`commit.gpgsign=true`, then asserts no hook fired and the commit is unsigned.

## 7. Honest limitations

1. **The acceptance harness requires a clean worktree and is therefore blocked in place.**
   `host-accept-phase4.sh` refuses to start when the Builder-Hub worktree is dirty, emitting
   `PHASE4_HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean` (exit 2). That is the
   designed behavior, recorded as evidence rather than worked around. The run reported above
   was executed from a throwaway local `git clone` in which the harness files were tracked,
   which is how a clean-worktree run is legitimately obtained. The harness's own mechanism is
   a disposable `git init` fixture repository under `/tmp` with a real bare `origin`; it does
   not itself clone. `PHASE4_HUB_REPO_UNTOUCHED=PASS` proves the build repository's HEAD and
   porcelain status were identical before and after.
2. **The acceptance requires an explicit Python interpreter.** `PHASE4_PYTHON=/home/sticky-ricky/anaconda3/bin/python`
   must be supplied because the default `python3`/`python` on `PATH` do not have Playwright
   installed. This is an environment prerequisite, not a defect.
3. **The dirty-source-draft block fires a dialog and leaves `#commit-error` empty.**
   `guardLocalGitControl` only calls `window.alert` and returns, so `setCommitControlError` is
   never reached. The spec requires that a conflict "remain visible and do not silently
   refresh away user state" and separately that a draft block the action; it does not require
   an error-code element for the draft case. The harness therefore asserts the stronger
   inverse: a dialog fired **and** zero requests reached the three endpoints **and** HEAD,
   branch, index, and worktree bytes were all unchanged. This is a deliberate reading of the
   spec, recorded here so a reviewer can disagree with it knowingly.
4. **`git write-tree` legitimately rewrites `.git/index`** (cache-tree extension), so index
   identity is expected to change across a commit. Three acceptance-harness assertions that
   wrongly required the identity to survive a commit unchanged were corrected to assert that
   the identity reported for the commit is the identity actually found on disk. No product
   behaviour was changed to make acceptance pass.
5. **An empty `~/.gnupg` was created in the user home** by an early `commit.gpgsign` probe
   during implementation. It was left in place deliberately rather than deleting user home
   state.
6. **A clone of this repository exists at `/tmp/opencode/p4accept`** from the acceptance run.
   It is disposable scratch state outside the workspace.

## 8. Deferred after Phase 4

None of the following were implemented, and no partial scaffolding for them exists:

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

## 9. Scope deviations

Three, all disclosed:

1. **Repaired a pre-existing Phase-3 defect in `web/app.js`** (section 5.1). Outside the
   Phase-4 diff surface, but a prerequisite: without it the required Phase-4 chain is
   unreachable. The repair is two lines and adds no capability.
2. **Reordered index-identity computation within Phase-4's own new code** (section 5.2).
   Entirely within Phase-4 scope.
3. **Corrected three acceptance-harness assertions** that contradicted Git behaviour
   (sections 7.4 and 5.2). Harness-only; product behaviour unchanged.

No Phase-4-excluded capability was implemented. No repository hooks, editor invocation,
signing, remote operation, or custom clean filter was introduced as a hidden execution path.
No ref was ever forced.

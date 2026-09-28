# Phase 4 Test Report

```text
status=PASS
phase=4
implementation=dee699794b6a36f752c7770610cd4f27f82433f6
mechanical_verification=PASS
```

## Mechanical verification

Run in place from a clean worktree; exit 0 with the worktree clean before and after.

```text
gofmt -l cmd internal            -> no output
go build ./...                   -> clean
go test ./...                    -> ok (2 packages)
go test -race ./...              -> ok (2 packages)
go vet ./...                     -> clean
node --test tests/*.test.cjs     -> 23 tests / 23 pass / 0 fail
git diff --check                 -> clean
scripts/verify-phase4.sh         -> PHASE4_LOCAL_GIT_COMMIT_CONTROL_VERIFY=PASS
```

`verify-phase4.sh` composes upward through the Phase-3 and Phase-2 verifiers, re-asserts
that verification changed neither HEAD nor worktree state, and re-verifies the sealed
Phase-4 pack digests.

## Inventory

Go test functions: 65 total (1 in `cmd/llm-hub-local`, 64 in `internal/workstation`).
Phase-4 attributable: 35 — `git_commit_control_test.go` 20, `index_identity_test.go` 8,
`server_phase4_test.go` 5, `phase4_export_test.go` 2.

Node tests: 23 total, of which 10 are pre-existing and still passing; 13 are Phase-4.

## Repository-layer coverage

- stage changes the index only; worktree bytes and HEAD unchanged
- stage/unstage/commit each reject stale index, worktree, HEAD, and branch identity,
  including adversarial drift applied to the real repository between inspect and request
- custom `filter` and `working-tree-encoding` are rejected before any mutation
- mode and type changes are not staged; the existing index mode is preserved
- unstage restores exactly one index entry, leaves other entries untouched, and leaves
  worktree bytes byte-identical
- commit creates exactly one new one-parent commit equal to the bound staged snapshot, with
  the commit message forwarded as exact bytes and no invented suffix
- commit advances only the bound current branch, via compare-and-swap; a locked ref yields
  `REF_UPDATE_CONFLICT` and the ref is never forced
- commit runs no hook, editor, or signing path — tripwire hooks plus `commit.gpgsign=true`
  produce no marker file and an unsigned commit
- commit rejects detached HEAD, unborn HEAD, and all eight in-progress operation markers
- commit rejects empty staged state and every non-modification staged shape
- commit preserves unrelated unstaged changes; no remote configuration or remote ref changes
- a PATH-shim `git` argv recorder asserts the plumbing subcommand allowlist, the absence of
  every porcelain mutation verb, the absence of signing flags, and the presence of
  `core.fsmonitor=false`, `core.hooksPath=/dev/null`, and `commit.gpgsign=false`

## Index identity coverage

`RepositorySnapshot.IndexSHA256` is SHA-256 of the raw `.git/index` bytes. Tests prove it is
deterministic and stable across repeated inspection, empty when no index exists, unchanged
by a Phase-3 worktree write, and — after stage, unstage, commit, and plain inspection —
equal to the bytes actually found on disk. All four of the last group fail on a reverted
implementation, with explicit reported-versus-disk hash mismatches.

## HTTP boundary coverage

Happy-path 200 and JSON shape for all three endpoints; every spec error code mapped to its
correct status through real repository states (409 family, 422 family, and 400 message
validation); malformed JSON, unknown fields, and oversized bodies rejected; 404
`WRITE_NOT_CONFIGURED` when the controller is absent; and a log line containing identities
and status but neither file content nor commit message.

The log-redaction assertion was proven non-vacuous: adding a `secret` field to the log
record in a scratch copy made the test fail with the leaked message, and the change was
discarded.

## Static export coverage

`phase4_export_test.go` asserts the static export contains none of `STAGE FILE`,
`UNSTAGE FILE`, `COMMIT STAGED`, `/api/stage-file`, `/api/unstage-file`, `/api/commit`,
using a non-empty repository snapshot so the sweep cannot pass vacuously. The existing
Phase-1/2/3 export assertions were preserved.

## Browser-layer coverage

Pure exported helpers are unit-tested for request construction and the eligibility matrix.
Structural assertions against `app.js` source prove `WRITE FILE` never implies
`STAGE FILE`, `STAGE FILE` never implies `COMMIT STAGED`, and that no push, fetch, or pull
affordance exists. Additional tests cover the dirty-draft gate, the commit-message textarea
staying browser-local, and the `bind()` wiring of the commit controls.

## Defects repaired during this phase

1. Pre-existing Phase-3 `web/app.js` binding-scope defect that made `WRITE FILE`
   unreachable (`a5afa20`).
2. Phase-4 stale index identity caused by `git diff` refreshing the index stat cache
   (`4d6de59`).

Both have regression tests proven non-vacuous. See `phase_4/reports/build-report.md`.

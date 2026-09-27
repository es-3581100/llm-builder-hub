# BUILD TASK — PHASE 4 LOCAL GIT COMMIT CONTROL

You are the primary implementation agent for Phase 4 of LLM Builder Hub.

This is a bounded implementation pass, not a redesign.

## Authority

The sealed Phase-3 completion base is:

```text
f42531ddc654b6357c19bda1ffa507d560bdb523
```

The current checkout contains the Phase-4 prompt pack on top of that base.

Before modifying source:

1. read `local_app/PHASE3_COMPLETE.md`;
2. read `phase_4/spec/LOCAL_GIT_COMMIT_CONTROL.md` completely;
3. read `local_app/PHASE3_WRITE_MODEL.md`, `STATE_MODEL.md`, and `UI_CONTRACT.md`;
4. inspect the current repository/writer/server/browser/test architecture;
5. run the existing Phase-3 verifier;
6. confirm the Phase-3 base is an ancestor of HEAD;
7. confirm the worktree is clean.

If the baseline does not reproduce, STOP with the exact blocker.

Do not modify the sealed files under:

```text
phase_4/prompts/
phase_4/spec/
phase_4/manifests/
```

## Primary objective

Implement exactly this local authority chain:

```text
WRITE FILE
→ unstaged tracked modification
→ STAGE FILE
→ staged modification
→ COMMIT STAGED
→ new LOCAL commit
```

and:

```text
staged modification
→ UNSTAGE FILE
→ worktree bytes preserved
```

Preserve the existing distinctions:

```text
SAVE CHANGES != WRITE FILE != STAGE FILE != COMMIT STAGED
COMMIT STAGED != PUSH
```

## Required implementation

Implement the Phase-4 contract in `phase_4/spec/LOCAL_GIT_COMMIT_CONTROL.md`.

At minimum this requires:

- deterministic repository index identity in `RepositorySnapshot`;
- guarded typed stage operation for supported tracked modifications;
- guarded typed unstage operation;
- guarded local commit operation;
- HTTP endpoints:
  - `POST /api/stage-file`
  - `POST /api/unstage-file`
  - `POST /api/commit`
- typed errors and structured non-secret logging;
- browser STAGE FILE / UNSTAGE FILE controls;
- explicit commit panel with staged summary, commit message, and COMMIT STAGED;
- fresh repository reconciliation after every successful mutation;
- source-draft guards preserved;
- static HTML export remaining mutation-free;
- updated Phase-4 docs and test report;
- automated, non-interactive Playwright host acceptance.

## Git implementation constraint

Do NOT implement the product feature using:

```text
git add
git commit
git reset
git restore
```

Do not invoke a shell command string for Git mutation.

Use explicit Git plumbing and argument arrays consistent with the Phase-4 spec:

```text
git check-attr
git hash-object
git update-index
git ls-tree
git write-tree
git commit-tree
git update-ref
```

The implementation must not introduce repository hooks, editors, signing prompts, remote operations, or custom Git clean filters as hidden execution paths.

Use compare-and-swap identity checks. Never force a ref.

## TDD order

Work in small coherent checkpoints.

1. Add failing tests for repository/index identity and stage semantics.
2. Implement the smallest stage primitive until those tests pass.
3. Add failing unstage tests; implement unstage.
4. Add failing commit tests; implement plumbing commit + CAS.
5. Add HTTP tests/routes.
6. Add browser helper/UI tests.
7. Add automated host acceptance.
8. Run the full composed verifier and acceptance.
9. Update Phase-4 docs/report only after behavior is demonstrated.

Do not replace existing working Phase-1/2/3 architecture merely for style.

## Safety / scope exclusions

Do not add:

```text
new-file creation
delete / rename
worktree restore/discard
mode/type staging
branch creation/switching/deletion
merge/rebase/cherry-pick/revert
amend
tags
fetch/pull/push product features
remote synchronization
OpenCode execution from the Hub
SEAL/RUN/PUBLISH controls
plugin architecture
Phase 5 work
```

The builder process may push the completed implementation branch only as explicitly authorized below; that is publication of this source build, not a Hub product feature.

## Verification

Run and record:

```text
existing Phase-3 verifier
all Go tests
Go race tests
go vet
all Node tests
git diff --check
new Phase-4 verifier
fully automated Playwright host acceptance
```

Acceptance must be machine-driven. Do not ask the human to click buttons, answer y/n, count labels, inspect DevTools, or manually modify test files.

Use a disposable Git repository for mutation acceptance.

Add adversarial tests for stale identities and unsupported Git state/attributes.

If any test fails, identify whether the defect is product code, test code, or harness code. Repair only within Phase-4 scope. Do not claim PASS around a failing check.

## Git / commit requirements

- Preserve all existing history.
- Do not reset/rewrite unrelated history.
- Commit coherent implementation checkpoints.
- Do not leave intentional changes uncommitted.
- Do not force-push.
- Do not push any branch except `phase4/local-git-commit-control`.
- After complete verification and only if the local branch is a fast-forward of the remote branch, push:
  `phase4/local-git-commit-control`.
- If push authentication is unavailable, STOP after a clean local commit and report that publication blocker. Do not invent another remote path.

Write/update:

```text
phase_4/reports/build-report.md
```

with verification evidence and any blockers. Do not modify the sealed prompt/spec/manifests.

## Stop conditions

STOP instead of guessing if:

- Phase-3 baseline or ancestry fails;
- the worktree starts unexpectedly dirty;
- implementation requires a capability excluded by Phase 4;
- a supported operation cannot be implemented without hidden hooks/editor/filter execution;
- a custom filter/encoding safety boundary cannot be verified;
- Git mutation would require force;
- tests or automated acceptance cannot be made honest;
- credentials/secrets appear in commit-bound material;
- remote push would not be a fast-forward to the authorized Phase-4 branch.

## Completion response

Return exactly these fields:

```text
STATUS
BASE
BRANCH
HEAD
COMMITS
FILES_CHANGED
VERIFICATION
HOST_ACCEPTANCE
WORKTREE
PUSH
FAILURES_OR_BLOCKERS
SCOPE_DEVIATIONS
NEXT_SMALLEST_STAGE
```

Use `STATUS=BUILD_PASS` only if repository-controlled Phase-4 work and automated host acceptance pass. Otherwise return `FAIL` or `BLOCKER` with the first unresolved issue.

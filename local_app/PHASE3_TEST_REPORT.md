# Phase-3 Test Report

## Mechanical checkpoint

The Phase-3 lineage has demonstrated:

```text
Phase-2 composed regression suite: PASS
Go unit/integration tests:         PASS
Go race tests:                     PASS
Node UI helper tests:              PASS
gofmt state:                       PASS
git diff --check:                  PASS
non-mutating verifier rerun:       PASS
```

A verifier hygiene defect was found earlier: the Phase-1 verifier used `gofmt -w cmd internal`, rewriting tracked source before testing. The repaired verifier uses a non-mutating `gofmt -l` gate.

The acceptance-candidate line additionally added:

- a regression test preventing one dirty repository source draft from being silently replaced by editing another source;
- a static-export regression test that rejects Phase-3 mutation controls/API strings.

## Phase-3 completion verifier

`scripts/verify-phase3.sh` composes earlier verification and requires a clean Builder-Hub worktree, clean formatting, Go tests, race tests, vet, Node tests, `git diff --check`, unchanged HEAD, and unchanged worktree status.

## Automated host acceptance

`scripts/host-accept-phase3.sh` delegates browser behavior to `scripts/host-accept-phase3.py`, which uses Playwright with a real Chromium-class executable.

No human answers are part of the acceptance result.

The automated acceptance covers:

- source-edit entry and browser-only source draft;
- cross-document dirty-draft guard;
- dirty REFRESH guard;
- successful source write and fresh unstaged projection;
- cancel without write;
- externally induced stale-content conflict;
- visible `CONTENT_CONFLICT`;
- stale draft preservation;
- workstation SAVE separation;
- Git HEAD/branch/index/staging invariants;
- static export remaining mutation-free.

```text
mechanical_test_status=PASS on prior checkpoint
automated_host_acceptance_status=PENDING_DESTINATION_RUN
phase3_complete=PENDING_DESTINATION_RUN
```

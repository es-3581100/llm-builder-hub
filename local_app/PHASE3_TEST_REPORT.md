# Phase-3 Test Report

## Mechanical checkpoint

The published Phase-3 UI checkpoint and verifier-repair lineage passed:

```text
Phase-2 composed regression suite: PASS
Go unit/integration tests:         PASS
Go race tests:                     PASS
Node UI helper tests:              9/9 PASS
gofmt state:                       PASS
git diff --check:                  PASS
non-mutating verifier rerun:       PASS
```

A verifier hygiene defect was found during checkpoint validation: the Phase-1 verifier used `gofmt -w cmd internal`, which rewrote tracked Go files before testing and could report PASS while leaving a dirty tree.

The repair changed formatting verification to a non-mutating `gofmt -l` gate and committed the formatter output separately. The repaired verifier then passed without creating additional worktree changes.

## Phase-3 completion verifier

`scripts/verify-phase3.sh` composes earlier verification and additionally requires:

- a clean starting Builder-Hub worktree;
- formatting already clean;
- Go tests;
- Go race tests;
- Go vet;
- Node UI tests;
- `git diff --check`;
- unchanged HEAD;
- unchanged worktree status after verification.

## Host acceptance

`scripts/host-accept-phase3.sh` is BUILD_REQUIRED because browser interaction cannot be fully established by the current unit-test surface.

Required real-browser proof includes:

- explicit source-edit entry;
- successful source write;
- cancel without write;
- visible stale-content conflict;
- stale draft retention;
- no Git index/HEAD/branch mutation;
- no workstation revision mutation;
- static export remaining mutation-free.

```text
mechanical_test_status=PASS
host_acceptance_status=PENDING
phase3_complete=PENDING
```

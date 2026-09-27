# Phase-2 Recovery Manifest

phase: PHASE 2 — REAL LOCAL REPOSITORY SEMANTICS
baseline_sha: 73f94e963d824fd22187cb0edff9f70816ac3ca8
branch: phase2/local-repository-semantics
checkpoint_sha: PENDING_THIS_COMMIT

## Recovery basis

The interrupted run created durable GitHub blob objects but did not advance the Phase-2 branch. This checkpoint references only preserved blobs whose identities survived the timeout.

## Last verified commands from interrupted run

Reported before transport interruption:

```text
./scripts/verify.sh                         PASS
go test -race ./...                        PASS
Phase-2 adversarial Git matrix             PASS
live loopback HTTP vertical slice          PASS
```

These results were produced before the stream timeout. They are preserved as prior-run evidence; this recovery runtime has not yet reconstructed the full missing test set locally.

## Preserved implementation capabilities

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

## Durable preserved blob identities

```text
local_app/README.md                                      32febf04e82da335d8082d779bc7c99ab00a3a0e
local_app/PHASE2_REPOSITORY_MODEL.md                     9fe9b193855a936e7ac383b67818e11689e22960
local_app/PHASE2_BUILD_REPORT.md                         b88d273013f00613bff26c28506a9f2f9c42ccb3
local_app/PHASE2_TEST_REPORT.md                          0f4e10c7759a9694d3e149527c67f18421dbe83e
local_app/cmd/llm-hub-local/main.go                      e820196e62b7f87094a769813c2ddfe74b287285
local_app/cmd/llm-hub-local/main_test.go                 e61f01707d9f04a5de6f130da6334112b6490f3e
local_app/internal/workstation/repository.go              ef3f6d1713523aa43c45e7c2273b2f7ddf2a2059
local_app/internal/workstation/server.go                  10378c94bc1427738e5c4db0a348a1fb4d9f1157
local_app/internal/workstation/export.go                  ea6129f7e5962cbd4866a623afce4cca7874fe83
local_app/internal/workstation/web/index.html             796aae2c53643114c7a7a7d868a5452d3db4dea3
local_app/internal/workstation/web/styles.css             a8c4d4d5a55bb57ccfa5bf1b2236cc64c38f3c02
```

## Known incomplete recovery work

```text
exact tested Phase-2 app.js not yet recovered
Phase-2 repository/adversarial Go test source not yet durable on branch
Phase-2 server authority-boundary test additions not yet durable on branch
verify-phase2.sh not yet durable on branch
Phase-2 JavaScript test extension not yet durable on branch
fresh recovery-runtime verification still required
```

A previously uploaded `app.js` blob exists at `5d4cc5337583c128e6b2b1c51fcead28a5ce31e1`, but the interrupted run explicitly flagged it as not matching the locally tested file hash. It is therefore not accepted into this checkpoint.

## Next smallest action

Recover or reconstruct only the missing Phase-2 tests/scripts and exact browser projection behavior from the preserved backend contract, run the complete Phase-1 + Phase-2 verification, then create the next checkpoint before any further feature work.

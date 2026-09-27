# Phase-2 Test Report

## Frozen Phase-1 baseline

Before Phase-2 implementation, the reconstructed accepted baseline was verified with:

```bash
./scripts/verify.sh
go test -race ./...
```

Observed:

```text
PHASE1_LOCAL_APP_VERIFY=PASS
4 / 4 Phase-1 JavaScript tests PASS
Go tests PASS
go vet PASS
go build PASS
go test -race PASS
```

## Phase-2 complete verifier

```bash
./scripts/verify-phase2.sh
```

The verifier runs the complete Phase-1 suite first, then Phase-2 race coverage.

## Adversarial Git states exercised with real temporary repositories

Automated tests use the real installed Git CLI and temporary repositories, not a fake Git implementation.

```text
clean repository                         PASS
unstaged modification                    PASS
staged modification                      PASS
staged + unstaged same file              PASS
untracked file                            PASS
detached HEAD                             PASS
empty/new unborn repository              PASS
file deletion                             PASS
rename with previous-path relationship   PASS
binary/non-text file presence             PASS
repository path containing spaces         PASS
stable Git-backed document ID on refresh PASS
repository snapshot changes on Git edit  PASS
inspection leaves .git/index unchanged   PASS
non-repository rejection                  PASS
repository-scoped source IDs              PASS
multiple local-history commits            PASS
```

## Authority-boundary tests

Automated server tests verify:

```text
/api/state refresh observes external Git change        PASS
repository refresh does not advance workstation rev    PASS
SAVE changes workstation metadata only                 PASS
SAVE leaves source bytes unchanged                     PASS
SAVE leaves Git status unchanged                       PASS
saved project-root label cannot retarget inspector     PASS
static export carries repository provenance            PASS
static export includes Git-backed text documents       PASS
static export remains read-only                        PASS
```

## Live HTTP vertical slice

A real repository was created at:

```text
/tmp/llm hub phase2 real repo
```

with:

```text
MM source.txt
?? binary.bin
?? "new file.txt"
```

The running Phase-2 service was exercised over loopback process HTTP:

```text
GET  /api/state
POST /api/save
GET  /api/export
```

Observed:

```text
HTTP_PHASE2_VERTICAL_SLICE=PASS
repository_id=52048804adf9ea88f0da2bac565cd598ef081dc0c4af45d2d014282dc56876ab
head=aac6e5561636669b743e63233a17f3563852b52f
branch=main
clean=false
staged=1
unstaged=1
untracked=2
repository_documents=3
workstation_revision_after_save=2
git_status_unchanged_after_save=true
export_sha256=48b2aec85f33e10e91b2ebbe47fd50ff8a0d8b3c0e1f16ad257ee7fcdd054c0c
```

The Git status after SAVE remained exactly:

```text
## main
MM source.txt
?? binary.bin
?? "new file.txt"
```

## Destination-host acceptance retained

The Phase-1 execution environment limitation around direct Chromium/browser navigation to loopback is preserved as a destination-host acceptance item. Phase 2 did not reinterpret it as an application defect or redesign the workstation shell around it.

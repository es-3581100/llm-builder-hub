# Phase-2 Test Report

## Evidence classes

This report distinguishes:

```text
PRESERVED_PRE_INTERRUPTION
  results produced before the stream timeout and retained in the recovered blobs

FRESH_RECOVERY_RUNTIME
  results rerun during recovery against the preserved Phase-2 implementation contract

REMOTE_GIT_OBJECT
  branch/tree/blob identity read back from GitHub after checkpointing
```

## Preserved pre-interruption verification

Before transport interruption, the implementation reported:

```text
./scripts/verify.sh                         PASS
go test -race ./...                        PASS
Phase-2 adversarial Git matrix             PASS
live loopback HTTP vertical slice          PASS
```

The preserved implementation blobs from that run are the blobs now attached to the Phase-2 branch. The one browser-script transport mismatch was not accepted blindly; recovery found and repaired the single corrupted `.hidden=true` assignment before it entered branch history.

## Fresh recovery-runtime Go verification

```text
go test ./internal/workstation             PASS — 16 tests
go test ./...                              PASS
go test -race ./...                        PASS
go vet ./...                               PASS
```

Fresh adversarial repositories exercised the real installed Git CLI.

```text
clean repository                           PASS
unstaged modification                      PASS
staged modification                        PASS
staged + unstaged same file                PASS
untracked file                              PASS
detached HEAD                              PASS
unborn/new repository                      PASS
file deletion                              PASS
rename + previous-path relationship        PASS
binary/non-text presence                   PASS
repository path containing spaces          PASS
stable Git-backed document identity        PASS
repository-scoped identity                 PASS
inspection leaves .git/index unchanged     PASS
non-repository rejection                   PASS
recent local history                       PASS
```

## Fresh authority-boundary verification

```text
external Git change appears on refresh                 PASS
repository refresh does not advance workstation rev    PASS
SAVE changes workstation state only                    PASS
SAVE leaves source bytes unchanged                     PASS
SAVE leaves Git status unchanged                       PASS
saved project-root metadata cannot retarget inspector PASS
/api/repository remains read-only                      PASS
static export carries Git provenance                   PASS
static export carries Git-backed text documents        PASS
static export exposes no mutation surface              PASS
```

## Exact durable browser-script verification

Exact branch blob:

```text
local_app/internal/workstation/web/app.js
e46153cfcd103c3951ffdcc9f6fd7b4f02ff9f07
```

The exact durable blob was parsed/executed in a JavaScript runtime.

```text
syntax                                                PASS
0/1/2/3 sliders -> 4/3/2/1 columns                  PASS
compact mode -> one column                          PASS
structural dirty comparison                         PASS
tool shortcut mapping                               PASS
branch / DETACHED / UNBORN / UNKNOWN labels         PASS
```

## Fresh process-level HTTP slice

A real repository whose path contained spaces was created with:

```text
MM source.txt
?? binary.bin
?? "new file.txt"
```

The running workstation was exercised over loopback with:

```text
GET  /api/state
POST /api/save
GET  /api/export
```

Observed:

```text
HTTP_PHASE2_RECOVERY_SLICE=PASS
staged=1
unstaged=1
untracked=2
workstation_revision_after_save=2
git_status_unchanged_after_save=true
export_sha256=28fb897ae4d83357816e90adea77f39f74ba085bad3f5051af9eccbd8455f339
```

The Git status after SAVE remained exactly:

```text
MM source.txt
?? binary.bin
?? "new file.txt"
```

## Remote branch integrity

Recovery checkpoints are append-only and fast-forwarded from the frozen Phase-1 commit. No force update was used.

The final handoff must additionally verify:

```text
Phase-2 branch result commit
phase_0 tree unchanged
main unchanged
only intended Phase-2 paths changed
```

## Destination-host item retained

Direct real-browser navigation to the loopback service remains a destination-host acceptance item from Phase 1. It is not counted as a Phase-2 implementation failure and was not used to justify shell redesign.

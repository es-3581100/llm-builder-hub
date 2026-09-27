# Phase-2 Destination-Host Browser Acceptance

Frozen implementation baseline:

```text
phase2/local-repository-semantics
ab7156dba65d0d6a2a7dd67a5502e5a701df3cdd
```

This acceptance is intentionally separate from Phase-2 implementation verification.

The implementation baseline remains frozen. This file and the host harness are evidence-layer additions only.

## Required acceptance

The seam being closed is:

```text
HOST_BROWSER_LOOPBACK_ACCEPTANCE
```

A valid PASS requires an actual graphical browser on the destination host to navigate directly to the real service:

```text
http://127.0.0.1:<port>/
```

Injected HTML, mocked browser transport, static-file substitution, or a different host do not satisfy this acceptance.

## Run

From the Builder-Hub repository:

```bash
cd local_app
./scripts/host-accept-phase2.sh
```

Optional controls:

```bash
PORT=18765 ./scripts/host-accept-phase2.sh
BROWSER_CMD=firefox ./scripts/host-accept-phase2.sh
HOST_EVIDENCE_DIR="$HOME/path/to/evidence" ./scripts/host-accept-phase2.sh
```

The harness verifies accepted Phase-2 ancestry and a clean Builder-Hub worktree; creates a disposable real Git repository; records source SHA, Git status, and Git index SHA; builds and starts the real service on loopback; opens a real graphical browser; records explicit visual confirmations; checks workstation revision advancement after SAVE; mechanically proves repository source bytes/status/index remain unchanged; captures static export and SHA-256; and exercises real detached-HEAD and unborn-repository states.

## Required visual/browser confirmations

```text
real browser loads real loopback service
/api/state-derived repository state appears in UI
0/1/2/3 sliders visibly produce 4/3/2/1 document columns
VIEW/EDIT workstation behavior works
dirty draft produces UNSAVED EDITS
dirty REFRESH shows the guard
CANCEL REFRESH preserves the draft
CLEAR EDITS restores saved workstation state
SAVE CHANGES succeeds
repository-backed source documents remain read-only
DETACHED label renders after a real Git detach + refresh
UNBORN repository renders after a real service restart against a new Git repository
EXPORT HTML5 downloads
downloaded export renders as STATIC PROJECTION / READ ONLY
```

## Mechanical invariants captured

The harness fails acceptance if any of these change unexpectedly:

```text
repository source.txt SHA-256
git status --porcelain=v1 --untracked-files=all
.git/index SHA-256
```

SAVE must increase workstation revision while preserving those repository values.

## Evidence location

By default:

```text
$XDG_STATE_HOME/llm-hub/host-acceptance/<UTC timestamp>/
```

or:

```text
~/.local/state/llm-hub/host-acceptance/<UTC timestamp>/
```

Expected evidence:

```text
acceptance.txt
browser-checks.txt
service.log
state-normal-before.json
state-normal-after.json
state-detached-api.json
state-unborn-api.json
llm-hub-project.html
RESULT.txt
```

No secret or credential material should be recorded.

## Completion rule

Only:

```text
PHASE2_IMPLEMENTATION=PASS
HOST_BROWSER_LOOPBACK_ACCEPTANCE=PASS
PHASE2_COMPLETE=PASS
```

closes Phase 2.

Until then, Phase 3 remains unauthorized.

Do not rewrite the earlier recovery evidence. Preserve pre-interruption verification as historical evidence and fresh post-recovery verification as a distinct class.

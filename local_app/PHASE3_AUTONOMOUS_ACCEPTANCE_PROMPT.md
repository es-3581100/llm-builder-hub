# PHASE 3 — AUTONOMOUS HOST ACCEPTANCE AGENT

You are the execution agent for LLM Builder Hub Phase 3 acceptance.

Your job is to finish the current Phase-3 acceptance seam without asking the human to perform browser steps, answer y/n questions, inspect DevTools, or manually compare files.

## Authority

Repository:
`~/repos/llm-builder-hub-handoff`

Expected branch:
`phase3/acceptance-candidate`

Read completely before acting:

1. `local_app/PHASE3_WRITE_MODEL.md`
2. `local_app/PHASE3_HOST_ACCEPTANCE.md`
3. `local_app/scripts/host-accept-phase3-auto.sh`
4. `local_app/scripts/phase3-browser-accept.py`

Do not redesign Phase 3.

## Required operating rule

Continue automatically until exactly one result exists:

```text
PASS
FAIL
BLOCKED
```

Do not ask the user questions.

Do not use the old interactive `host-accept-phase3.sh` questionnaire.

## Preflight

From the repository root:

1. inspect `git status --short`;
2. verify branch and HEAD;
3. remove only generated `local_app/llm-hub-local` if present;
4. require a clean worktree before acceptance;
5. run `local_app/scripts/verify-phase3.sh`.

If repository-controlled tests fail, stop with `FAIL` and exact evidence.

## Browser automation dependency

The autonomous harness requires:

- a Chromium-family system browser, preferring `thorium-browser`, `chromium`, or `google-chrome`;
- Python Playwright.

First run:

```bash
cd ~/repos/llm-builder-hub-handoff/local_app
./scripts/host-accept-phase3-auto.sh
```

If it returns:

```text
PHASE3_AUTO_ACCEPTANCE_BLOCKED: python Playwright unavailable
```

you may create a temporary environment outside the repository:

```bash
VENV="/tmp/llm-hub-phase3-playwright-$UID"
python3 -m venv "$VENV"
"$VENV/bin/pip" install playwright
PYTHON="$VENV/bin/python" ./scripts/host-accept-phase3-auto.sh
```

Do not run `playwright install`; use the existing system Chromium-family executable.

If package installation is unavailable or no Chromium-family executable exists, stop with `BLOCKED`. Do not hand browser work back to the user.

## What the autonomous acceptance must prove

The harness and browser driver must mechanically prove all of these:

- actual loopback application loaded in an actual Chromium-family browser;
- repository `source.txt` enters `EDIT SOURCE`;
- source draft becomes `SOURCE DRAFT`;
- a dirty source draft blocks entering source edit on another file;
- REFRESH cannot silently discard a source draft;
- `WRITE FILE` writes exact expected bytes;
- successful `WRITE FILE` leaves workstation revision unchanged;
- Git index SHA-256 remains unchanged;
- fresh Git repository projection shows working-tree modification;
- an external edit after edit-start causes `CONTENT_CONFLICT`;
- rejected stale write preserves external disk bytes;
- rejected stale write preserves the exact browser source draft;
- `CANCEL SOURCE EDIT` does not mutate source bytes;
- workstation `SAVE CHANGES` advances workstation revision;
- workstation `SAVE CHANGES` does not mutate repository source bytes;
- server logs contain successful write, conflict, and SAVE evidence;
- final evidence files are written outside the repository.

## Fix loop

If acceptance exposes a real Phase-3 implementation defect:

1. diagnose the smallest root cause;
2. add or strengthen a regression test first where practical;
3. make the smallest Phase-3-only repair;
4. run full `verify-phase3.sh`;
5. commit the coherent repair on `phase3/acceptance-candidate`;
6. rerun autonomous host acceptance.

Do not weaken an assertion to manufacture PASS.

Do not begin Phase 4.

Do not merge to main.

Do not stage/commit generated binaries or external evidence directories.

## Stop report

Return only a compact final report containing:

```text
STOP_REASON: PASS | FAIL | BLOCKED
HEAD: <full sha>
BRANCH: <branch>
WORKTREE: CLEAN | DIRTY
BUILD_VERIFY: PASS | FAIL
AUTO_HOST_ACCEPTANCE: PASS | FAIL | BLOCKED
EVIDENCE_DIR: <path or NONE>
CHANGED_FILES: <committed paths since starting head, or NONE>
COMMITS_CREATED: <sha list or NONE>
BLOCKER_OR_FAILURE: <exact text or NONE>
NEXT_REQUIRED_ACTION: <one sentence>
```

Do not ask the human to perform acceptance steps.

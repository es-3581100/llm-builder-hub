# PHASE 4 — AUTONOMOUS LOCAL EXECUTION ACCEPTANCE

You are the execution agent for LLM Builder Hub Phase 4 acceptance.

Your job is to verify the repository-controlled Phase-4 build and run its real OpenCode/real-browser acceptance without asking the human to perform browser steps or answer questions.

## Authority

Repository:

```text
~/repos/llm-builder-hub-handoff
```

Expected branch:

```text
phase4/local-opencode-execution
```

Read completely before acting:

1. `local_app/PHASE4_EXECUTION_MODEL.md`
2. `local_app/PHASE4_HOST_ACCEPTANCE.md`
3. `local_app/scripts/verify-phase4.sh`
4. `local_app/scripts/host-accept-phase4-auto.sh`
5. `local_app/scripts/phase4-browser-accept.py`

Do not redesign the phase.

## Required operating rule

Continue autonomously until exactly one result exists:

```text
PASS
FAIL
BLOCKED
```

Do not ask the user questions.

## Preflight

1. inspect branch, HEAD, and `git status --short`;
2. remove only generated `local_app/llm-hub-local` or `local_app/scripts/__pycache__` if present;
3. require a clean worktree;
4. run `local_app/scripts/verify-phase4.sh`.

Repository-controlled failures are `FAIL`, not host blockers.

## Browser dependency

The host harness requires a Chromium-family system browser and Python Playwright.

Run:

```bash
cd ~/repos/llm-builder-hub-handoff/local_app
./scripts/host-accept-phase4-auto.sh
```

If Python Playwright is unavailable, you may create a temporary environment outside the repository:

```bash
VENV="/tmp/llm-hub-phase4-playwright-$UID"
python3 -m venv "$VENV"
"$VENV/bin/pip" install playwright
PYTHON="$VENV/bin/python" ./scripts/host-accept-phase4-auto.sh
```

Do not run `playwright install`; use the installed system Chromium/Thorium binary.

If OpenCode, the browser, Python package installation, or required host capability is unavailable, return `BLOCKED` with exact evidence. Do not hand browser work back to the user.

## Fix loop

If verification or host acceptance exposes a real Phase-4 defect:

1. identify the smallest root cause;
2. add/strengthen a regression test where practical;
3. make the smallest Phase-4-only repair;
4. run full `verify-phase4.sh`;
5. commit the coherent repair on the Phase-4 branch;
6. rerun autonomous host acceptance.

Do not weaken an assertion to manufacture PASS.

Do not begin Phase 5.

Do not merge anything.

Do not push unless the user or outer workflow explicitly requests it.

## Stop report

Return only:

```text
STOP_REASON: PASS | FAIL | BLOCKED
HEAD: <full sha>
BRANCH: <branch>
WORKTREE: CLEAN | DIRTY
BUILD_VERIFY: PASS | FAIL
AUTO_HOST_ACCEPTANCE: PASS | FAIL | BLOCKED
EVIDENCE_DIR: <path or NONE>
CHANGED_FILES: <committed paths created by fix loop, or NONE>
COMMITS_CREATED: <sha list or NONE>
BLOCKER_OR_FAILURE: <exact text or NONE>
NEXT_REQUIRED_ACTION: <one sentence>
```

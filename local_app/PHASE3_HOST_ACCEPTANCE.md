# Phase-3 Automated Host Acceptance

Phase 3 uses a fully automated destination-host acceptance path. No human questionnaire is part of the result.

`scripts/host-accept-phase3.sh` creates a disposable Git repository, starts the real loopback service, and invokes `scripts/host-accept-phase3.py` to drive an actual Chromium-class browser engine with Playwright.

The accepted Phase-3 run used:

```text
Python:  /home/sticky-ricky/anaconda3/bin/python
Browser: /usr/bin/thorium-browser
Mode:    headless
```

The automated browser proves:

```text
real browser engine → real loopback service
EDIT SOURCE
separate SOURCE DRAFT
dirty source draft cannot be replaced by another source editor
dirty source draft blocks REFRESH
WRITE FILE success
fresh unstaged Git projection
CANCEL SOURCE EDIT performs no write
external edit creates stale-content conflict
CONTENT_CONFLICT is visible
stale browser draft survives conflict
SAVE CHANGES advances workstation state only
```

The shell tail independently proves:

```text
expected source bytes
external conflict bytes preserved
.git/index SHA-256 unchanged
HEAD unchanged
branch unchanged
no staged changes
only expected files unstaged
exactly one workstation revision advance
successful and rejected write attempts logged
static export contains no mutation surface
test repository remote configuration unchanged
```

Evidence from the accepted run:

```text
/home/sticky-ricky/.local/state/llm-hub/phase3-host-acceptance/20260927T231737Z
```

Accepted implementation commit:

```text
5bc6a90efb107e86e57b59c342331dc67943ace1
```

Final result:

```text
PHASE3_LOCAL_VERIFY=PASS
PHASE3_BROWSER_AUTOMATION=PASS
PHASE3_SOURCE_WRITE_ACCEPTANCE=PASS
PHASE3_CONFLICT_ACCEPTANCE=PASS
PHASE3_SAVE_WRITE_SEPARATION=PASS
PHASE3_HOST_ACCEPTANCE=PASS
PHASE3_COMPLETE=PASS
```

Default browser discovery prefers Thorium/Chromium-class executables. `PHASE3_PYTHON` and `PHASE3_BROWSER_EXECUTABLE` remain available as explicit overrides for other hosts.

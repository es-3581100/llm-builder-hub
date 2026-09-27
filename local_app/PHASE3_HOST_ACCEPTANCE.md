# Phase-3 Automated Host Acceptance

Phase 3 no longer requires a human questionnaire.

`scripts/host-accept-phase3.sh` creates a disposable Git repository, starts the real loopback service, and invokes `scripts/host-accept-phase3.py` to drive an actual Chromium-class browser engine with Playwright.

Default browser discovery prefers:

```text
/usr/bin/thorium-browser
thorium-browser
chromium
chromium-browser
google-chrome
google-chrome-stable
```

Override when needed:

```bash
PHASE3_BROWSER_EXECUTABLE=/path/to/chromium ./scripts/host-accept-phase3.sh
```

The browser runs headless by default. To watch the automation:

```bash
PHASE3_BROWSER_HEADLESS=0 ./scripts/host-accept-phase3.sh
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

Evidence is written outside the repository under:

```text
$XDG_STATE_HOME/llm-hub/phase3-host-acceptance/<UTC>/
or
~/.local/state/llm-hub/phase3-host-acceptance/<UTC>/
```

There are no `y/n` prompts and no manual browser steps.

A missing Python Playwright installation or Chromium-class executable is reported as `PHASE3_HOST_ACCEPTANCE_BLOCKED`; functional assertion failures return `PHASE3_HOST_ACCEPTANCE_FAIL`.

Success ends with:

```text
PHASE3_COMPLETE=PASS
```

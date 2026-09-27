# Phase 3 Host Acceptance

This is the destination-host acceptance seam for Phase 3. It combines mechanical evidence with a real graphical browser against the real loopback service.

Mechanical checks prove exact working-tree bytes, unchanged Git index, workstation revision separation, server-side write logging, a real Git diff, and preservation of externally changed bytes on stale-write conflict.

Browser checks prove EDIT SOURCE, SOURCE DRAFT, prevention of dirty-draft replacement by another source editor, REFRESH blocking, successful WRITE FILE, fresh Git projection, CONTENT_CONFLICT with draft preservation, explicit CANCEL SOURCE EDIT, and SAVE CHANGES remaining workstation-only.

Evidence is written outside the repository:

```text
~/.local/state/llm-hub/phase3-host-acceptance/<UTC timestamp>/
```

The disposable repository is never staged, restored, committed, branched, fetched, pulled, or pushed after its setup commit.


## Autonomous acceptance

The preferred Phase-3 host acceptance path is now `scripts/host-accept-phase3-auto.sh`. It drives the real loopback UI with a Chromium-family browser through Python Playwright and records machine-verifiable browser, filesystem, Git-index, API, log, and revision evidence. The older interactive questionnaire is retained only as a historical/manual fallback and is not the default execution path.

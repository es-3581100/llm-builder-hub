# Phase-3 Test Report

## Mechanical verification

The accepted Phase-3 lineage passed:

```text
Phase-2 composed regression suite: PASS
Go unit/integration tests:         PASS
Go race tests:                     PASS
Node UI helper tests:              PASS
gofmt state:                       PASS
git diff --check:                  PASS
non-mutating verifier rerun:       PASS
PHASE3_LOCAL_WRITE_VERIFY:         PASS
```

A verifier hygiene defect found during Phase 3 was repaired: formatting verification now uses a non-mutating `gofmt -l` gate rather than rewriting tracked source before testing.

Regression coverage includes:

- dirty repository source draft cannot be silently replaced by editing another source;
- static export contains no Phase-3 mutation controls or write API surface.

## Automated destination-host acceptance

Final accepted implementation commit:

```text
5bc6a90efb107e86e57b59c342331dc67943ace1
```

The automated host harness used:

```text
Python:  /home/sticky-ricky/anaconda3/bin/python
Browser: /usr/bin/thorium-browser
Mode:    headless
```

It drove the real loopback UI with Playwright and independently checked disk/Git state.

The accepted run proved:

- source-edit entry and browser-only source draft;
- cross-document dirty-draft guard;
- dirty REFRESH guard;
- successful source write;
- fresh unstaged Git projection;
- cancel without write;
- externally induced stale-content conflict;
- visible `CONTENT_CONFLICT`;
- stale draft preservation;
- workstation SAVE separation;
- unchanged Git HEAD, branch, and index;
- no staged changes;
- expected-only unstaged paths;
- static export remaining mutation-free.

Evidence:

```text
/home/sticky-ricky/.local/state/llm-hub/phase3-host-acceptance/20260927T231737Z
```

Final machine result:

```text
PHASE3_LOCAL_VERIFY=PASS
PHASE3_BROWSER_AUTOMATION=PASS
PHASE3_SOURCE_WRITE_ACCEPTANCE=PASS
PHASE3_CONFLICT_ACCEPTANCE=PASS
PHASE3_SAVE_WRITE_SEPARATION=PASS
PHASE3_HOST_ACCEPTANCE=PASS
PHASE3_COMPLETE=PASS
phase3_automated_exit=0
```

## Status

```text
mechanical_test_status=PASS
automated_host_acceptance_status=PASS
phase3_complete=PASS
```

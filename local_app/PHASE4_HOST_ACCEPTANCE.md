# Phase 4 Host Acceptance

Phase 4 host acceptance proves the repository-controlled execution surface against:

- the installed OpenCode binary;
- OpenCode's own current `run --help` contract;
- the real loopback Go service;
- a real Chromium-family browser;
- a disposable clean local Git repository;
- private execution evidence outside the repository.

The acceptance prompt is deliberately a no-tools/no-file-changes acknowledgment. The goal is to prove execution transport and custody without depending on a model-generated code change.

A passing run must prove:

1. the browser shows the explicit `LOCAL OPENCODE EXECUTION` surface;
2. the request is bound to repository ID + exact HEAD;
3. `--auto` remains off unless explicitly selected;
4. the real OpenCode command exits successfully;
5. transcript contains `PHASE4_EXECUTION_ACK`;
6. workstation revision does not advance;
7. repository HEAD, index, and working tree remain unchanged for the no-op prompt;
8. prompt/version/help/transcript/execution evidence exists at private modes outside the repository;
9. command evidence contains prompt hash/length, not prompt content;
10. service logs contain execution identity but not prompt content.

Run:

```bash
./scripts/host-accept-phase4-auto.sh
```

Expected terminal result:

```text
PHASE4_BUILD_GATES=PASS
PHASE4_REAL_OPENCODE_EXECUTION=PASS
PHASE4_BROWSER_RUN_ACCEPTANCE=PASS
PHASE4_EVIDENCE_CUSTODY=PASS
PHASE4_NOOP_REPOSITORY_PRESERVATION=PASS
PHASE4_AUTONOMOUS_HOST_ACCEPTANCE=PASS
```

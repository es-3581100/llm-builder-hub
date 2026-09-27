# Phase 4 Build Report — Explicit Local OpenCode Execution

## Baseline

```text
accepted Phase-3 checkpoint:
f94cdb9c66eac6b3fd545ce58937d94e3f803a2a
```

## Repository-controlled implementation

Phase 4 adds:

- exact repository-ID and starting-HEAD execution binding;
- clean-worktree preflight;
- exact prompt SHA-256 and byte-count identity;
- explicit model, optional variant, explicit `--auto`, and bounded timeout;
- installed OpenCode `--version` and `run --help` freshness capture;
- argv-based execution without shell interpretation;
- bounded transcript capture;
- private external evidence directory with exact prompt, CLI metadata, transcript, and execution result;
- typed execution errors;
- one-active-run HTTP gate;
- source-write exclusion while execution is active;
- explicit browser `RUN OPENCODE` panel;
- clean-repository/source-draft browser guards;
- read-only static export exclusion for all execution controls;
- independent GitHub CI for Go/Node/build/script verification;
- autonomous real-browser + real-OpenCode destination-host acceptance.

## Preserved distinctions

```text
SAVE CHANGES
    = workstation-state persistence

WRITE FILE
    = explicit repository working-tree mutation

RUN OPENCODE
    = explicit bounded executor invocation
```

None of these actions silently implies either of the others.

## Deliberately not added

- Git add/reset/restore;
- commit creation;
- branch switching;
- merge/rebase;
- fetch/pull/push;
- GitHub publication;
- release publication;
- project-specific planner/decomposer logic;
- hidden autorun.

## Completion dimensions

Repository-controlled build verification is performed by `scripts/verify-phase4.sh` and the `local-app-ci` GitHub workflow.

Real OpenCode/browser proof remains a destination-host acceptance seam until `scripts/host-accept-phase4-auto.sh` passes on the machine with OpenCode installed.

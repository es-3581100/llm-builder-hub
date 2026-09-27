# Phase 4 — Explicit Local OpenCode Execution

Phase 4 starts from the accepted Phase-3 checkpoint:

```text
f94cdb9c66eac6b3fd545ce58937d94e3f803a2a
```

Phase 4 adds one new Hub-owned capability:

> bind an exact local repository state and exact prompt to an explicit OpenCode invocation, then preserve execution evidence.

It does **not** make the Hub responsible for project architecture or project-specific build decomposition.

## Authority model

```text
PROJECT PAYLOAD
    ↓ exact prompt bytes

LOCAL GIT REPOSITORY
    ↓ repository ID + exact HEAD + clean-worktree preflight

EXPLICIT RUN
    ↓ OpenCode CLI contract discovery
    ↓ exact model / variant / auto setting

OPENCODE EXECUTION
    ↓

TRANSCRIPT + EXECUTION METADATA
    ↓

FRESH LOCAL GIT SNAPSHOT
```

The Hub records custody and result evidence. The project still decides what the prompt asks the executor to build.

## Required execution request

A run is bound to:

- repository identity;
- exact starting HEAD commit;
- exact prompt bytes and SHA-256;
- model;
- optional variant;
- explicit auto-approval boolean;
- bounded timeout.

The executor refuses:

- malformed requests;
- a repository identity mismatch;
- a starting-HEAD mismatch;
- unborn HEAD;
- dirty/staged/unstaged/untracked preflight state;
- missing OpenCode;
- an installed OpenCode CLI whose own `run --help` does not advertise required flags;
- evidence directories inside the inspected source repository.

## OpenCode syntax authority

Immediately before execution the local runtime captures:

```text
opencode --version
opencode run --help
```

The exact help output is SHA-256 hashed. Required flags are checked before invocation.

The command is constructed as an argv vector, never through a shell:

```text
opencode run
  --model <model>
  --dir <canonical repository root>
  [--variant <variant>]
  [--auto]
  <exact prompt>
```

No prompt text is written to service logs.

## Evidence

Each run gets a private directory outside the repository containing at minimum:

```text
prompt.txt
opencode-version.txt
opencode-run-help.txt
transcript.log
execution.json
```

`execution.json` records the starting identity, prompt hash/length, model/variant/auto setting, timestamps, exit code, CLI-help hash, transcript truncation status, and final repository snapshot.

The transcript is bounded in memory. If output exceeds the configured limit, the stored transcript is explicitly marked truncated rather than silently consuming unbounded memory.

## Phase boundary

Phase 4 may add:

- executor core;
- typed local HTTP run boundary;
- explicit browser `RUN` control;
- run status/result projection;
- autonomous local acceptance using the installed OpenCode binary.

Phase 4 does **not** add:

- Git add/reset/restore;
- commit creation;
- branch switching;
- merge/rebase;
- fetch/pull/push;
- GitHub publication;
- release publication;
- project-specific planner/decomposer semantics;
- hidden/background autosave or autorun.

Execution must always be explicit.

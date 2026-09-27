# Phase 4 — Local Git Commit Control

Base:

```text
f42531ddc654b6357c19bda1ffa507d560bdb523
```

This pack authorizes the next bounded Hub capability:

```text
WRITE FILE
→ STAGE FILE
→ COMMIT STAGED
```

plus explicit `UNSTAGE FILE`.

It does not authorize remote Git product operations, branch switching, worktree restore/reset, or Phase 5.

Execution model:

```text
GPT/browser
  → sealed Phase-4 pack
  → OpenCode local implementation
  → committed + verified branch
  → GitHub
  → GPT/browser audit
```

Primary contract:

```text
phase_4/spec/LOCAL_GIT_COMMIT_CONTROL.md
```

Builder:

```text
phase_4/prompts/builder.md
```

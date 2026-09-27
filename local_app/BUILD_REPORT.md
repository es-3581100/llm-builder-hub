# Phase-1 Build Report — Local Workstation Shell

## Baseline

Builder Hub baseline commit used for the implementation branch:

```text
7f6cd33541f518f2e28db0d9cd638a33b4a87252
```

The existing repository contained protocol/docs/Phase-0 shell tooling but no local workstation runtime or Go application. The sealed Phase-0 prompt pack was intentionally left untouched.

## Implemented

A new isolated `local_app/` subtree provides:

- loopback Go HTTP service using only the standard library;
- authoritative persisted JSON state with atomic 0600 writes;
- revision-based conflict rejection;
- semantic HTML5 workstation UI;
- browser draft/saved split;
- SAVE / CLEAR / guarded REFRESH;
- persistent quick rail;
- three left-to-right hierarchical sliders;
- five-slot integrated tool spine per slider;
- exact 4/3/2/1 desktop document geometry;
- one ordered Markdown-fence document stream;
- in-place editing and active-document state;
- right-side overlay controls;
- read-only single-file HTML5 export;
- sample/disposable fixture data;
- Go and JavaScript regression tests.

## Preserved

Existing Hub protocol, Phase-0 pack, runtime-test history, official lifecycle prompts, licensing, and Hub boundary contract were not rewritten as part of the workstation implementation.

## Authority

For this phase, authoritative local workstation state is the server-owned JSON file, not browser memory and not GitHub.

GitHub remains only the development/publication location for this branch; no runtime GitHub synchronization was added.

## Persistence

Default:

```text
.llm-hub/workstation-state.json
```

Writes are validated, revision-checked, atomic, and mode 0600.

## Export

`GET /api/export` generates a self-contained HTML5 read-only projection from the persisted saved state. It does not accept browser draft content.

## Deferred by design

- real Git repository semantics beyond project metadata;
- OpenCode/executor integration;
- seal/run/publish actions;
- remote/GitHub synchronization;
- ChatGPT bridge/authentication;
- graduation automation.

## Next smallest phase

```text
PHASE 2 — REAL LOCAL REPOSITORY SEMANTICS
HEAD
branch
working tree status
diff
history
project identity
source relationships
Git-backed documents
```

Do not add execution or remote publishing before those local repository semantics are proven.

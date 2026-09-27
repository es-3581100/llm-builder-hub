# Phase-1 State Model

## Layers

The Phase-1 workstation deliberately separates three states:

```text
AUTHORITATIVE LOCAL STATE
    persisted JSON owned by the local service

SAVED UI / PROJECT STATE
    the last state returned by the local service after explicit save/load

DRAFT UI STATE
    the browser-side editable copy
```

Typing changes only the draft copy.

```text
DRAFT != SAVED
→ UNSAVED EDITS
```

`SAVE CHANGES` POSTs the complete draft state with its current revision. The local service validates the shape, requires exactly three sliders and exactly five tools per slider, advances the revision, and atomically replaces the authoritative state file.

A stale revision returns HTTP 409 `CONFLICTED` rather than overwriting a newer authoritative revision.

`CLEAR EDITS` restores the browser draft from the last saved state.

`REFRESH` reloads authoritative state only when the draft is clean. If dirty, the UI exposes explicit Save / Clear / Cancel choices and does not silently discard edits.

## Persistence

The default authoritative path is:

```text
.llm-hub/workstation-state.json
```

The state file is written atomically via temporary file + rename and with mode `0600`.

Phase 1 intentionally uses JSON rather than introducing a database.

## SAVE != SEAL != RUN != PUBLISH

Only SAVE exists in this phase.

```text
SAVE
→ mutable authoritative local project/workstation state

SEAL
→ deferred

RUN
→ deferred

PUBLISH
→ deferred
```

No fake buttons claim those later capabilities.

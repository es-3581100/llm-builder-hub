# Local Workstation UI Contract — Phases 1–3

## Navigation geometry

The permanent quick rail is not a slider.

Desktop document columns are an exact function of hierarchical slider depth:

```text
0 open sliders → 4 columns
1 open slider  → 3 columns
2 open sliders → 2 columns
3 open sliders → 1 column
```

The implementation exposes the current result through:

```html
data-document-columns="N"
```

The left slider region has a 75vw maximum and each deeper slider carries a small vertical offset.

Below 900px the UI explicitly enters compact mode and uses one document column.

## Tool spines

Each slider has exactly five validated tool slots. Tool selection changes the view within a navigation level; it does not change hierarchy depth or document-column count.

Default shortcuts remain collision-free by level:

```text
workspace: Alt+1 … Alt+5
project:   Ctrl+Alt+1 … Ctrl+Alt+5
branch:    Shift+Alt+1 … Shift+Alt+5
```

Customizing tool metadata edits workstation draft state and requires `SAVE CHANGES`.

## Documents

All documents belong to one ordered stream. CSS grid only changes visual packing.

Documents expose semantic attributes including:

```text
data-hub-document
data-document-id
data-document-kind
data-state
data-authority
data-editable
data-active
data-editing
```

Ordinary workstation documents use in-place VIEW ↔ EDIT and participate in workstation dirty/save semantics.

## Repository source editing

Repository-backed text documents are read-only by default and use a separate explicit mutation surface:

```text
GIT WORKTREE
  → EDIT SOURCE
  → SOURCE EDIT
  → SOURCE DRAFT
  → WRITE FILE
```

The source textarea is browser-only state. Typing there does not mark workstation state dirty.

Clicking inside an active source editor must not rebuild or destroy that editor.

`CANCEL SOURCE EDIT` performs no request and no filesystem mutation.

`WRITE FILE` sends only the source-write request bound to repository/document/path/original-content-hash identity. During the request the source textarea is disabled.

A typed HTTP conflict remains visibly distinct from workstation revision conflict. The source draft remains available after the write is refused.

A successful source write adopts the fresh repository snapshot returned by the service and exits source-edit mode.

A dirty source draft blocks REFRESH so it cannot be silently discarded. It also blocks starting EDIT SOURCE on a different repository document until the existing draft is written or explicitly cancelled.

## Right drawer

Project Management and System Settings are fixed right-side overlays. Opening them does not resize the workspace or modify document columns.

## Static projection

`GET /api/export` produces one standalone HTML5 document from saved workstation state plus the current repository projection.

The export:

```text
is visibly STATIC PROJECTION / READ ONLY
contains semantic document metadata
contains no textarea
contains no SAVE CHANGES control
contains no EDIT SOURCE control
contains no WRITE FILE control
contains no /api/write-file mutation code
```

Static export is a projection, never an authority surface.

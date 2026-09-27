# Phase-1 UI Contract

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

Below 900px the UI explicitly enters compact mode and uses one document column rather than silently crushing the desktop contract.

## Tool spines

Each slider has exactly five validated tool slots. The spine is integrated into the slider's right edge and uses a stepped/angled shoulder.

Tool selection changes the view within a navigation level; it does not add/remove hierarchy depth and therefore does not change the document-column count.

Default shortcuts are collision-free by level:

```text
workspace: Alt+1 … Alt+5
project:   Ctrl+Alt+1 … Ctrl+Alt+5
branch:    Shift+Alt+1 … Shift+Alt+5
```

Customizing icon/label/target/shortcut/enabled edits draft configuration and requires `SAVE CHANGES`.

## Documents

All documents belong to one ordered stream. CSS grid only changes visual packing; it does not create independent semantic columns.

Every document exposes semantic attributes including:

```text
data-hub-document
data-document-id
data-document-kind
data-document-order
data-state
data-authority
data-editable
data-active
data-editing
```

Ordinary editing is in-place: VIEW becomes a textarea in the same document pane.

## Right drawer

Project Management and System Settings are fixed right-side overlay panes. Opening them does not resize the workspace or modify document columns.

## Static projection

`GET /api/export` produces one standalone HTML5 document from saved authoritative state.

The export:

```text
is visibly STATIC PROJECTION / READ ONLY
contains semantic document metadata
contains the saved state as application/json
contains no textarea
contains no SAVE control
contains no mutation endpoint code
```

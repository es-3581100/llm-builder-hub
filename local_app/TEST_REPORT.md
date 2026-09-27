# Phase-1 Test Report

## Automated Go / JS verification

Command:

```bash
cd local_app
./scripts/verify.sh
```

Observed result during implementation:

```text
go test ./...                         PASS
go vet ./...                          PASS
node --test tests/*.test.cjs          4 / 4 PASS
go build ./cmd/llm-hub-local          PASS
go test -race ./...                    PASS
PHASE1_LOCAL_APP_VERIFY               PASS
```

Go tests directly cover:

- default local authority/state schema;
- exactly three sliders / five tools each;
- explicit save and stale-revision conflict rejection;
- 0600 persisted-state permissions;
- read-only semantic export;
- export sourced from persisted state;
- malformed tool spine rejection;
- security/cache headers.

JavaScript tests directly cover:

- 0:4 / 1:3 / 2:2 / 3:1 desktop geometry;
- explicit compact-mode one-column fallback;
- structural dirty comparison;
- shortcut matching.

## Browser UI acceptance

A Chromium/Playwright acceptance pass exercised the UI logic with the real HTML/CSS/JavaScript and a controlled in-browser state adapter.

Observed:

```text
BROWSER_UI_ACCEPTANCE=PASS
```

It exercised:

- 4 → 3 → 2 → 1 columns;
- actual slider retraction including child visibility;
- tool selection independent from navigation depth;
- VIEW → EDIT;
- DIRTY indication;
- CLEAR restores saved content;
- SAVE persistence semantics;
- dirty REFRESH guard / cancel preservation;
- tool customization dirty/clear behavior;
- right overlay without workspace-width or column change;
- active document identity across depth changes.

The execution environment blocks Chromium from directly navigating to loopback HTTP, so the browser pass injected the same HTML/CSS/JS and a controlled adapter rather than claiming direct browser-to-loopback verification.

## HTTP vertical slice

The actual running Go service was exercised over loopback using HTTP:

```text
GET  /api/state
POST /api/save
GET  /api/state
GET  /api/export
```

Observed:

```text
HTTP_VERTICAL_SLICE=PASS
STATIC_EXPORT_ACCEPTANCE=PASS
STATIC_BROWSER_RENDER=PASS
```

The round trip proved saved state survives server reload/read and appears in the export with the expected download disposition.

## Environment limitation

Direct rendered-browser ↔ loopback integration remains a destination-host acceptance item because this execution environment returns `ERR_BLOCKED_BY_ADMINISTRATOR` for Chromium loopback and `file://` navigation even though the service itself is reachable from normal process HTTP clients. The exported HTML was nevertheless parsed and rendered by Chromium through injected document content, proving the artifact itself is browser-renderable without a backend.

This is not represented as a repository implementation failure.

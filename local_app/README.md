# LLM-Hub Local Workstation — Phase 1

This subtree is the first local-first workstation vertical slice for LLM Builder Hub.

It does **not** replace the existing Hub chain-of-custody protocol. It adds a local working surface whose authority is explicit and whose static HTML projection can later shorten the GitHub-centered wire without reducing evidence.

## Run

```bash
cd local_app
go run ./cmd/llm-hub-local
```

Default:

```text
url:   http://127.0.0.1:8765/
state: ./.llm-hub/workstation-state.json
```

Use a disposable state path:

```bash
go run ./cmd/llm-hub-local \
  -addr 127.0.0.1:8765 \
  -state /tmp/llm-hub-workstation.json
```

The server binds to loopback by default.

## Verify

```bash
./scripts/verify.sh
```

This runs:

```text
gofmt
go test ./...
go vet ./...
node --test tests/*.test.cjs
go build ./cmd/llm-hub-local
```

## Phase-1 scope

Implemented:

```text
AUTHORITY: LOCAL
persisted local JSON state
revision conflict protection
browser saved/draft distinction
SAVE CHANGES
CLEAR EDITS
safe REFRESH guard
persistent far-left quick rail
3 hierarchical left sliders
exact 0:4 / 1:3 / 2:2 / 3:1 desktop document geometry
integrated 5-slot tool spine per slider
tool customization under explicit save semantics
one ordered Markdown-fence document stream
in-place VIEW ↔ EDIT
active document identity preservation
right overlay drawer that does not change document geometry
single-file static HTML5 export
semantic data-* attributes for agent inspection
compact-mode fallback below 900px
```

Intentionally deferred:

```text
Git command UI
OpenCode execution
sealed task execution
GitHub sync/publish
ChatGPT connector/OAuth
graduation automation
multi-user collaboration
```

## Authority

The browser is a projection/editor, not the canonical state store.

```text
browser draft
    ↓ SAVE CHANGES
local service validation
    ↓ atomic write
persisted local state
```

`GET /api/export` reads the persisted state directly; unsaved browser drafts are therefore not exported accidentally.

See `STATE_MODEL.md` and `UI_CONTRACT.md`.

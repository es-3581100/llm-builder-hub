# Builder Hub Boundary Contract

## Purpose

LLM Builder Hub is a **chain-of-custody and lifecycle handoff system for software work**.

It is not the project's architect, application framework, orchestration language, domain model, or implementation plan.

The governing idea is:

```text
human / GPT defines work
        ↓
Hub records exact task + source identity
        ↓
Hub seals / verifies the handoff
        ↓
execution surface performs the project-specific work
        ↓
Git + evidence record the result
        ↓
Hub returns the exact result for audit
```

The Hub should understand enough about a task to identify, transport, constrain at a generic boundary, and audit it. It should **not need to understand the project's internal architecture in order to move the work safely**.

## The payload-opaque rule

Project-specific implementation instructions are payload.

Examples:

```text
build a Go reducer
add these Rust types
use SQLite
implement this actor runtime
repair this HTML5 parser
run these project-specific tests
```

Those instructions belong to the project task/prompt and its repository evidence.

They do not become Builder Hub architecture merely because the Hub transported them.

A useful test is:

> If replacing Don-Dawg with an unrelated Rust, Go, Python, HTML5, or other project would make a Hub rule meaningless, that rule probably belongs to the project payload rather than the Hub.

## What the Hub owns

The Hub owns the reusable handoff contract:

```text
source identity
task / prompt identity
pack identity
checksums
workspace / repository identity
generic execution boundary
executor identity / version evidence
result commit identity
mechanical evidence
audit disposition
lifecycle transitions
runtime-test history
```

Its reusable lifecycle operations include:

```text
artifact → verified build-dev
controlled merge
verified build-dev → official project
retry / continuation when explicitly invoked
audit / PASS / REPAIR / BLOCK
```

## What the project owns

The project owns its implementation semantics:

```text
language and framework choices
domain types
application architecture
feature decomposition
phase contents
project-specific invariants
project-specific test commands
dependency choices
UI/runtime/storage design
project-specific OpenCode allowlists
the meaning of next phase
```

The Hub may carry these instructions and record their evidence. It does not promote them into global Hub policy without separate evidence that they are genuinely reusable workflow rules.

## Execution-envelope rule

The Hub may define generic execution profiles and universal safety/evidence expectations, for example:

```text
READ_ONLY
REPO_WRITE
REPO_BUILD
explicit remote publication
```

It may require universal facts such as:

```text
authorized repository boundary
starting Git identity
ending Git identity
clean/dirty state
executor identity
no silent push
no secret leakage
recorded verification
```

Project-specific command permissions remain part of the task payload unless a later runtime test proves that a rule belongs in the reusable Hub layer.

## Promotion rule

A project-specific lesson may become Hub policy only when the lesson is about the **handoff protocol itself**.

Examples that legitimately became Hub policy:

```text
flattened artifacts can lose executable-mode semantics
missing host capability is not missing implementation
publication state must not be collapsed into build state
```

Examples that should remain project-local:

```text
Don-Dawg should use a Go reducer
Don-Dawg needs EffectReceipt
a particular project should run go vet
a particular project should allow one exact shell command
```

## Minimal matrix

The Hub should be able to represent each handoff with boring, broadly understood artifacts:

```text
HTML / HTML5
JSON / YAML
plain text
Git
SHA-256
ordinary files and directories
```

Specialized systems may exist inside a project payload or execution runtime:

```text
MCP
RAG
LangGraph / LangChain
DAGs
SQLite
Chroma
actors
state machines
WASM
native toolchains
```

None of them are required to understand the Hub's chain of custody.

## Design test

Before adding Hub machinery, ask:

1. Is this required to prove what crossed the boundary?
2. Is this required to prove where it ran?
3. Is this required to prove what changed?
4. Is this required to preserve lifecycle state or auditability?
5. Would this still make sense for an unrelated project?

If the answer to 1–4 is no, or 5 is no, keep it in the project payload.

## Short form

> The Hub transports intent without owning intent.

> Reduce distance, not evidence.

> Shorter wire, same chain of custody.

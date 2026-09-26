# Rick's LLM Builder Hub — Workflow Contract v0.1

## Purpose

A Git/GitHub-centered handoff system for bounded software building:

```text
GPT/browser
    ↓
phase design + sealed prompt pack
    ↓
GitHub builder hub
    ↓
local identity gate
    ↓
OpenCode execution
    ↓
source Git commit + mechanical evidence
    ↓
GitHub
    ↓
GPT/browser audit of exact commit
    ↓
PASS / REPAIR / BLOCK
    ↓
next bounded phase
```

The system exists so a build is attributable to an exact source revision, an exact prompt, a reproducible prompt pack, an execution surface, and a resulting commit.

## Core invariant

```text
GPT designs
Git records
OpenCode executes
Git records
GPT audits
```

Important state must not exist only inside model conversation context.

## Roles

**GPT/browser** is architect, planner, prompt-pack producer, and post-build auditor. It defines scope and evidence requirements; it does not treat the builder's prose as proof.

**GitHub** is the handoff/history/evidence surface. The hub repository stores phase packs and audit continuity. Product/source repositories store the actual implementation commits.

**OpenCode** is the local execution worker. It receives a verified builder prompt and operates only in the authorized worktree.

**Git commits** are build checkpoints. Intentional source changes must be committed before a build can be called complete.

**SHA-256 manifests** provide byte identity and corruption/drift detection. They are not a substitute for repository trust or signatures.

**build_ledger** is the persistent phase state machine.

## Repository standard

```text
llm-builder-hub/
├── index.html                     # canonical human + agent entry point
├── README.md                      # short GitHub pointer
├── WORKFLOW.md                    # prose contract
└── phase_0/
    ├── README.md
    ├── build_ledger.yaml          # mutable continuity state
    ├── prompts/
    │   ├── builder.md
    │   └── verifier.md
    ├── manifests/
    │   ├── build-inputs.yaml      # immutable build identity inputs
    │   ├── pack.files.sha256      # generated seal manifest
    │   └── PROMPT_PACK_SHA256.txt # digest of the seal manifest
    ├── commands/
    │   ├── common.sh
    │   ├── normalize-input.sh
    │   ├── seal-pack.sh
    │   ├── prepare-workspace.sh
    │   ├── verify-inputs.sh
    │   ├── run-builder.sh
    │   └── collect-build-evidence.sh
    ├── schemas/
    │   └── build-ledger.schema.json
    ├── audit/
    │   └── post-build-audit.md
    ├── reports/                   # mutable output
    └── runtime/                   # mutable local execution evidence
```

Future phases repeat the same shape.

## Identity model

Every build records or resolves:

```text
EXPECTED_HUB_COMMIT
OBSERVED_HUB_COMMIT
PROMPT_PACK_SHA256
BUILDER_PROMPT_SHA256
SOURCE_MODE
BASE_SOURCE_COMMIT_SHA        # Git source
SOURCE_ARCHIVE_SHA256         # archive source
LOCAL_BASELINE_COMMIT_SHA     # prepared local worktree
RESULT_COMMIT_SHA
OPENCODE_VERSION
OPENCODE_RUN_HELP_SHA256
```

### Self-reference rule

A Git commit cannot safely contain the hash of itself, and a manifest cannot hash itself without a circular definition. Therefore:

- the **expected hub commit** is part of the external execution handoff, then recorded as observed runtime/ledger evidence;
- the **prompt-pack digest** is `sha256(pack.files.sha256)`;
- `pack.files.sha256` covers immutable phase inputs but excludes itself, `PROMPT_PACK_SHA256.txt`, `build_ledger.yaml`, `reports/`, and `runtime/`.

This gives deterministic pack identity without circular hashes.

## Source identity

For Git sources:

```text
BASE_SOURCE_COMMIT_SHA = required
SOURCE_ARCHIVE_SHA256 = NONE
```

For archive sources:

```text
BASE_SOURCE_COMMIT_SHA = NONE
SOURCE_ARCHIVE_SHA256 = required
```

An archive import is initialized as a local Git repository if necessary, producing `LOCAL_BASELINE_COMMIT_SHA`. That local commit does not replace the archive SHA; both are recorded because they answer different questions.

## Markitdown rule

`markitdown` creates a derived input. It must never replace original-byte identity.

Correct order:

```text
original file
    ↓ sha256
original SHA-256
    ↓ markitdown
Markdown derivative
    ↓ sha256
derived SHA-256
```

Use `phase_0/commands/normalize-input.sh ORIGINAL OUTPUT.md` for this operation. Cloud Document Intelligence / Content Understanding is not part of the default path.

## Phase design and sealing

GPT/browser prepares the next phase by:

1. reading the current ledger and latest audited source commit;
2. writing a bounded builder prompt;
3. writing required verification and stop conditions;
4. identifying the source mode and exact source identity;
5. filling `manifests/build-inputs.yaml`;
6. running `commands/seal-pack.sh`;
7. committing/pushing the phase pack;
8. returning an execution handoff containing the resulting hub commit and prompt-pack digest.

The execution handoff is intentionally external to the sealed commit so the commit can be named without self-reference.

## Workspace preparation

Incoming artifacts stage under:

```text
~/Downloads/<source>-dev-workspace
```

Actual development repositories live under:

```text
~/repos/build-dev/<source>
```

`prepare-workspace.sh` refuses to overwrite an existing build directory unless `--replace` is explicitly supplied. Archive SHA-256 is checked before extraction. Git sources are cloned and detached at the exact expected commit.

## Pre-build identity gate

Before implementation, `verify-inputs.sh` must prove:

1. hub checkout equals the externally supplied expected hub commit;
2. hub worktree is clean;
3. sealed prompt-pack files match `pack.files.sha256`;
4. `sha256(pack.files.sha256)` equals the externally supplied pack digest;
5. builder prompt matches `builder_prompt_sha256`;
6. source commit/archive hash matches the phase manifest;
7. prepared source worktree is clean.

Failure is a hard stop. Do not regenerate hashes to make a mismatch pass.

## OpenCode freshness gate

The installed OpenCode binary is the syntax authority for execution. Immediately before building, the runner captures:

```text
opencode --version
opencode run --help
```

It hashes the exact `run --help` snapshot and verifies that required flags are actually advertised before using them.

The configured model/variant come from `build-inputs.yaml`. `--auto` is not silently enabled; the human must pass `--auto` to the wrapper. This keeps headless auto-approval visible in the execution record.

## Builder run

Canonical form:

```bash
./phase_0/commands/run-builder.sh \
  "$EXPECTED_HUB_COMMIT" \
  "$EXPECTED_PROMPT_PACK_SHA256"
```

Explicit headless auto-approval:

```bash
./phase_0/commands/run-builder.sh \
  "$EXPECTED_HUB_COMMIT" \
  "$EXPECTED_PROMPT_PACK_SHA256" \
  --auto
```

The wrapper captures the OpenCode transcript and mechanical Git evidence even if the model process exits non-zero.

## Completion contract

A build is not complete merely because OpenCode exits successfully.

Required evidence:

```text
result commit SHA
clean/dirty working tree
files changed from base
build/verification commands
verification results
known failures
unresolved issues
scope deviations
generated artifacts
OpenCode version
OpenCode exact-command help hash
OpenCode transcript
mechanical evidence report
```

Intentional repository changes must be committed. An unexplained dirty worktree is audit evidence, not something to hide.

## GPT/browser audit

After the builder returns, GPT/browser audits the exact result commit against the exact sealed prompt pack.

It must:

1. inspect the result tree/diff;
2. compare every required task with repository evidence;
3. inspect tests and verification output;
4. check scope boundaries and uncommitted residue;
5. distinguish demonstrated, partially demonstrated, unverified, and false claims;
6. write one disposition: `PASS`, `REPAIR`, or `BLOCK`;
7. record the audit in `build_ledger.yaml`;
8. use a PASS result commit as the next phase's base revision.

## Ledger state model

Suggested progression:

```text
planned
→ sealed
→ running
→ built
→ audit_pending
→ passed
```

Failure branches:

```text
repair
blocked
abandoned
```

The JSON Schema in `phase_0/schemas/build-ledger.schema.json` fixes the Phase-0 field contract while allowing future schema versions to evolve deliberately.

## Security / trust boundary

A SHA-256 stored beside an artifact proves identity relative to that digest; it does not prove who authored the digest or that GitHub itself was not modified.

Phase 0 uses:

```text
Git history
exact commit SHAs
clean-worktree gates
SHA-256 pack/source identity
runtime help capture
mechanical result evidence
```

Later hardening may add signed commits, signed tags, release manifests, CI attestations, or branch protection. They are not Phase-0 prerequisites.

Never place credentials, API keys, `.env` contents, tokens, or private secrets in prompt packs, transcripts intended for commit, or reports. If a secret is discovered in commit-bound material, stop.

## TODO disposition

Completed in this bootstrap:

- refine end-to-end flow;
- define build-ledger schema;
- define prompt-pack directory standard;
- write `prepare-workspace.sh`;
- write `verify-inputs.sh`;
- write `run-builder.sh`;
- define commit/report requirements;
- define GPT post-build audit procedure;
- integrate `markitdown` as a provenance-preserving derived-input step;
- define non-circular pack identity/sealing;
- add mechanical post-build evidence capture;
- add OpenCode version/exact-help freshness capture;
- create indexed HTML5 human+agent README.

Already completed by user:

- create `ricks-llm-builder-hub` repository (initially private; now public).

Runtime-only remaining work:

- fill one real phase manifest and builder prompt;
- commit/push the sealed pack and record the external handoff SHA/digest;
- run one complete loop on a small real repository using the installed OpenCode binary;
- audit that exact result commit;
- update the ledger to PASS / REPAIR / BLOCK.

That final loop cannot be truthfully marked complete until it runs on the user's machine against the real OpenCode execution surface.

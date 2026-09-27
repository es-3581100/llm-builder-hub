# BUILD TASK — PHASE 3: BOUNDED SOURCE-FILE MUTATION

## ROLE

You are the primary implementation agent for **Phase 3** of Rick's LLM Builder Hub local workstation.

Build working software. Do not merely propose architecture.

This is a bounded extension of the accepted Phase-2 local workstation. Preserve the proven shell and repository-observation model. Add only the smallest source-mutation slice defined by `PHASE3_SOURCE_MUTATION_MODEL.md`.

Do not begin Phase 4.

---

# 0. AUTHORITY AND BASELINE

Repository:

```text
es-3581100/llm-builder-hub
```

Required working branch:

```text
phase3/source-file-mutation
```

Accepted Phase-2 source baseline:

```text
8b637f930dadd960ff98fee771240d948f3559cf
```

Phase-3 design contract:

```text
local_app/PHASE3_SOURCE_MUTATION_MODEL.md
```

The Phase-3 branch was created directly from the accepted Phase-2 commit. Phase 2 has already passed its real destination-host/browser acceptance. Do not reopen or redesign Phase 2 unless a Phase-3 regression proves a real inherited defect.

Before modifying anything:

1. verify the repository identity and current branch;
2. verify the branch descends from `8b637f930dadd960ff98fee771240d948f3559cf`;
3. require a clean worktree;
4. read:
   - `local_app/PHASE3_SOURCE_MUTATION_MODEL.md`
   - `local_app/PHASE2_REPOSITORY_MODEL.md`
   - `local_app/STATE_MODEL.md`
   - `local_app/UI_CONTRACT.md`
   - `local_app/README.md`
   - `PHASE_COMPLETION_SOP.md`;
5. inspect the complete `local_app/` source and tests;
6. run the existing Phase-2 verifier before implementation.

If the baseline does not reproduce, stop with:

```text
PHASE3_BASELINE_FAIL
```

Do not patch around a failing baseline and call it Phase 3.

---

# 1. PRIMARY OBJECTIVE

Implement exactly this new authority path:

```text
existing eligible repository text document
        ↓ explicit EDIT SOURCE
browser source draft
        ↓ explicit SAVE SOURCE
loopback service validates identity + current bytes
        ↓ atomic worktree replacement
fresh repository inspection
        ↓
ordinary unstaged Git worktree change
```

The user must be able to edit an already-materialized repository text document in the local browser and explicitly save that source file.

This is **filesystem source mutation only**.

It is not Git staging, committing, branch management, remote synchronization, or OpenCode execution.

---

# 2. NON-NEGOTIABLE AUTHORITY SEPARATION

Preserve these three independent surfaces:

```text
WORKSTATION STATE
browser workstation draft
→ SAVE CHANGES
→ persisted workstation JSON

REPOSITORY OBSERVATION
configured local Git worktree
→ read-only Git inspector
→ RepositorySnapshot

SOURCE MUTATION
eligible repository document
→ source draft
→ SAVE SOURCE
→ atomic filesystem replacement
→ fresh RepositorySnapshot
```

These actions must never collapse together:

```text
SAVE CHANGES != SAVE SOURCE
SAVE SOURCE  != git add
SAVE SOURCE  != git commit
SAVE SOURCE  != SEAL
SAVE SOURCE  != RUN
SAVE SOURCE  != PUBLISH
```

Do not reuse `POST /api/save` for source-file writes.

---

# 3. TARGET ELIGIBILITY

Phase 3 may write only an **existing repository document already materialized by the server**.

A writable target must be all of:

- in the configured non-bare repository worktree;
- represented by a fresh server-side repository inspection;
- existing;
- a regular file;
- not a symlink;
- not binary;
- not oversized;
- within the existing repository-document size bound;
- identified by server-issued repository/document identity.

Do not accept a browser-supplied filesystem path as write authority.

Out of scope:

```text
create file
delete file
rename file
chmod
symlink write/follow
directory mutation
submodule mutation
.git mutation
```

If the current repository model cannot safely resolve a document ID back to an eligible file, add the smallest server-side resolution layer necessary. Do not weaken the identity model.

---

# 4. CONTENT IDENTITY / CONFLICT PROTECTION

Every materialized repository text document must expose a SHA-256 of its current file bytes.

A source-save request must bind to the minimum equivalent of:

```text
repository_id
document_id
expected_content_sha256
content
```

Do not make a raw path authoritative request input.

Before writing, freshly inspect/resolve the configured repository and re-check the target.

Required stale-write rule:

```text
request.expected_content_sha256 == current bytes SHA-256
```

If false:

- return HTTP 409;
- do not overwrite;
- preserve the externally changed bytes;
- return/show a clear source-conflict state.

Test a real race-shaped case:

1. browser/API loads document at hash A;
2. external fixture operation changes file to hash B;
3. stale save still claims A;
4. server returns 409;
5. B remains byte-identical.

Do not implement "last writer wins."

---

# 5. ATOMIC SOURCE WRITE

Implement the source write as an atomic local filesystem replacement.

Required properties:

1. server resolves the target;
2. server revalidates it is still eligible;
3. temporary file is created in the same directory;
4. existing file permission bits are preserved;
5. complete replacement content is written;
6. file is closed/synced appropriately;
7. temporary file is atomically renamed over the target;
8. temporary residue is removed on failure;
9. symlinks are never followed as writable targets.

Do not normalize line endings or silently rewrite unrelated content.

Keep the implementation small and auditable.

---

# 6. GIT INVARIANTS

`SAVE SOURCE` mutates the worktree file only.

It must not itself change:

```text
HEAD
symbolic branch
.git/index
staged changes
Git config
remote refs
remote state
```

After a successful tracked-file source edit, a fresh repository snapshot should show the path as an **unstaged** modification.

No mutating Git command is authorized.

Do not invoke:

```text
git add
git reset
git restore
git checkout
git switch
git commit
git merge
git rebase
git fetch
git pull
git push
git config
```

Existing Phase-2 read-only Git inspection remains allowed.

Tests must mechanically compare:

- HEAD before/after;
- branch before/after;
- `.git/index` SHA-256 before/after;
- staged state before/after.

---

# 7. API

Add one narrow loopback source-mutation endpoint.

Name/path is your implementation choice, but keep it explicit and source-specific.

The endpoint must:

- accept only the source-mutation request;
- reject unknown JSON fields;
- use a bounded request body;
- reject invalid repository identity;
- reject unknown document identity;
- reject ineligible targets;
- use HTTP 409 for stale-content conflict;
- never trust browser repository snapshot/path data;
- return enough fresh authoritative repository/document data for the browser to reconcile after success.

Do not expose generic filesystem write, shell, command, or Git-command endpoints.

---

# 8. BROWSER UX

Repository documents were read-only in Phase 2. Extend only those eligible documents with explicit source editing.

Required behavior:

```text
VIEW
→ EDIT SOURCE
→ browser-only source draft
→ SAVE SOURCE or CANCEL SOURCE EDIT
```

Requirements:

- repository docs remain read-only until explicit `EDIT SOURCE`;
- use a source draft separate from workstation-state draft;
- textarea focus/content must survive clicks inside the active source editor;
- `CANCEL SOURCE EDIT` performs no server write;
- `SAVE SOURCE` writes exactly the selected eligible source document;
- workstation `SAVE CHANGES` remains workstation-state-only;
- workstation `CLEAR EDITS` must not silently write source;
- a source conflict must be visibly distinct from workstation revision conflict;
- after successful save, reconcile with a fresh repository snapshot;
- the edited path should visibly appear as unstaged;
- source editor state must not be accidentally destroyed by ordinary document activation/rendering.

Preserve the Phase-2 fix from commit `8b637f9` that prevents an open editor from being destroyed by clicking inside it. Extend its principle to source editing rather than regressing it.

Do not add fake or disabled placeholder controls for later phases.

---

# 9. STATIC EXPORT

Static export remains read-only.

Prove the exported HTML:

- contains no source editor;
- contains no `SAVE SOURCE`;
- contains no source-mutation request code;
- does not expose a generic mutation endpoint;
- still represents current saved/observed repository content and provenance as appropriate.

---

# 10. TEST-FIRST IMPLEMENTATION

Use TDD for every security/authority boundary.

Before production implementation, add failing tests for the smallest safe behavior.

At minimum prove all of the following.

## Server/repository tests

1. eligible existing text file can be replaced;
2. stale expected hash returns HTTP 409;
3. stale conflict preserves external bytes;
4. wrong `repository_id` is rejected;
5. unknown `document_id` is rejected;
6. symlink is rejected and target bytes remain untouched;
7. binary/ineligible target is rejected;
8. oversized target is rejected;
9. arbitrary path traversal is not representable as write authority;
10. file permission bits are preserved;
11. HEAD unchanged after save;
12. symbolic branch unchanged after save;
13. `.git/index` SHA unchanged after save;
14. no staged change introduced;
15. successful edit appears as unstaged;
16. repository ID remains stable;
17. document ID remains stable for ordinary content edit;
18. content SHA changes to match new bytes;
19. no temporary residue remains after successful write;
20. relevant failure paths do not leave partial replacement content.

## Existing-boundary regressions

21. workstation `SAVE CHANGES` still cannot write repository source;
22. workstation `CLEAR EDITS` performs no source write;
23. workstation revision semantics remain unchanged by source save;
24. Phase-1 dirty-refresh guard remains correct;
25. Phase-2 detached/unborn/read-only inspection behavior remains correct.

## Browser tests

26. eligible repository document exposes explicit source-edit entry;
27. entering source edit does not affect workstation dirty indicator by itself;
28. typing changes only source draft until `SAVE SOURCE`;
29. clicking inside active source editor does not rebuild/destroy it;
30. canceling source edit performs no mutation request;
31. saving source issues the narrow request with repository/document/hash identity;
32. HTTP 409 produces a visible source-conflict state;
33. successful save reconciles fresh repository state;
34. ordinary workstation `SAVE CHANGES` does not include source draft;
35. static export remains mutation-free.

Do not delete or weaken existing tests to make Phase 3 pass.

---

# 11. VERIFIER

Add:

```text
local_app/scripts/verify-phase3.sh
```

It must compose prior verification rather than replace it.

At minimum it should run the existing Phase-2 verification, clean only its known temporary build artifact if necessary, then run all Phase-3 Go/race/browser-unit checks.

Preserve executable mode on verification scripts.

A PASS requires zero ignored failures.

---

# 12. HOST ACCEPTANCE

Create a bounded host acceptance document/script if real-browser behavior is not fully mechanically demonstrated.

Preferred artifacts:

```text
local_app/PHASE3_HOST_ACCEPTANCE.md
local_app/scripts/host-accept-phase3.sh
```

Use a disposable Git repository, never the Builder Hub itself as the mutation target.

The acceptance must mechanically preserve evidence before/after:

```text
source SHA-256
HEAD
branch
.git/index SHA-256
staged status
unstaged status
repository snapshot
```

The human browser portion should verify:

1. real browser loads real loopback service;
2. repository source document enters EDIT SOURCE;
3. typing is preserved;
4. CANCEL causes no file change;
5. SAVE SOURCE changes only the expected source file;
6. fresh UI marks it unstaged;
7. induced stale-content conflict is visibly refused.

The mechanical tail must prove:

- source bytes changed only for the intended successful save;
- HEAD unchanged;
- branch unchanged;
- index SHA unchanged;
- no staged changes added;
- conflict attempt did not overwrite externally changed bytes.

Browser-only proof may remain `HOST_ACCEPTANCE=PENDING` if the execution session cannot perform it, but the acceptance procedure itself is BUILD_REQUIRED.

---

# 13. DOCUMENTATION

Update documentation so the implemented authority is explicit.

At minimum reconcile:

```text
local_app/README.md
local_app/STATE_MODEL.md
local_app/UI_CONTRACT.md
```

Add:

```text
local_app/PHASE3_BUILD_REPORT.md
local_app/PHASE3_TEST_REPORT.md
```

The reports must distinguish:

```text
build_status
host_acceptance_status
publication_status
known limitations
deferred Phase-4+ work
```

Do not rewrite historical Phase-1/Phase-2 reports to pretend they contained Phase 3.

---

# 14. EXPLICITLY DEFERRED

Do **not** implement:

```text
new-file creation
delete/rename
git add/reset/restore
commit creation
branch switching
merge/rebase
fetch/pull/push
remote synchronization
OpenCode execution
actor execution
SEAL
RUN
PUBLISH
GitHub mutation
OAuth
plugin architecture
multi-user collaboration
graduation automation
```

If safe source mutation truly requires one of these, stop and report:

```text
PHASE3_SCOPE_EXPANSION_REQUIRED
```

Do not silently expand scope.

---

# 15. SAFETY / STOP CONDITIONS

Hard stop on:

- baseline verifier failure;
- unexpected pre-existing dirty worktree;
- repository identity mismatch;
- evidence that the design would require trusting a browser path;
- inability to provide stale-content conflict protection;
- test proving writes can escape the configured worktree;
- symlink-follow write;
- Git index/HEAD mutation caused by SAVE SOURCE;
- need for generic shell/command execution;
- unresolved repository-controlled failing tests.

Return one of:

```text
PHASE3_IMPLEMENTATION_PASS
PHASE3_REPAIR_REQUIRED
PHASE3_BLOCKED
PHASE3_SCOPE_EXPANSION_REQUIRED
NEW_BASELINE_DEFECT_FOUND
```

Do not proceed into Phase 4.

---

# 16. COMMIT / GIT-READY CONTRACT

All intentional Phase-3 repository changes must be committed on:

```text
phase3/source-file-mutation
```

Use focused commits while building, then finish with a clean worktree.

Do not force-push.

Do not merge to `main`.

Do not rewrite the accepted Phase-2 branch.

The final implementation checkpoint must report:

```text
BASELINE_SHA
PHASE3_RESULT_SHA
branch
git status --short
files changed from baseline
test commands
test results
host acceptance status
known limitations
scope deviations
next smallest phase
```

If the environment cannot publish, classify publication separately according to `PHASE_COMPLETION_SOP.md`.

---

# 17. EXECUTION ORDER

Use this order:

1. identity/preflight;
2. existing Phase-2 verification;
3. inspect current implementation;
4. add failing source-mutation model/server tests;
5. implement server-side identity/hash/atomic-write core;
6. pass server/repository tests;
7. add failing browser-state tests;
8. implement explicit repository source editor;
9. preserve workstation-state separation;
10. verify static export remains read-only;
11. add composed Phase-3 verifier;
12. run full verifier including race tests;
13. add bounded host-acceptance procedure;
14. reconcile docs/reports;
15. independent adversarial review of path, symlink, stale-write, Git-index, and source/workstation authority boundaries;
16. repair any supported finding;
17. rerun full verification;
18. commit final clean checkpoint;
19. stop.

Do not substitute TODOs, comments, planned tests, mocks, or prose for required behavior.

Do not stop after the happy path.

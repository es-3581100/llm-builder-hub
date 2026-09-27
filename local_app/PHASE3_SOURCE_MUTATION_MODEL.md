# Phase-3 Source Mutation Model

## Baseline

Phase 3 starts from the completed Phase-2 branch state:

```text
branch: phase2/local-repository-semantics
baseline commit: 8b637f930dadd960ff98fee771240d948f3559cf
host browser acceptance: PASS
```

Phase 3 is deliberately narrower than "Git mutation" or "execution".

## Objective

Add one explicit local-authority mutation surface:

```text
existing eligible repository text document
        ↓ explicit EDIT SOURCE
browser source draft
        ↓ explicit SAVE SOURCE
loopback service validation
        ↓ atomic worktree file replacement
fresh repository inspection
        ↓
unstaged Git worktree change
```

This phase does not stage, commit, switch branches, contact remotes, or run OpenCode.

## Authority separation

The workstation now has three independent authorities:

```text
WORKSTATION STATE
browser draft → SAVE CHANGES → local workstation JSON

REPOSITORY OBSERVATION
local Git worktree → read-only inspector → RepositorySnapshot

SOURCE MUTATION
eligible repository document
  → browser source draft
  → SAVE SOURCE
  → validated atomic filesystem write
  → fresh RepositorySnapshot
```

The actions must remain visibly distinct:

```text
SAVE CHANGES != SAVE SOURCE
SAVE SOURCE  != git add
SAVE SOURCE  != git commit
SAVE SOURCE  != SEAL
SAVE SOURCE  != RUN
SAVE SOURCE  != PUBLISH
```

## Eligible mutation target

Phase 3 may modify only an existing source document that the server itself materialized from the configured repository.

The target must be:

- in the configured non-bare worktree;
- represented by the current repository snapshot;
- an existing regular file;
- not a symlink;
- not binary;
- not oversized;
- within the existing repository-document size bound;
- identified by server-issued repository/document identity rather than a browser-supplied filesystem path.

No create, delete, rename, chmod, symlink mutation, directory mutation, submodule mutation, or Git metadata mutation is in scope.

## Mutation request identity

The browser must not be allowed to choose an arbitrary path.

A source-save request must bind to server-issued identity and stale-content protection. The request should contain the minimum equivalent of:

```text
repository_id
document_id
expected_content_sha256
new content
```

A raw path is not authoritative request identity.

Before writing, the server must freshly resolve the configured repository, locate the requested document by identity, and verify that the current file still satisfies the eligibility rules.

## Conflict rule

Every materialized repository text document must expose a SHA-256 of its current bytes.

`SAVE SOURCE` succeeds only when:

```text
request.expected_content_sha256 == current file SHA-256
```

If another process edits the file after the browser loaded it, the server must refuse the stale write with HTTP 409 and leave the external bytes untouched.

The UI must expose that source conflict distinctly from workstation-state revision conflicts.

## Atomic write rule

Successful source mutation must:

1. resolve the server-owned target;
2. revalidate target type/identity;
3. create a temporary file in the target directory;
4. preserve the existing regular file's permission bits;
5. write the complete replacement bytes;
6. sync/close as appropriate for the implementation;
7. atomically rename the temporary file onto the target;
8. clean up temporary residue on failure;
9. never write through a symlink.

Do not silently normalize line endings or rewrite unrelated bytes.

## Git invariants

A successful source save is a filesystem worktree edit only.

Mechanically prove that it does not itself change:

```text
HEAD commit
current branch
.git/index bytes
staged changes
Git config
remote state
```

After a successful save, the fresh repository snapshot should normally show the edited path as unstaged.

No mutating Git command is authorized in this phase.

Forbidden examples include:

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

Read-only Git inspection from Phase 2 remains allowed.

## Browser semantics

Repository documents may gain explicit source editing controls.

Required behavior:

- repository documents remain read-only until the user explicitly enters source-edit mode;
- source editing uses a separate browser draft from workstation-state draft;
- clicking inside an open source editor must not destroy/recreate the focused textarea;
- `CANCEL SOURCE EDIT` discards only that source draft and performs no write;
- `SAVE SOURCE` writes only the selected source file;
- workstation `SAVE CHANGES` remains workstation-state-only;
- workstation `CLEAR EDITS` must not silently write or discard an unrelated source edit;
- repository refresh after a clean source save displays the new bytes and fresh content SHA;
- an HTTP 409 source conflict is visible and must not overwrite the external edit.

Do not add fake SEAL, RUN, COMMIT, PUSH, or PUBLISH controls.

## API semantics

Use a narrow loopback mutation endpoint rather than expanding `POST /api/save`.

The exact path/name is an implementation choice, but it must:

- accept only the source-mutation request shape;
- reject unknown fields;
- enforce a bounded request body;
- return clear 4xx errors for invalid/ineligible requests;
- return HTTP 409 for stale-content conflicts;
- return a fresh authoritative repository snapshot/document state on success;
- never accept repository snapshot data from the browser as authority.

## Static export

Static HTML export remains read-only.

It may show the latest saved repository content/provenance, but it must contain no source mutation endpoint code, editable repository textarea, or source-save control.

## Verification requirements

At minimum add deterministic tests proving:

1. happy-path replacement of an eligible existing text file;
2. stale expected hash returns 409 and preserves external bytes;
3. wrong repository identity is rejected;
4. unknown document identity is rejected;
5. symlink targets cannot be written;
6. binary/ineligible targets cannot be written;
7. oversized targets cannot be written;
8. browser-supplied path traversal is impossible because the API does not trust a path;
9. permission bits are preserved;
10. HEAD and branch are unchanged;
11. `.git/index` SHA-256 is unchanged;
12. no staged change is introduced;
13. the resulting source edit appears as unstaged in a fresh snapshot;
14. repository/document identity remains stable for an ordinary content edit;
15. Phase-1/Phase-2 SAVE/CLEAR/REFRESH behavior remains intact;
16. repository editor focus is preserved while typing;
17. CANCEL SOURCE EDIT performs no write;
18. SAVE SOURCE does not modify workstation JSON revision/state;
19. static export remains mutation-free;
20. all existing Phase-1 and Phase-2 tests continue to pass.

Add a Phase-3 verifier that composes the earlier verifiers rather than replacing them.

## Host acceptance

If browser-only behavior cannot be fully demonstrated mechanically, add a bounded destination-host acceptance script using a disposable Git repository.

It must prove at least:

- real browser → real loopback service;
- explicit source edit and SAVE SOURCE change exactly one expected file;
- changed file SHA differs as expected;
- HEAD is unchanged;
- `.git/index` SHA is unchanged;
- no staged changes appear;
- fresh UI/repository state shows the path as unstaged;
- a deliberately induced stale-content conflict refuses overwrite;
- no remote/network Git operation occurs.

Classify browser-only proof as `HOST_ACCEPTANCE` per the repository SOP rather than hiding an implementation defect.

## Explicitly deferred

Do not implement in Phase 3:

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
actors
SEAL
RUN
PUBLISH
GitHub mutation
OAuth
plugin architecture
multi-user collaboration
graduation automation
```

## Completion condition

Phase 3 is implementation-complete only when repository-controlled work is committed, the full composed test suite passes, source mutation invariants are mechanically demonstrated, documentation matches behavior, and any remaining browser-only proof has an explicit host-acceptance procedure.

Do not begin Phase 4.

# Phase-2 Repository Model

## Boundary

Phase 2 adds observation, not Git authority.

```text
LOCAL GIT REPOSITORY
        ↓
read-only inspector
        ↓
RepositorySnapshot
        ↓
API / live HTML / static HTML
```

The inspector invokes only local Git read operations and sets:

```text
GIT_OPTIONAL_LOCKS=0
GIT_TERMINAL_PROMPT=0
GIT_PAGER=cat
LC_ALL=C
```

Diff capture explicitly disables external diffs and text conversion. No remote command is used.

## Repository identity

`repository_id` is a SHA-256 identity derived from repository-specific local evidence including the canonical worktree root, Git common directory, object format, and known root commits. It is not a folder basename.

The identifier is intended to remain stable across ordinary refreshes, edits, staging changes, branch movement, and new descendants of the same history. Reinitializing/moving a repository or introducing unrelated root histories can legitimately change identity.

## HEAD state

The snapshot distinguishes:

```text
normal symbolic branch
DETACHED HEAD
UNBORN branch / new repository
```

`head_commit` is the current commit when one exists. Dirty working-tree state is represented separately and never disguised as a new authoritative commit.

## Working-tree state

Repository state contains separate arrays for:

```text
staged
unstaged
untracked
```

A path can therefore appear in both staged and unstaged sets.

Rename/copy records preserve `previous_path` when Git reports it. Deleted tracked files remain represented as source relationships even when no worktree document can be loaded.

## Source relationships

Every tracked/untracked path discovered by the inspector receives a repository-scoped `source` identity. Relationships expose:

```text
path
previous path when available
tracked / untracked
exists
regular / symlink
symlink target metadata
size
binary
oversized
staged status
unstaged status
Git-backed document ID when loaded
content authority
```

Symlinks are never followed for document loading.

## Git-backed documents

Eligible worktree text files become read-only repository documents.

```text
source: git_worktree
authority: repository
editable: false
```

Document identity is derived from:

```text
repository_id + relative path
```

so content edits do not change document identity.

The first vertical slice deliberately bounds document materialization to:

```text
64 documents
256 KiB per document
```

The full file relationship list still records skipped binary, oversized, symlink, deleted, and over-limit paths.

## Workstation state remains separate

The Phase-1 persisted JSON state remains the only target of `SAVE CHANGES`.

Changing workstation metadata such as the saved project-root label does **not** retarget the Git inspector. The inspected repository is selected by the server's `-repo` argument.

Therefore:

```text
SAVE CHANGES
!= write source file
!= git add
!= git commit
!= checkout
```

## Refresh

`REFRESH` retains its Phase-1 dirty-draft guard.

When refresh is permitted, `/api/state` reloads both:

```text
persisted workstation state
fresh repository snapshot
```

This is the only Phase-2 semantic extension to REFRESH.

## Static export

Static export still reads persisted workstation state rather than browser draft state. Phase 2 additionally captures a fresh read-only repository snapshot and the bounded Git-backed documents.

The export contains no mutation controls or Git command surface.

## Known representation limits

- non-UTF-8 filenames are not guaranteed to round-trip losslessly through JSON/HTML;
- Git-backed document materialization is bounded as described above;
- rename history is explicit in the current snapshot, but long-term identity after a completed rename remains path-derived;
- diffs are currently captured in full and can make exports large on very large working-tree changes;
- direct browser-to-loopback acceptance remains a destination-host item exactly as documented in Phase 1.

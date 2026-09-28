# Phase 4 Host Acceptance

```text
status=PASS
phase=4
implementation=dee699794b6a36f752c7770610cd4f27f82433f6
host_acceptance=PASS
browser_checks=75
browser_checks_passed=75
browser_checks_failed=0
```

## Run shape

Fully automated and non-interactive. No human y/n, no manual browser steps, no
hand-edited test files, no counting labels by eye.

```text
script=local_app/scripts/host-accept-phase4.sh
driver=local_app/scripts/host-accept-phase4.py
browser=/usr/bin/thorium-browser (headless)
python=/home/sticky-ricky/anaconda3/bin/python
service=real llm-hub-local binary on 127.0.0.1:18767
fixture=disposable `git init` repository under /tmp, path contains a space
origin=real bare repository, pushed once during setup so a remote-tracking ref exists
evidence=/home/sticky-ricky/.local/state/llm-hub/phase4-host-acceptance/20260928T013018Z
```

The harness refuses to start against a dirty Builder-Hub worktree, emitting
`PHASE4_HOST_ACCEPTANCE_BLOCKED: Builder-Hub worktree must be clean` with exit 2. The
accepted run was executed from a throwaway local `git clone` in which the harness files were
tracked, which is how a clean-worktree run is legitimately obtained. The harness's own
mechanism is a disposable `git init` fixture; it does not clone. The build repository's HEAD
and porcelain status are asserted identical before and after the run.

## Result

```text
PHASE4_LOCAL_VERIFY=PASS
PHASE4_BROWSER_AUTOMATION=PASS
PHASE4_WRITE_FILE_ACCEPTANCE=PASS
PHASE4_INDEX_IDENTITY=PASS
PHASE4_HEAD_IDENTITY=PASS
PHASE4_STAGE_ACCEPTANCE=PASS
PHASE4_UNSTAGE_ACCEPTANCE=PASS
PHASE4_COMMIT_ACCEPTANCE=PASS
PHASE4_BRANCH_STABLE=PASS
PHASE4_REMOTE_IMMUTABLE=PASS
PHASE4_COMMITTED_SNAPSHOT_CONSUMED=PASS
PHASE4_AUTHORITY_SEPARATION=PASS
PHASE4_SAVE_WRITE_SEPARATION=PASS
PHASE4_DRAFT_BLOCK=PASS
PHASE4_CONFLICT_VISIBILITY=PASS
PHASE4_LOG_REDACTION=PASS
PHASE4_EXPORT_READ_ONLY=PASS
PHASE4_HUB_REPO_UNTOUCHED=PASS
PHASE4_HOST_ACCEPTANCE=PASS
PHASE4_COMPLETE=PASS
```

## Properties proven

The chain driven through the real browser is:

```text
WRITE FILE -> STAGE FILE -> UNSTAGE FILE -> STAGE FILE -> COMMIT STAGED
```

1. **Worktree bytes after `WRITE FILE`** — read from disk and compared byte for byte.
2. **Index identity changes on stage and again on unstage** — measured from the filesystem,
   and cross-checked against the identity the API reported.
3. **HEAD unchanged through stage and unstage.**
4. **HEAD changes only on commit** — the new HEAD equals the reported `new_head_commit`,
   has exactly one parent, and that parent is the pre-commit HEAD.
5. **Branch unchanged** throughout.
6. **No remote mutation** — `git remote -v`, `refs/remotes`, and the bare `origin`'s refs are
   byte-identical before and after, and the remote-tracking ref still points at the
   pre-commit HEAD, proving nothing was pushed.
7. **After commit the staged count is zero** for the committed snapshot while an unrelated
   unstaged change remains.
8. **Authority separation** — `SAVE CHANGES` changes neither index nor HEAD; `WRITE FILE`
   does not stage; `STAGE FILE` does not commit; the commit-message textarea never marks
   the workstation dirty.
9. **A dirty source draft blocks stage, unstage, and commit** — a dialog fires, zero
   requests reach the three endpoints, and HEAD, branch, index, and worktree bytes are
   unchanged.
10. **A 409 conflict stays visible and does not clobber** — the browser is deliberately
    desynchronized from the worktree, `STAGE FILE` returns 409, `#commit-error` shows
    `WORKTREE_CONFLICT` with status 409, and the external bytes are not overwritten.
11. **Server logs** carry identities and status for each successful mutation and contain
    neither file contents nor the commit message.
12. **Static export stays read-only** — the exported HTML contains none of `STAGE FILE`,
    `UNSTAGE FILE`, `COMMIT STAGED`, `/api/stage-file`, `/api/unstage-file`, `/api/commit`,
    `<textarea`, `WRITE FILE`, `EDIT SOURCE`, `CANCEL SOURCE EDIT`, `/api/write-file`.
13. **The build repository is untouched** across the run.

## Independent Git evidence

```text
stage1_index_sha256=0b4696ded8d7f5229cc13b106e07ff5a27d960aa2c34a0e0441234e8042b5dca
unstage_index_sha256=3b56fa687c16b66df633f2e6aff2af373967a44df33c6354851be52877eabf57
restage_index_sha256=0b4696ded8d7f5229cc13b106e07ff5a27d960aa2c34a0e0441234e8042b5dca
commit_index_sha256=54ddc68e25f563161d638d80a12f64a09cf20f72c68fb6b003fe547b61b8f9ba
new_head_commit=bc16814346f1384ca4dafecb91e0c68751e2f5b9
pre_commit_head=b89d8102dd508714a007000e6177a5d92854c00b
```

Resulting history shows exactly one parent and no invented message suffix:

```text
bc16814346f1384ca4dafecb91e0c68751e2f5b9 b89d8102dd508714a007000e6177a5d92854c00b Phase Four Automated Acceptance phase4 acceptance commit 9f3a1c
b89d8102dd508714a007000e6177a5d92854c00b  Phase Four Automated Acceptance phase4 automated host baseline
```

Final fixture status is ` M unrelated.txt`.

## Reproducing

```bash
PHASE4_PYTHON=/path/to/python-with-playwright local_app/scripts/host-accept-phase4.sh
```

`PHASE4_PYTHON` is required because the default `python3`/`python` on `PATH` do not have
Playwright. Exit 0 on success, 1 on an assertion failure, 2 on an environment or
clean-worktree blocker.

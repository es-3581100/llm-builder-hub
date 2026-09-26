# Phase Completion SOP

This SOP defines the smallest decision rules needed to keep a completed build from being mislabeled `PARTIAL` merely because the current execution environment lacks a target-host, publication, or human-policy capability.

It does **not** permit missing implementation work to be hidden.

## 1. Classify every incomplete requirement

Before final disposition, classify each remaining item as exactly one of:

- `BUILD_REQUIRED` — repository-controlled implementation, repair, test, documentation, packaging, or verification required for the phase artifact itself.
- `HOST_ACCEPTANCE` — proof that can only run on the destination host/session/service, such as Firefox GUI behavior, desktop notifier behavior, real cron/systemd installation, hardware access, or another machine-specific integration.
- `PUBLICATION` — remote repository creation, push, remote read-back, release publication, or other external publishing action.
- `HUMAN_DECISION` — a decision the agent must not invent, such as baseline promotion, license selection, destructive migration approval, or another policy choice.
- `PROHIBITED` — an operation intentionally forbidden by the phase or safety policy.

Unknown items are not silently downgraded. Investigate them or keep them build-blocking.

## 2. Build PASS rule

A Phase build may be reported as PASS when all of the following are true:

1. every `BUILD_REQUIRED` requirement is DONE and verified;
2. there are no known failing repository-controlled tests or checks;
3. the intentional worktree is clean and committed;
4. every remaining `HOST_ACCEPTANCE`, `PUBLICATION`, `HUMAN_DECISION`, or `PROHIBITED` item is explicitly recorded;
5. every pending `HOST_ACCEPTANCE` item has a concrete executable acceptance procedure, expected success result, and failure condition;
6. no pending external item is being used to hide a known implementation defect.

Pending external acceptance does **not** convert a completed build into `PARTIAL`.

Report these dimensions separately:

```text
build_status: PASS | PARTIAL | BLOCKED
host_acceptance_status: PASS | PENDING | BLOCKED | NOT_APPLICABLE
publication_status: PASS | PENDING | BLOCKED | NOT_APPLICABLE
human_decision_status: RESOLVED | PENDING | NOT_APPLICABLE
```

The phase's main PASS/REPAIR/BLOCK disposition is based on `build_status` unless the phase contract explicitly states that a particular external action is itself part of the build deliverable.

## 3. Host-capability SOP

At the beginning of a completion run, record the capabilities actually available in the current environment.

At minimum inspect, when relevant:

```text
git
opencode
gh or authenticated Git remote
graphical session
Firefox/browser
desktop notifier
cron/systemd
required language/runtime/toolchain
```

For each capability record:

```text
AVAILABLE
UNAVAILABLE
PROHIBITED
NOT_APPLICABLE
```

If a required host capability is unavailable:

1. complete all repository-controlled implementation and fixture testing that can be done honestly;
2. create or preserve the exact acceptance command/script for the destination host;
3. record expected output and failure criteria;
4. classify the unresolved proof as `HOST_ACCEPTANCE`;
5. do not mark the underlying implementation requirement incomplete solely because the proof environment is absent.

If evidence shows the implementation itself is broken, classify it `BUILD_REQUIRED` and repair or block it.

## 4. Publication SOP

Publication is a separate status unless the phase explicitly defines remote publication as the build artifact.

Use this order:

1. existing authenticated Git remote + `git push`;
2. authenticated `gh` repository creation/push;
3. another explicitly authorized repository connector;
4. otherwise record `publication_status: PENDING` with the exact destination and command required.

Lack of repository-creation tooling does not invalidate a clean, verified local build.

Never force-push or overwrite an unrelated repository.

## 5. License / AI-TDM SOP

Do not invent a license or AI/TDM policy.

A missing project license is handled as follows:

- **private/unpublished project:** `HUMAN_DECISION=PENDING`; this does not block the build or private local completion;
- **public publication:** require an explicit licensing disposition before claiming publication complete;
- **third-party material with license obligations:** compatibility remains build/publication blocking until resolved.

AI/TDM policy is optional unless the project contract explicitly requires one.

"No license selected yet" is a valid recorded state; it is not permission to fabricate legal terms.

## 6. Artifact fidelity SOP

Content hashes alone do not fully describe an executable source tree.

When a flattened, HTML, archive-derived, or reconstructed artifact can lose filesystem semantics, preserve and verify at least:

```text
path
entry type
content SHA-256
executable bit / Git mode when semantically significant
symlink target when applicable
```

Do not preserve machine-specific owner/group/timestamps unless the project explicitly requires them.

A mode-loss defect discovered during reconstruction must be repaired, recorded, and regression-tested rather than silently chmodded.

## 7. Final disposition examples

A locally complete build with pending Firefox and GitHub publication proof:

```text
build_status: PASS
host_acceptance_status: PENDING
publication_status: PENDING
human_decision_status: RESOLVED
final_disposition: PASS
```

A build with a failing repository-controlled test:

```text
build_status: BLOCKED
host_acceptance_status: PENDING
publication_status: PENDING
final_disposition: BLOCK
```

A build whose code is complete but a required implementation TODO remains unfinished:

```text
build_status: PARTIAL
final_disposition: REPAIR
```

The rule is simple:

> Missing execution environment is not missing implementation. Missing implementation is still missing implementation.

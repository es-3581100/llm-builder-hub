# Runtime Test Records

This directory is the durable record of real Builder Hub runtime tests.

Each runtime test gets a stable ID and its own directory:

```text
runtime_tests/
├── index.yaml
├── TEMPLATE.md
├── RT-001-don-dawg-phase0/
│   ├── README.md
│   └── record.yaml
└── RT-002-.../
```

## Purpose

Runtime tests are not examples or synthetic fixtures. They record what happened when the Builder Hub workflow was exercised against a real project/build surface.

A runtime test may promote a lesson into Hub policy only when that lesson concerns the reusable handoff/lifecycle contract. Project-specific architecture discovered during a run remains project-local payload. See [`HUB_BOUNDARY.md`](../HUB_BOUNDARY.md).

A record should preserve:

- runtime-test ID;
- date;
- project/artifact identity;
- baseline and result commits;
- Builder Hub commit used;
- initial disposition;
- framework defects discovered;
- framework fixes made;
- retry/continuation result;
- repository-controlled verification;
- external host acceptance state;
- publication state;
- human-decision state;
- evidence that was independently verified;
- evidence that remains reported/local/unpublished;
- lessons promoted into Builder Hub policy.

## Status model

Use the phase-completion dimensions from `PHASE_COMPLETION_SOP.md`:

```text
build_status
host_acceptance_status
publication_status
human_decision_status
```

A runtime-test record may be marked `PASS` when the repository-controlled build passes under the live SOP. Pending external acceptance or publication remains visible and must not be rewritten as completed.

## Evidence rule

Do not collapse evidence classes.

Use:

- `VERIFIED_REMOTE` — independently readable from a remote authoritative source;
- `VERIFIED_HUB` — verified in this Builder Hub repository;
- `REPORTED_LOCAL` — reported by the execution agent from a local/unpublished project state;
- `PENDING_EXTERNAL` — requires a destination host, publication surface, or human decision.

When a later publication or host proof upgrades evidence, append/update the run record without deleting the earlier state.

## Append-only intent

Runtime-test IDs are never reused. Corrections should preserve the original result and record the correction/retry rather than pretending the first run never happened.

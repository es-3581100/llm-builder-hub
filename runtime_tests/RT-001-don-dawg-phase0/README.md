# RT-001 — Don-Dawg Phase-0

**Status:** PASS at the repository-controlled build boundary.

This is the first recorded real runtime test of the LLM Builder Hub workflow.

## What happened

The initial Don-Dawg Phase-0 completion run successfully reconstructed and verified the supplied HTML5 artifact, preserved the baseline, produced a clean Git history, and reported 23/23 regression tests passing.

Real execution exposed one artifact-fidelity defect: the HTML5 reconstruction preserved file contents but lost executable mode metadata. The resulting `Permission denied` failure was diagnosed, the three executable entrypoints were repaired, and a regression was added instead of silently applying `chmod`.

The initial run then returned:

```text
PHASE0_COMPLETION_PARTIAL
```

despite the repository-controlled implementation passing, because unavailable destination-host checks, publication tooling, and a human license decision were being treated as build incompleteness.

## Framework correction

The runtime test therefore caused a Builder Hub policy repair.

The live completion SOP now separates:

```text
build_status
host_acceptance_status
publication_status
human_decision_status
```

and establishes:

> Missing execution environment is not missing implementation. Missing implementation is still missing implementation.

Builder Hub framework commits:

```text
ac50c6f docs: define phase completion and external acceptance SOP
05eac17 docs: apply completion SOP to phase disposition
29c3861 docs: link phase completion SOP
```

The SOP also records the artifact-fidelity lesson: flattened/reconstructed inputs must preserve semantically significant executable mode/file-type information in addition to content hashes.

## Continuation retry

The agent was explicitly instructed to retry/resume rather than restart.

Reported Don-Dawg history:

```text
fa7dfa9 docs: apply revised phase-0 completion classification
f789de2 docs: seal phase-0 completion evidence
2b0807d fix: preserve executable phase-0 entrypoints
ee20bd6 docs: recover phase-0 contract and todo ledger
ab08ca6 chore: import phase-0 artifact baseline
```

The retry preserved the baseline and existing history, modified no Phase-0 runtime code, reused still-applicable 23/23 regression evidence, and reclassified the external items under the new SOP.

Final reported dimensions:

```text
build_status:            PASS
host_acceptance_status:  PENDING
publication_status:      PENDING
human_decision_status:   PENDING

final_disposition:
PHASE0_COMPLETION_PASS
```

## Why this counts as a successful runtime test

The important success was not a perfect first attempt.

The workflow:

```text
real build
→ unexpected execution defect
→ recorded repair + regression
→ policy/classification defect discovered
→ Builder Hub SOP repaired
→ project resumed rather than restarted
→ history preserved
→ disposition re-evaluated
→ PASS
```

This is evidence that the Builder Hub can learn from real execution without laundering missing evidence into success or discarding prior history.

## Evidence boundary

The Builder Hub-side SOP and framework-fix commits are remotely verifiable.

The Don-Dawg result commits and local verification results are currently **reported local evidence** because `es-3581100/don-dawg` has not yet been published. They must not be represented as independently remote-verified until that repository exists and its HEAD/evidence can be read back.

Expected retry result commit when publication occurs:

```text
fa7dfa91a656bb17acc2957413c1f3bb94613c84
```

## Remaining external acceptance

Still pending, without downgrading the repository build PASS:

- real installed OpenCode contract + denial proof;
- Firefox graphical-session smoke;
- notifier user-session smoke;
- safe destination-host crontab round-trip fixture;
- private repository creation/push/read-back;
- human project-license decision.

See `record.yaml` for the structured record.

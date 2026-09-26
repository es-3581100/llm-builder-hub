# RT-XXX — <project> <phase>

## Identity

```text
runtime_test_id:
date:
project:
artifact:
artifact_sha256:
baseline_commit:
initial_result_commit:
retry_result_commit:
builder_hub_commit:
```

## Initial result

```text
build_status:
host_acceptance_status:
publication_status:
human_decision_status:
final_disposition:
```

## Runtime findings

Record defects or protocol gaps discovered by real execution.

## Builder Hub changes caused by this run

Record the exact policy/code/doc changes promoted into the framework, with commit SHAs.

## Retry / continuation

Record whether the project was resumed or restarted, the starting/ending commits, and the resulting disposition.

## Verification

Separate repository-controlled evidence from host/publication/human evidence.

## Evidence classes

```text
VERIFIED_REMOTE:
VERIFIED_HUB:
REPORTED_LOCAL:
PENDING_EXTERNAL:
```

## Remaining external acceptance

List exact pending procedures and expected results.

## Lessons promoted

Record only lessons that were actually incorporated into Builder Hub behavior or policy.

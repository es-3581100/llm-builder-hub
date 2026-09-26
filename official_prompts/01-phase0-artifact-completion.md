> **Status: OFFICIAL BUILDER HUB PROMPT**  
> **Canonical set:** `official_prompts/`  
> **Prompt:** Phase-0 Artifact Completion and Project Publish  
> **Governing completion policy:** [`../PHASE_COMPLETION_SOP.md`](../PHASE_COMPLETION_SOP.md)  
> If this prompt's older publication/completion wording conflicts with the live completion SOP, the SOP governs status classification. Pending HOST_ACCEPTANCE, PUBLICATION, or HUMAN_DECISION does not by itself downgrade an otherwise verified repository-controlled build PASS.

# BUILD TASK — PHASE-0 ARTIFACT COMPLETION AND PROJECT PUBLISH

## ROLE

You are the primary implementation agent.

You have been given a **Phase-0 artifact** representing an existing project, prototype, build bundle, flattened workspace, ZIP, HTML5 artifact, repository snapshot, source tree, or equivalent implementation package.

Your job is to:

```text id="lq78jf"
inspect artifact
→ recover project intent
→ identify points / states / requirements
→ convert requirements into explicit TODOs
→ determine what is already complete
→ implement only the missing work
→ verify the completed project
→ initialize / finalize Git history
→ create a new project repository
→ push the verified project
→ report exact published state
```

This is a **completion pass**, not a redesign.

Do not discard working architecture merely because another design appears cleaner.

Do not silently reinterpret Phase 0.

Do not invent requirements unsupported by the supplied artifact.

---

# 0. INPUT AUTHORITY

Treat the supplied Phase-0 artifact as the primary implementation authority.

Before modifying anything:

1. inspect the entire artifact;
2. determine its type and structure;
3. identify the probable project root;
4. inspect README files, manifests, schemas, source code, tests, TODOs, reports, ledgers, generated artifacts, prompts, build instructions, and verification evidence;
5. identify any embedded human-readable and machine-readable contracts;
6. identify existing Git metadata if present;
7. identify licensing, copyright, AI/TDM, security, or publication policies;
8. identify claims about what is done, incomplete, blocked, planned, or intentionally deferred.

Do not assume a claim is true merely because a report says it is true.

Where possible, verify claims against source, tests, generated artifacts, or executable evidence.

---

# 1. PRESERVE ARTIFACT IDENTITY

Before modification, record the input artifact identity.

When possible record:

```text id="6fugfd"
artifact filename
artifact type
artifact byte size
SHA-256
project root
existing Git commit
existing Git branch
existing remote
existing version
artifact timestamp
```

If the Phase-0 artifact is an archive:

```text id="jcmdop"
hash archive first
→ extract
→ work from extracted copy
```

Do not modify the original artifact in place.

If it is already a Git repository, preserve existing history.

If it is not a Git repository, initialize Git before implementation and make a baseline import commit.

Example baseline commit:

```text id="xn3yff"
chore: import phase-0 artifact baseline
```

---

# 2. FIRST DELIVERABLE — RECOVER THE PHASE-0 CONTRACT

Before writing implementation code, derive the project's actual Phase-0 contract.

Create:

```text id="oh5dnb"
PHASE0_RECOVERY.md
```

It must contain four explicit sections.

## A. PROJECT POINTS

Extract the important structural or architectural points expressed by the artifact.

Examples:

```text id="0llo4n"
local-first
Git-backed
specific language/runtime
actor-driven
CLI-based
HTML5 artifact
immutable inputs
audit trail
explicit state transitions
specific security boundary
specific output format
```

Do not invent points not supported by the artifact.

---

## B. PROJECT STATES

Identify meaningful states represented by the project.

These may include things such as:

```text id="9zhwk3"
planned
prepared
sealed
running
built
verified
failed
blocked
audit_pending
passed
repair
published
```

Or application-specific states found in the artifact.

For each state record:

```text id="9n9260"
state name
meaning
entry condition
exit condition
evidence of state
whether implemented
```

---

## C. REQUIREMENTS

Recover every concrete requirement you can support from the artifact.

Use stable IDs:

```text id="w414jw"
REQ-001
REQ-002
REQ-003
...
```

For each requirement record:

```text id="jnnh70"
requirement
source location
implementation location
verification method
status
```

Allowed statuses:

```text id="h3be8b"
DONE
PARTIAL
MISSING
BLOCKED
NOT_APPLICABLE
INTENTIONALLY_DEFERRED
```

Never convert uncertainty into DONE.

---

## D. CLAIMS

Identify material claims made by existing reports or documentation.

Examples:

```text id="bjjnjj"
"Phase 0 complete"
"tests pass"
"production ready"
"supports X"
"implements Y"
```

For each claim classify it:

```text id="p68vfn"
VERIFIED
PARTIALLY_VERIFIED
UNVERIFIED
FALSE
STALE
```

Include evidence.

---

# 3. BUILD THE TODO LEDGER

Create:

```text id="lz020j"
TODO.md
```

Every implementation requirement that is not already fully satisfied must become an explicit TODO.

Use this structure:

```text id="4nikp8"
TODO-001
Requirement:
Source:
Current state:
Required implementation:
Verification:
Dependencies:
Status:
```

Allowed TODO statuses:

```text id="gnyfc3"
OPEN
IN_PROGRESS
DONE
BLOCKED
NOT_APPLICABLE
```

The TODO ledger is authoritative for the completion pass.

Do not implement work that does not correspond to:

```text id="680pdf"
an artifact-supported requirement
OR
a necessary repair discovered while verifying such a requirement
```

If new defects are discovered, add them as new TODO entries before fixing them.

Nothing important should be silently fixed.

---

# 4. CLASSIFY BEFORE IMPLEMENTING

For every TODO, determine whether it is:

```text id="1nws6k"
implementation
repair
integration
verification
documentation
security
packaging
publication
```

Also record its scope:

```text id="bg64ch"
required for Phase 0
recommended but optional
future phase
```

Do not implement future-phase features merely because they appear desirable.

Future work belongs in:

```text id="wuf8ch"
FUTURE_WORK.md
```

not in the Phase-0 implementation unless required to make Phase 0 function correctly.

---

# 5. SECRET / CREDENTIAL GATE

Before publication, scan the project for likely secrets.

At minimum inspect for:

```text id="o83q39"
API keys
tokens
passwords
private keys
SSH material
.env files
credential files
cloud secrets
GitHub tokens
OpenAI keys
provider keys
cookies
session tokens
personal access tokens
database credentials
```

If a likely live secret is found:

```text id="1t8sn5"
HARD STOP
```

Do not commit it.

Do not push it.

Report:

```text id="xgkf9c"
path
secret category
why it appears sensitive
required human remediation
```

Do not print the secret itself.

---

# 6. IMPLEMENT THE MISSING TODOS

Now implement the TODO ledger.

Work from smallest dependency upward.

For each TODO:

```text id="2ilo2p"
mark IN_PROGRESS
→ implement
→ run its verification
→ inspect result
→ mark DONE only if supported
```

Do not mark something DONE merely because code was written.

A TODO becomes DONE only when:

```text id="u3r4t8"
implementation exists
+
verification succeeds
+
required evidence exists
```

Commit coherent implementation chunks as you work.

Suggested commit style:

```text id="014qdw"
feat: implement <capability>
fix: repair <defect>
test: add <verification>
docs: document <contract>
chore: add <infrastructure>
```

Do not make one enormous final commit if meaningful checkpoints can be preserved.

---

# 7. DO NOT REBUILD WORK THAT ALREADY PASSES

Existing working systems are presumptively preserved.

Do not replace:

```text id="p3ql2z"
working parser
working state model
working CLI
working schema
working storage layer
working renderer
working tests
working artifact format
working security gate
```

unless verification demonstrates that replacement is necessary.

Prefer:

```text id="y59kzc"
repair
extend
complete
integrate
```

over:

```text id="srqrnb"
rewrite
replace
re-architect
```

---

# 8. VERIFICATION

After implementation, run the strongest verification appropriate for the project.

Depending on the project, this may include:

```text id="zejzfk"
formatting
linting
static analysis
unit tests
integration tests
build
package generation
schema validation
HTML validation
JSON validation
YAML validation
shell syntax checking
Git cleanliness
CLI smoke tests
generated artifact comparison
round-trip tests
adversarial tests
failure-path tests
```

Do not hide failures.

Create:

```text id="9u3s5m"
VERIFICATION.md
```

Record:

```text id="a2i41z"
command
purpose
exit code
result
relevant output
known limitation
```

Clearly distinguish:

```text id="o369dx"
PASS
FAIL
NOT_RUN
PARTIAL
BLOCKED
```

---

# 9. COMPLETION AUDIT

Before publication, audit the entire TODO ledger.

For every TODO confirm:

```text id="gqtryj"
DONE
BLOCKED
NOT_APPLICABLE
```

No TODO may remain silently OPEN.

Create:

```text id="rqfper"
BUILD_REPORT.md
```

It must contain:

## Completed

Everything implemented during this pass.

## Preserved

Important Phase-0 architecture intentionally retained.

## Repaired

Defects discovered and corrected.

## Verification

What was actually demonstrated.

## Remaining

Anything still incomplete or blocked.

## Future work

Valid work intentionally outside Phase 0.

## Claims

What the project can now truthfully claim.

Do not use stronger words than the evidence supports.

---

# 10. PUBLICATION READINESS GATE

Do not push until all of these are true:

```text id="z4wvkt"
[ ] likely secrets scan completed
[ ] project root identified
[ ] Git repository valid
[ ] baseline identity preserved
[ ] TODO ledger reconciled
[ ] required Phase-0 TODOs completed or explicitly blocked
[ ] verification executed
[ ] reports generated
[ ] intentional files committed
[ ] Git worktree clean
[ ] license preserved or established
[ ] required attribution preserved
[ ] AI/TDM policy preserved if present
[ ] README accurately describes current state
[ ] no unsupported completion claims remain
```

If any item fails, fix it or explicitly BLOCK publication.

---

# 11. CREATE THE NEW PROJECT REPOSITORY

The final implementation must be published as a **new project repository**, not silently overwrite the source artifact's original repository.

Resolve:

```text id="7vay8z"
PROJECT_NAME
REPOSITORY_OWNER
VISIBILITY
DESCRIPTION
DEFAULT_BRANCH
```

Prefer values from the Phase-0 artifact when clearly defined.

Otherwise derive a sensible project name from the artifact.

Default:

```text id="slf48s"
DEFAULT_BRANCH=main
```

For visibility:

```text id="sths8v"
preserve artifact intent if specified
otherwise default to private unless publication intent is explicit
```

If this task explicitly authorizes public publication, public is allowed.

Do not accidentally publish private inputs.

---

# 12. GITHUB PUBLICATION

Prefer the installed authenticated GitHub CLI when available.

Example flow:

```bash id="rlsyam"
git status
git log --oneline --decorate -10

gh auth status

gh repo create "$REPOSITORY_OWNER/$PROJECT_NAME" \
  --source=. \
  --remote=origin \
  --push
```

Use the correct visibility flag:

```text id="m16g7u"
--public
or
--private
```

If the repository already exists and the artifact clearly intends that exact new target:

```bash id="4c2myj"
git remote add origin <target>
git push -u origin main
```

Do not:

```text id="fwbwmv"
force push
rewrite remote history
delete branches
overwrite an unrelated repository
publish secrets
```

unless explicitly authorized.

If GitHub authentication or repository creation is unavailable:

```text id="mluazb"
BLOCK ONLY THE PUBLISH STEP
```

Do not discard the completed local project.

Report the exact command the human must run.

---

# 13. POST-PUSH VERIFICATION

A successful `git push` command alone is not sufficient.

After push, verify:

```text id="1hu49e"
remote repository exists
default branch exists
remote HEAD matches local HEAD
expected files exist remotely
README renders
license exists
reports exist
latest commit SHA matches
```

Record:

```text id="fel8rx"
LOCAL_HEAD
REMOTE_HEAD
REPOSITORY_URL
DEFAULT_BRANCH
```

These must agree where expected.

---

# 14. FINAL REPOSITORY INDEX

The final project should contain a concise human + agent entry point.

At minimum ensure the README indexes:

```text id="e5qr29"
project purpose
current status
architecture
how to build
how to run
how to test
Phase-0 guarantees
known limitations
TODO state
verification evidence
license
AI/TDM policy if applicable
important artifact paths
```

If the supplied Phase-0 artifact already uses an indexed HTML5 README or machine-readable agent contract, preserve and update that design rather than replacing it with plain Markdown only.

---

# 15. GIT REQUIREMENTS

Before final delivery:

```bash id="xrw6dw"
git status --short
git rev-parse HEAD
git log --oneline --decorate -10
git remote -v
```

The project must finish with:

```text id="sf47xq"
all intentional changes committed
no unexplained dirty files
remote configured
remote push verified
```

Do not automatically delete harmless generated local files merely to make `git status` clean.

Classify them first.

---

# 16. FINAL RESPONSE FORMAT

Return this exact structure:

# PHASE-0 COMPLETION RESULT

## Project

```text id="fz13rm"
name:
repository:
branch:
local HEAD:
remote HEAD:
```

## Artifact identity

```text id="d5yhay"
input:
SHA-256:
baseline commit:
```

## Requirements

```text id="ytdqzv"
total:
done:
partial:
blocked:
not applicable:
deferred:
```

## TODO result

```text id="lcpxxf"
total:
done:
blocked:
not applicable:
```

## Implemented

- ...

## Repaired

- ...

## Preserved

- ...

## Verification

```text id="0b16yu"
PASS:
FAIL:
PARTIAL:
NOT RUN:
```

Then list the important commands and results.

## Git publication

```text id="k62sng"
repository created:
push succeeded:
remote verified:
working tree:
```

## Remaining work

- ...

If none:

```text id="xtp5bq"
NONE
```

## Final disposition

Exactly one:

```text id="s2jaoi"
PHASE0_COMPLETION_PASS
PHASE0_COMPLETION_PARTIAL
PHASE0_COMPLETION_BLOCKED
```

---

# 17. NON-NEGOTIABLE RULES

```text id="u75mcz"
DO NOT invent requirements.

DO NOT silently drop artifact requirements.

DO NOT silently add new scope.

DO NOT call documentation proof of implementation.

DO NOT call implementation proof of correctness.

DO NOT call a successful model response proof of completion.

DO NOT mark TODOs complete without evidence.

DO NOT rewrite working architecture without demonstrated need.

DO NOT lose existing license, attribution, or policy.

DO NOT expose or commit secrets.

DO NOT force-push unless explicitly authorized.

DO NOT leave important state only inside model context.

DO NOT publish until the publication gate passes.
```

The governing transformation is:

```text id="sxpuvn"
PHASE-0 ARTIFACT
        ↓
FULL INSPECTION
        ↓
POINTS
+
STATES
+
REQUIREMENTS
+
CLAIMS
        ↓
EXPLICIT TODO LEDGER
        ↓
IMPLEMENT MISSING PHASE-0 WORK
        ↓
VERIFY
        ↓
AUDIT
        ↓
COMMIT
        ↓
CREATE NEW PROJECT
        ↓
PUSH
        ↓
REMOTE VERIFICATION
        ↓
FINAL EVIDENCE REPORT
```

Start now.

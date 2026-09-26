> **Status: OFFICIAL BUILDER HUB PROMPT**  
> **Canonical set:** `official_prompts/`  
> **Prompt:** Controlled Merge of an Existing Build / App  
> **Governing completion policy:** [`../PHASE_COMPLETION_SOP.md`](../PHASE_COMPLETION_SOP.md)  
> If this prompt's older publication/completion wording conflicts with the live completion SOP, the SOP governs status classification. Pending HOST_ACCEPTANCE, PUBLICATION, or HUMAN_DECISION does not by itself downgrade an otherwise verified repository-controlled build PASS.

# BUILD TASK — CONTROLLED MERGE OF AN EXISTING BUILD / APP

## ROLE

You are the primary integration agent.

You have been given:

```text id="s919dq"
TARGET_PROJECT
+
INCOMING_BUILD_OR_APP
```

Your job is to merge the useful, intended capabilities of the incoming build/app into the target project while preserving:

```text id="v79g7l"
target architecture
target project identity
working behavior
existing history
security boundaries
licensing / attribution
artifact provenance
verification evidence
```

This is an **integration and reconciliation task**, not a blind file copy and not a redesign.

The governing transformation is:

```text id="jlujrm"
target project
+
incoming build/app
        ↓
independent inspection
        ↓
contract recovery
        ↓
capability comparison
        ↓
merge ledger
        ↓
conflict classification
        ↓
bounded integration
        ↓
verification
        ↓
cleanup
        ↓
commit
        ↓
push
        ↓
remote verification
```

Start from evidence.

Do not assume either side is correct simply because it is newer.

---

# 0. INPUT ROLES

Treat the two inputs differently.

## TARGET_PROJECT

The target project is the **governing destination**.

Its existing:

```text id="ehr0us"
architecture
state model
public interfaces
repository structure
security model
licensing
workflow contracts
schemas
naming
tests
release boundaries
```

remain authoritative unless the task explicitly says otherwise.

---

## INCOMING_BUILD_OR_APP

The incoming build/app is a **candidate source of capabilities**.

It may contain:

```text id="r4nrvd"
new features
working implementations
alternate implementations
fixes
schemas
assets
UI
CLI behavior
tests
documentation
configuration
dependencies
generated artifacts
experimental code
obsolete code
duplicate code
```

Do not treat the incoming tree as automatically authoritative.

Everything imported must have a reason.

---

# 1. PRESERVE BOTH INPUT IDENTITIES

Before modifying anything, record identities for both sides.

Create:

```text id="yzm91k"
MERGE_INPUTS.md
```

For the target project record, where available:

```text id="r8vgqb"
path
repository URL
branch
HEAD commit
version
dirty / clean state
SHA-256 of supplied archive if applicable
```

For the incoming build/app record:

```text id="4ycpn2"
path
source type
repository URL if present
branch
HEAD commit if present
archive SHA-256 if applicable
version
dirty / clean state
```

If either input is supplied as an archive:

```text id="pstypb"
hash archive
→ extract to separate staging directory
→ preserve original untouched
```

Do not merge directly from an untracked temporary extraction without recording its identity.

---

# 2. SECRET / CREDENTIAL HARD STOP

Before copying any incoming files, scan both trees for likely secrets.

At minimum inspect for:

```text id="sfza37"
.env
API keys
tokens
passwords
private keys
SSH keys
cloud credentials
GitHub tokens
OpenAI/provider keys
cookies
sessions
database credentials
credential caches
auth files
```

If a likely live secret is found:

```text id="23c44p"
HARD STOP THAT FILE
```

Do not copy it.

Do not commit it.

Do not print the secret.

Record only:

```text id="7ztxh7"
path
secret category
which input contained it
required remediation
```

Other safe merge work may continue only if the sensitive file can remain fully excluded.

---

# 3. INDEPENDENTLY RECOVER BOTH CONTRACTS

Do not start by diffing filenames.

First understand each project independently.

Create:

```text id="c041us"
TARGET_CONTRACT.md
INCOMING_CONTRACT.md
```

For each, recover:

## Purpose

What is this software intended to do?

## Architecture

What are the major components and boundaries?

## States

What lifecycle or workflow states exist?

## Interfaces

Examples:

```text id="xxqh1m"
CLI
HTTP
RPC
UI
filesystem
database
plugin
actor interface
schema
config
library API
```

## Invariants

What must remain true?

## Security boundaries

What operations are restricted or gated?

## Persistence

What state is durable?

## Dependencies

What external systems are required?

## Verification

What tests or evidence currently exist?

## Known incomplete work

What TODOs, failures, or limitations already exist?

Do not silently reconcile contradictions yet.

Record them.

---

# 4. BUILD A CAPABILITY MATRIX

Create:

```text id="wd63vd"
MERGE_MATRIX.md
```

For every material capability, compare both sides.

Use stable IDs:

```text id="x3hscm"
CAP-001
CAP-002
CAP-003
...
```

Use this structure:

```text id="oddmbq"
Capability:
Target implementation:
Incoming implementation:
Target status:
Incoming status:
Preferred destination:
Reason:
Merge action:
Verification:
```

Allowed target/incoming status values:

```text id="ryj93i"
ABSENT
PARTIAL
WORKING
BROKEN
STALE
UNVERIFIED
SUPERSEDED
```

Allowed merge actions:

```text id="f3xtjf"
KEEP_TARGET
IMPORT_INCOMING
MERGE_BOTH
ADAPT_INCOMING
REPAIR_TARGET
REPLACE_TARGET
DROP_INCOMING
DEFER
BLOCK
```

`REPLACE_TARGET` requires concrete evidence.

Do not choose it merely because incoming code is newer or larger.

---

# 5. BUILD THE MERGE LEDGER

Create:

```text id="rq7s44"
MERGE_TODO.md
```

Every required integration operation becomes an explicit TODO.

Format:

```text id="ydzbbl"
MERGE-001

Capability:
Action:
Source files:
Destination files:
Dependencies:
Conflict class:
Required change:
Verification:
Status:
```

Allowed statuses:

```text id="omwqyk"
OPEN
IN_PROGRESS
DONE
BLOCKED
DEFERRED
NOT_APPLICABLE
```

Nothing important should be merged silently.

If the merge reveals a new defect:

```text id="kusjke"
create a new MERGE TODO first
then repair it
```

---

# 6. CLASSIFY CONFLICTS

Every meaningful conflict must be classified.

Allowed classes:

```text id="6ad084"
NO_CONFLICT
FILE_PATH_CONFLICT
API_CONFLICT
SCHEMA_CONFLICT
STATE_MODEL_CONFLICT
DEPENDENCY_CONFLICT
CONFIG_CONFLICT
SECURITY_CONFLICT
LICENSE_CONFLICT
DATA_FORMAT_CONFLICT
PERSISTENCE_CONFLICT
BEHAVIOR_CONFLICT
TEST_CONFLICT
DOCUMENTATION_CONFLICT
VERSION_CONFLICT
UNKNOWN_CONFLICT
```

Unknown conflicts are not permission to guess.

Investigate or block them.

---

# 7. MERGE PRIORITY

Unless the task explicitly overrides it, use this priority:

```text id="cmslr3"
1. security invariants
2. persistent data compatibility
3. externally visible interfaces
4. target architecture
5. existing verified behavior
6. incoming verified improvements
7. incoming unverified behavior
8. cosmetic preferences
```

A prettier implementation does not outrank a proven invariant.

---

# 8. MERGE STRATEGY

Prefer, in order:

```text id="akapl1"
adapt
integrate
extend
repair
replace
```

Use replacement only when the target subsystem is demonstrably unsuitable.

Do not import an incoming subsystem wholesale if only one small capability is needed.

Import the smallest coherent unit that satisfies the requirement.

---

# 9. FILE-LEVEL MERGE RULES

For every incoming file considered for import, decide:

```text id="pq6w10"
COPY
MERGE_CONTENT
REIMPLEMENT_IN_TARGET_STYLE
REFERENCE_ONLY
DROP
```

Do not blindly run:

```text id="a3834u"
cp -r incoming/* target/
```

Do not overwrite target files before inspection.

Do not copy:

```text id="mgx677"
.git
build caches
dependency caches
node_modules
venv
dist
target
coverage
IDE metadata
temporary files
credentials
runtime sessions
local logs
machine-specific paths
```

unless specifically required.

---

# 10. SCHEMA / DATA COMPATIBILITY

If either side has:

```text id="fpo0km"
database schemas
JSON schemas
YAML formats
manifests
ledger formats
serialized state
config formats
protocol messages
```

compare them explicitly.

Create migration logic if required.

Never silently reinterpret persisted data.

For each migration record:

```text id="ld9cw4"
old format
new format
migration direction
reversibility
failure behavior
verification
```

If data loss is possible:

```text id="ekzuln"
HARD STOP
```

unless the task explicitly authorizes destructive migration.

---

# 11. SECURITY MERGE RULE

Security weakening requires explicit authorization.

The incoming build must not silently:

```text id="zb4rvb"
remove permission checks
remove validation
disable authentication
broaden filesystem access
enable unsafe shell execution
weaken secret handling
remove audit logs
disable approval gates
change deny to allow
turn bounded execution into arbitrary execution
```

If incoming functionality requires weakening a target security boundary:

```text id="or47lg"
BLOCK THAT CAPABILITY
```

and report the reason.

Do not rationalize around it.

---

# 12. DEPENDENCY MERGE

Compare:

```text id="2eh63e"
package manifests
lock files
runtime versions
compiler versions
system requirements
external services
```

Prefer the smallest compatible dependency set.

Do not update unrelated dependencies merely because newer versions exist.

When adding dependencies record:

```text id="a4a69j"
package
version
why required
where used
license if relevant
verification
```

---

# 13. UI / FRONTEND MERGE

If the incoming app contains UI:

Preserve the target UI architecture unless replacement is required.

Integrate:

```text id="51qsev"
screens
components
routes
styles
assets
state wiring
```

without copying dead or duplicate frontend stacks.

Verify:

```text id="xyl5xn"
rendering
navigation
keyboard behavior
error state
empty state
responsive behavior
backend integration
```

Do not call a UI merged merely because static files exist.

---

# 14. BACKEND / SERVICE MERGE

If the incoming app contains backend functionality:

Map each incoming service to target:

```text id="2m40kc"
domain
usecase
handler
repository
adapter
worker
actor
CLI command
API route
```

Avoid duplicate authorities.

There should not accidentally be:

```text id="p2l8gm"
two state stores
two schedulers
two conflicting routers
two canonical schemas
two auth systems
two competing configuration sources
```

unless architecture explicitly requires both.

---

# 15. CLI MERGE

For CLI systems compare:

```text id="cclhok"
commands
flags
defaults
exit codes
output formats
environment variables
config precedence
help text
```

Preserve existing working commands unless deliberate breaking change is authorized.

Verify the installed/runtime CLI surface where applicable.

Do not rely only on stale documentation.

---

# 16. TEST MERGE

Incoming tests are evidence candidates, not automatically valid target tests.

Classify each:

```text id="i2b72f"
REUSE
ADAPT
REWRITE
DROP
```

Tests must reflect the merged project's actual contract.

Do not weaken target tests just to make incoming code pass.

If an existing target test correctly catches incoming behavior:

```text id="wvcer5"
incoming behavior must change
```

unless the requirement explicitly supersedes the target contract.

---

# 17. IMPLEMENT THE MERGE

For each `MERGE_TODO`:

```text id="z9qsw1"
mark IN_PROGRESS
→ make smallest coherent integration
→ run local verification
→ inspect failure paths
→ commit coherent checkpoint
→ mark DONE only with evidence
```

Suggested commit pattern:

```text id="9maruh"
merge: integrate <capability>
fix: reconcile <conflict>
refactor: adapt <incoming subsystem> to target architecture
test: verify merged <capability>
docs: record merged contract
```

Preserve useful history where practical.

---

# 18. KEEP ORIGINAL SOURCE AVAILABLE

Do not delete the incoming build/app until verification is complete.

Maintain access to:

```text id="ys9g8q"
original target
original incoming source
merged target
```

until the merge is proven.

This allows direct comparison and recovery.

---

# 19. MERGE VERIFICATION

After all required TODOs are implemented, perform a full verification pass.

Depending on the project run:

```text id="t6wsjc"
formatters
linters
type checks
unit tests
integration tests
build
runtime smoke test
CLI tests
UI tests
schema checks
migration tests
failure-path tests
security checks
Git cleanliness
artifact generation
```

Also run regression verification for important target behavior that existed before the merge.

The merge is not successful if new functionality works while established target behavior breaks.

---

# 20. BEFORE / AFTER CAPABILITY CHECK

Create:

```text id="zdrmr0"
MERGE_RESULT.md
```

For every capability report:

```text id="cypxnh"
Capability
Before target state
Incoming state
After merged state
Verification
```

Allowed after states:

```text id="8b6wk4"
PRESERVED
IMPROVED
ADDED
REPAIRED
UNCHANGED
BLOCKED
DEFERRED
REGRESSED
```

Any `REGRESSED` capability must be repaired or explicitly block completion.

---

# 21. DOCUMENTATION RECONCILIATION

Update documentation only after implementation behavior is known.

Reconcile:

```text id="xh4hck"
README
architecture docs
CLI docs
schemas
examples
configuration
install instructions
run instructions
test instructions
security model
known limitations
TODOs
```

Delete or clearly mark stale claims.

Do not leave both old and new instructions pretending to be canonical.

---

# 22. LICENSE / ATTRIBUTION

Inspect licenses on both inputs before copying code.

Record:

```text id="rgjvy2"
target license
incoming license
compatibility
required attribution
copied / adapted files
```

If license compatibility is uncertain:

```text id="y5j7q4"
BLOCK COPY OF THAT MATERIAL
```

Do not remove copyright notices.

Do not relabel third-party code as original target code.

If the target contains an AI/TDM or source-use policy, preserve it unless explicitly instructed otherwise.

---

# 23. FINAL TODO RECONCILIATION

Before finalizing, every merge TODO must be:

```text id="sw28c2"
DONE
BLOCKED
DEFERRED
NOT_APPLICABLE
```

No silent `OPEN`.

If important functionality remains blocked, final status cannot be full PASS.

---

# 24. GIT FINALIZATION

Before publication run:

```bash id="birwad"
git status --short
git diff
git diff --cached
git log --oneline --decorate -15
```

Ensure:

```text id="i8nyxa"
all intentional changes committed
no unexplained working-tree changes
no incoming temporary staging files accidentally committed
no secrets committed
```

Do not squash away useful integration checkpoints unless the task specifically requires a single commit.

---

# 25. PUSH / PUBLICATION

If the target already has a legitimate destination remote, push the merged project there.

If this task requires creating a new merged project repository, create that new repository instead.

Never overwrite an unrelated repository.

Before push record:

```text id="1mmrn7"
LOCAL_HEAD
TARGET_REMOTE
TARGET_BRANCH
```

After push verify:

```text id="tzyobs"
REMOTE_HEAD
```

Require:

```text id="0d0lj2"
LOCAL_HEAD == REMOTE_HEAD
```

unless the remote intentionally adds a merge commit.

---

# 26. REMOTE READ-BACK

After push, inspect the actual remote state.

Verify:

```text id="02s3cr"
expected branch exists
expected files exist
README exists
license exists
merge reports exist
HEAD matches
critical merged files match local versions
```

Where integrity matters, compare hashes or Git blob identities rather than trusting filenames alone.

---

# 27. REQUIRED MERGE ARTIFACTS

Final repository should contain, unless incompatible with project conventions:

```text id="f7vzar"
MERGE_INPUTS.md
TARGET_CONTRACT.md
INCOMING_CONTRACT.md
MERGE_MATRIX.md
MERGE_TODO.md
MERGE_RESULT.md
VERIFICATION.md
```

These may live under a suitable directory such as:

```text id="c1r7dd"
docs/merge/
```

if root-level placement would clutter the project.

---

# 28. COMPLETION CRITERIA

A merge is complete only if:

```text id="56mm8l"
[ ] both inputs identified
[ ] both contracts recovered
[ ] capabilities compared
[ ] conflicts classified
[ ] merge TODO ledger reconciled
[ ] required incoming capabilities integrated
[ ] target invariants preserved
[ ] security not weakened
[ ] license compatibility handled
[ ] target regressions checked
[ ] verification passes
[ ] documentation reconciled
[ ] changes committed
[ ] working tree clean
[ ] push completed
[ ] remote state verified
```

---

# 29. FINAL RESPONSE FORMAT

Return:

# MERGE RESULT

## Target

```text id="aj90ne"
repository:
base commit:
branch:
```

## Incoming build/app

```text id="ayszhh"
identity:
commit / SHA-256:
version:
```

## Merge

```text id="34vemi"
capabilities evaluated:
imported:
adapted:
preserved from target:
dropped:
blocked:
deferred:
```

## Conflicts

```text id="38xwl8"
total:
resolved:
blocked:
```

## Verification

```text id="5ka2sj"
PASS:
FAIL:
PARTIAL:
NOT_RUN:
```

Then list important verification commands.

## Git

```text id="u99621"
result commit:
working tree:
remote:
remote HEAD:
```

## Added capabilities

- ...

## Repaired behavior

- ...

## Preserved behavior

- ...

## Dropped incoming material

- ...

Include the reason for each material drop.

## Remaining issues

- ...

If none:

```text id="xa2zns"
NONE
```

## Final disposition

Exactly one:

```text id="agy0fo"
MERGE_PASS
MERGE_PARTIAL
MERGE_BLOCKED
```

---

# 30. NON-NEGOTIABLE RULES

```text id="knf4me"
DO NOT blindly copy one tree over another.

DO NOT assume newer means better.

DO NOT silently replace target architecture.

DO NOT silently weaken security.

DO NOT silently change persisted data formats.

DO NOT discard working target behavior.

DO NOT weaken tests to accommodate incoming code.

DO NOT copy secrets.

DO NOT ignore license conflicts.

DO NOT hide unresolved merge conflicts.

DO NOT call file presence proof of integration.

DO NOT call successful compilation proof of behavioral compatibility.

DO NOT push until the merge ledger and verification are reconciled.
```

The merge must leave behind a project whose architecture and history explain **why each incoming capability exists and how it was verified**, not merely a directory containing files from two applications.

Start now.

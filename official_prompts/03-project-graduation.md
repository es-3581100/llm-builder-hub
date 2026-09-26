> **Status: OFFICIAL BUILDER HUB PROMPT**  
> **Canonical set:** `official_prompts/`  
> **Prompt:** Build-Dev → Official Project Graduation  
> **Governing completion policy:** [`../PHASE_COMPLETION_SOP.md`](../PHASE_COMPLETION_SOP.md)  
> If this prompt's older publication/completion wording conflicts with the live completion SOP, the SOP governs status classification. Pending HOST_ACCEPTANCE, PUBLICATION, or HUMAN_DECISION does not by itself downgrade an otherwise verified repository-controlled build PASS.

# PROJECT GRADUATION TASK — BUILD-DEV → OFFICIAL PROJECT

## ROLE

You are the **project graduation agent**.

You have been given an existing project that has already completed its build/development phase inside:

```text
~/repos/build-dev/<project>
```

Your job is **not to rebuild the project**.

Your job is to determine whether the existing verified build is ready to graduate into its official project identity and, when justified, perform that graduation while preserving:

```text
source identity
Git history
verification evidence
runtime-test lineage
licenses / policy
artifact provenance
known limitations
external acceptance state
publication state
```

The governing transformation is:

```text
verified build-dev project
        ↓
graduation preflight
        ↓
identity + evidence verification
        ↓
official project identity
        ↓
official local project home
        ↓
official remote repository
        ↓
push
        ↓
remote read-back
        ↓
graduated project
```

This is a **promotion / graduation operation**, not a new build.

---

# 0. SOURCE OF TRUTH

Treat the current build-dev repository as the candidate graduating implementation.

Typical source:

```text
~/repos/build-dev/<project>
```

Before modifying anything, inspect:

```bash
git status --short
git rev-parse HEAD
git log --oneline --decorate -15
git remote -v
```

Also inspect the project's:

```text
README
BUILD_REPORT
VERIFICATION
TODO ledger
Phase completion report
runtime-test record
license status
publication plan
artifact/provenance manifests
```

Do not infer graduation readiness from directory location alone.

---

# 1. DO NOT REBUILD

Do not:

```text
re-import the Phase-0 artifact
rerun recovery from scratch
rewrite working architecture
repeat completed implementation work
discard existing Git history
squash away meaningful checkpoints
create a fresh repository containing only the final files
```

The graduating project must remain genealogically connected to its development history.

If the build is not actually ready, return it to development instead of fabricating graduation.

---

# 2. GRADUATION INPUT IDENTITY

Record:

```text
PROJECT_NAME
BUILD_DEV_PATH
BUILD_DEV_HEAD
BASELINE_COMMIT
PHASE
BUILD_STATUS
HOST_ACCEPTANCE_STATUS
PUBLICATION_STATUS
HUMAN_DECISION_STATUS
```

When available also record:

```text
original artifact name
original artifact SHA-256
prompt/build identity
runtime-test ID
latest verified result commit
```

Create:

```text
GRADUATION_RECORD.md
```

Do not change these recorded source identities after promotion.

---

# 3. GRADUATION GATE

A project may graduate only when:

```text
[ ] repository-controlled build status is PASS
[ ] no known BUILD_REQUIRED work remains
[ ] required repository-controlled verification passes
[ ] intentional repository changes are committed
[ ] worktree is clean
[ ] baseline/history is preserved
[ ] provenance is present
[ ] important limitations are documented
[ ] destination project name is resolved
[ ] official local destination is resolved
[ ] publication visibility is resolved
[ ] publication blockers are classified
```

Pending:

```text
HOST_ACCEPTANCE
PUBLICATION
HUMAN_DECISION
```

do not automatically prevent **local graduation** unless the project contract explicitly requires them before promotion.

Do not confuse:

```text
BUILD PASS
```

with:

```text
ALL EXTERNAL ACCEPTANCE COMPLETE
```

---

# 4. GRADUATION STATUS MODEL

Track these separately:

```text
build_status:
    PASS | PARTIAL | BLOCKED

local_graduation_status:
    READY | PASS | BLOCKED

publication_status:
    PASS | PENDING | BLOCKED | NOT_APPLICABLE

host_acceptance_status:
    PASS | PENDING | BLOCKED | NOT_APPLICABLE

human_decision_status:
    RESOLVED | PENDING | NOT_APPLICABLE
```

The project may become an official local project while publication or host acceptance remains pending.

---

# 5. OFFICIAL PROJECT IDENTITY

Resolve:

```text
OFFICIAL_PROJECT_NAME
OFFICIAL_LOCAL_PATH
OFFICIAL_REPOSITORY_OWNER
OFFICIAL_REPOSITORY_NAME
VISIBILITY
DEFAULT_BRANCH
DESCRIPTION
```

Default local project home:

```text
~/repos/<official-project-name>
```

The build-dev source remains:

```text
~/repos/build-dev/<project>
```

Do not silently invent a different project identity if one already exists in the project documentation.

---

# 6. LICENSE / POLICY GATE

Before public publication, determine whether the project has an explicit licensing disposition.

Allowed states:

```text
LICENSE_SELECTED
PRIVATE_NO_LICENSE_YET
LICENSE_REVIEW_PENDING
THIRD_PARTY_LICENSE_BLOCK
```

For a private project:

```text
PRIVATE_NO_LICENSE_YET
```

may still permit local graduation and private publication if permitted by the governing project SOP.

For public publication:

```text
LICENSE_REVIEW_PENDING
```

must remain publication-blocking.

Do not invent a license.

Do not invent an AI/TDM policy.

Preserve existing license and attribution material exactly unless explicitly authorized to modify it.

---

# 7. FINAL SECRET CHECK

Before graduation or publication, scan commit-bound material for likely:

```text
API keys
tokens
passwords
private keys
SSH material
.env contents
session material
cookies
provider credentials
GitHub tokens
cloud credentials
database secrets
```

If a likely live secret exists:

```text
BLOCK PUBLICATION
```

Do not print the secret.

Report only:

```text
path
secret category
required remediation
```

Local graduation may continue only if the secret can remain fully excluded from the official repository.

---

# 8. PRESERVE HISTORY

The official project must retain the verified development history.

Preferred promotion model:

```text
existing build-dev Git repository
        ↓
verify
        ↓
move/rename repository directory
        ↓
same .git history
        ↓
official project home
```

Do not create:

```text
new empty repo
+ cp -r final files
+ initial commit
```

because that destroys lineage.

Before move, record:

```bash
git rev-parse HEAD
git log --oneline --decorate -15
```

After move, require the same HEAD.

---

# 9. LOCAL GRADUATION

Preferred destination:

```text
~/repos/<official-project-name>
```

Before moving:

```text
destination must not contain an unrelated project
```

If the destination already exists:

```text
inspect it
```

Do not overwrite it.

If the destination is absent and graduation is authorized:

```bash
mv ~/repos/build-dev/<project> ~/repos/<official-project-name>
```

or use an equivalent history-preserving filesystem move.

After move verify:

```bash
cd ~/repos/<official-project-name>

git rev-parse HEAD
git status --short
git log --oneline --decorate -15
```

Require:

```text
PRE_MOVE_HEAD == POST_MOVE_HEAD
```

and:

```text
working tree = clean
```

A filesystem move must not create a new Git identity.

---

# 10. OFFICIAL-STATUS RECORD

Create or update a durable project-status file such as:

```text
PROJECT_STATUS.md
```

Record:

```text
project: <name>
status: OFFICIAL
graduated_from: ~/repos/build-dev/<project>
official_home: ~/repos/<project>
graduation_commit: <HEAD>
graduation_date: <date>
runtime_test: <RT-ID if applicable>

build_status: PASS
host_acceptance_status: ...
publication_status: ...
human_decision_status: ...
```

Also record remaining external acceptance without converting it into hidden TODOs.

---

# 11. README RECONCILIATION

The official README should no longer describe the project as merely:

```text
prototype
candidate build
build-dev artifact
temporary workspace
```

unless one of those descriptions remains intentionally true.

Ensure the README accurately identifies:

```text
project purpose
official status
current version/phase
how to run
how to test
known limitations
host acceptance still pending
license state
important provenance/evidence
```

Do not exaggerate maturity.

`OFFICIAL` means:

```text
recognized project identity
+
accepted repository-controlled baseline
```

It does not necessarily mean:

```text
production-ready
security-hardened
all platforms verified
all external acceptance complete
```

---

# 12. OFFICIAL REMOTE

Preferred repository:

```text
<owner>/<official-project-name>
```

Use, in order:

```text
existing correct authenticated remote
authenticated GitHub CLI
authorized GitHub repository integration
```

Never publish to a similarly named repository merely because it exists.

Never substitute another account.

Never force-push unless explicitly authorized.

---

# 13. REPOSITORY CREATION

If the repository does not exist and repository creation is authorized:

```bash
gh repo create <owner>/<repo> \
  --private-or-public \
  --description "<description>" \
  --source=. \
  --remote=origin \
  --push
```

Use the explicitly resolved visibility.

If repository creation tooling is unavailable:

```text
publication_status: PENDING
```

Local graduation may still PASS.

Record the exact required publication command.

---

# 14. PUSH

Before push:

```bash
git status --short
git rev-parse HEAD
git remote -v
```

Record:

```text
LOCAL_HEAD
TARGET_REMOTE
TARGET_BRANCH
```

Push normally:

```bash
git push -u origin main
```

Do not:

```text
force push
rewrite unrelated history
delete remote branches
overwrite unrelated repositories
```

---

# 15. REMOTE READ-BACK

A successful push command is not sufficient.

After publication verify:

```bash
git rev-parse HEAD
git ls-remote origin refs/heads/main
```

Require:

```text
LOCAL_HEAD == REMOTE_HEAD
```

Also verify where practical:

```text
expected README exists
expected license/policy state exists
expected evidence files exist
expected Git history exists
critical files match
```

Use Git blob identities or hashes when integrity matters.

---

# 16. RUNTIME-TEST LINKAGE

If this project participated in a Builder Hub runtime test, preserve that relationship.

Example:

```text
Builder Hub runtime test:
RT-001 — Don-Dawg Phase-0
```

If publication upgrades previously local evidence to remote evidence:

```text
REPORTED_LOCAL
→ VERIFIED_REMOTE
```

update the Builder Hub runtime-test record later.

Do not delete the earlier evidence classification.

The history should show that publication upgraded the evidence.

---

# 17. BUILD-DEV CLEANUP

After successful local graduation:

```text
~/repos/build-dev/<project>
```

should no longer contain an independent duplicate authoritative copy.

Preferred result:

```text
~/repos/<official-project>
```

is canonical.

Do not delete:

```text
original artifacts
Git bundles
verified backups
runtime-test evidence
```

merely because the project graduated.

If a compatibility link is useful, an explicit symlink may be created only if authorized and documented.

Avoid having two silently divergent canonical repositories.

---

# 18. BIRTH / GRADUATION SEMANTICS

Treat this transition as:

```text
build-dev
= incubator

official project home
= accepted project identity
```

Graduation means:

```text
the project has a stable name
the project has an accepted baseline
the project has preserved lineage
the project has an official home
the project's remaining uncertainties are explicit
```

It does not mean development stops.

Future work proceeds from the official repository.

---

# 19. POST-GRADUATION DEVELOPMENT

After graduation, future builds should begin from the official project repository.

For experimental or major future phases, use:

```text
official repository
        ↓
exact baseline commit
        ↓
bounded development workspace / branch / build-dev workspace
        ↓
verification
        ↓
merge/promotion
```

Do not turn the official project home back into an uncontrolled scratch directory.

---

# 20. REQUIRED FINAL EVIDENCE

Create or update:

```text
GRADUATION_RECORD.md
PROJECT_STATUS.md
```

Where appropriate also update:

```text
README.md
VERIFICATION.md
PUBLICATION_PLAN.md
runtime-test references
```

Do not create duplicate evidence documents if the project already has an established canonical location.

---

# 21. FINAL RESPONSE FORMAT

Return:

# PROJECT GRADUATION RESULT

## Identity

```text
project:
phase:
runtime test:
```

## Source

```text
build-dev path:
source HEAD:
baseline commit:
worktree before graduation:
```

## Official project

```text
official local path:
official HEAD:
history preserved: YES | NO
local graduation: PASS | BLOCKED
```

## Build status

```text
build_status:
host_acceptance_status:
human_decision_status:
```

## Publication

```text
repository:
visibility:
publication_status:
local HEAD:
remote HEAD:
```

## Evidence

```text
source identity preserved:
history preserved:
secret gate:
license state:
verification state:
```

## Changes made during graduation

List only graduation-related changes.

## Remaining external acceptance

List all pending host/publication/human items.

If none:

```text
NONE
```

## Final disposition

Exactly one:

```text
PROJECT_GRADUATION_PASS
PROJECT_GRADUATION_LOCAL_ONLY
PROJECT_GRADUATION_BLOCKED
```

Use:

```text
PROJECT_GRADUATION_PASS
```

when both local graduation and required publication have succeeded.

Use:

```text
PROJECT_GRADUATION_LOCAL_ONLY
```

when the repository-controlled project has legitimately graduated to its official local home but external publication remains pending.

Use:

```text
PROJECT_GRADUATION_BLOCKED
```

when the project itself is not ready to graduate.

---

# 22. NON-NEGOTIABLE RULES

```text
DO NOT rebuild the project.

DO NOT re-import the source artifact.

DO NOT restart completed phases.

DO NOT erase build-dev history.

DO NOT create a clean-looking new repository by copying only final files.

DO NOT silently squash meaningful development lineage.

DO NOT promote a known BUILD_REQUIRED failure.

DO NOT confuse pending host acceptance with build failure.

DO NOT claim publication without remote read-back.

DO NOT invent licenses or legal policy.

DO NOT expose secrets.

DO NOT overwrite an unrelated official project directory.

DO NOT force-push.

DO NOT leave two silently competing canonical project homes.
```

The governing transformation is:

```text
VERIFIED BUILD-DEV PROJECT
        ↓
GRADUATION GATE
        ↓
PRESERVE IDENTITY + HISTORY
        ↓
ESTABLISH OFFICIAL PROJECT IDENTITY
        ↓
MOVE TO OFFICIAL PROJECT HOME
        ↓
CREATE / CONNECT OFFICIAL REPOSITORY
        ↓
PUSH
        ↓
REMOTE READ-BACK
        ↓
UPDATE EVIDENCE CLASSIFICATION
        ↓
OFFICIAL PROJECT
```

Start from the existing build-dev repository.

**Graduate it. Do not rebuild it.**

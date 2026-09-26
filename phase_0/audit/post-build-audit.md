# GPT / Browser Post-Build Audit Procedure

Audit the committed repository state, not only the OpenCode final message.

## Required inputs

- expected hub commit from the execution handoff;
- prompt-pack digest;
- builder prompt digest;
- source identity (Git commit or archive SHA-256);
- resulting source-repository commit;
- OpenCode transcript;
- mechanical evidence report;
- builder prompt and verifier prompt.

## Procedure

1. Resolve the exact resulting source commit and inspect its tree/diff.
2. Re-open the exact builder prompt from the sealed prompt pack.
3. Compare each required task against concrete repository evidence.
4. Inspect tests and verification output; distinguish "test exists" from "test passed".
5. Check for uncommitted residue reported by the mechanical evidence collector.
6. Review unexpected conditions, known failures, unresolved issues, and scope deviations.
7. Check that no report claims stronger provenance than the evidence provides.
8. Record PASS, REPAIR, or BLOCK in the build ledger.
9. If REPAIR, define the smallest bounded repair phase; do not silently expand the prior phase.
10. If PASS, record the audited commit as the next phase's source base.

## Dispositions

- PASS: required work is supported by committed evidence and verification.
- REPAIR: bounded defects or omissions exist and a repair phase can address them.
- BLOCK: identity/provenance is broken, required evidence is unavailable, or continuing would rely on guessing.

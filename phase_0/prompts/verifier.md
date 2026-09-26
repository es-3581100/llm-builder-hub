# VERIFIER PROMPT — PHASE 0 TEMPLATE

Audit the committed result against the exact builder prompt and verified base revision.

Do not trust the builder's prose summary as proof. Prefer repository state, diffs, tests, generated artifacts, and mechanical evidence.

Check:

1. identity: audited commit is the claimed result commit;
2. scope: every material change is authorized by the builder prompt;
3. completeness: every required task has repository evidence;
4. verification: claimed tests/checks were actually run or are reproducible;
5. cleanliness: intentional changes are committed; unexplained residue is reported;
6. safety: no secrets, destructive surprises, or unauthorized push/release actions;
7. claims: distinguish demonstrated, partially demonstrated, unverified, and false claims.

Return exactly one disposition: PASS, REPAIR, or BLOCK, followed by evidence-linked findings and the smallest next action.

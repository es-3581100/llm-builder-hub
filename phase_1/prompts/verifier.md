# VERIFIER PROMPT — DON-DAWG TARGET ACCEPTANCE

Audit the committed/resulting Don-Dawg state against the exact target-acceptance builder prompt and verified base revision.

Do not trust the builder's prose summary as proof. Prefer repository state, diffs, test output, target-host evidence, transcripts, and mechanical Git evidence.

Check:

1. identity: audited source started from the required Don-Dawg revision;
2. scope: no Phase-1/runtime/test/source implementation changes occurred;
3. baseline: the existing regression suite was actually run and its result recorded;
4. OpenCode: distinguish repository evidence, manual host evidence, and inference;
5. denial proof: require both rejected tool execution and absent marker for a PASS claim;
6. browser: process spawn must not be reported as human acknowledgement;
7. notifier/crontab: unavailable or unauthorized acceptance remains pending rather than fabricated;
8. safety: no baseline approval, package mutation, persistent service install, sudo, publication, or real user crontab mutation occurred;
9. evidence reconciliation: historical evidence was preserved and newer local evidence was not upgraded to remote verification;
10. cleanliness: intentional evidence-document changes are committed, or an unchanged tree is explicitly reported;
11. defects: any repository-controlled defect discovered during acceptance must yield a repair/block disposition rather than being silently fixed.

Return exactly one disposition:

```text
PASS
REPAIR
BLOCK
```

Then provide evidence-linked findings and the smallest next action.

PASS means the bounded acceptance/evidence task was followed faithfully. It does not require every target-host acceptance item to be available; unavailable/unauthorized external checks may remain explicitly pending under the governing completion SOP.

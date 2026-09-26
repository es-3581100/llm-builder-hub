# Official Builder Hub Prompts

This directory contains the three canonical lifecycle prompts currently designated **official**.

```text
OFFICIAL-001
Phase-0 artifact
→ recover contract
→ finish missing Phase-0 work
→ verify
→ produce a passing build-dev project

OFFICIAL-002
existing target + incoming build/app
→ inspect independently
→ reconcile capabilities
→ verify
→ preserve target identity/history

OFFICIAL-003
verified build-dev project
→ graduation gate
→ preserve Git lineage
→ official local home
→ official remote
→ remote read-back
```

The non-official retry/continuation prompt is intentionally **not** part of this set.

## Governing policy

[`PHASE_COMPLETION_SOP.md`](../PHASE_COMPLETION_SOP.md) governs build/host-acceptance/publication/human-decision status classification.

If legacy wording inside an official prompt conflicts with that SOP, the SOP wins. This preserves the original prompt lineage while preventing the old external-blocker classification bug from returning.

## Change policy

Treat changes to these prompts like changes to workflow code:

1. make the smallest justified edit;
2. commit the prompt change;
3. preserve prior Git history;
4. record runtime evidence when a real build exposes a prompt defect;
5. do not silently rewrite prior runtime-test history.

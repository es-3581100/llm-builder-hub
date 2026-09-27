# VERIFIER PROMPT — PHASE 4 LOCAL GIT COMMIT CONTROL

Audit the published Phase-4 result against:

```text
phase_4/prompts/builder.md
phase_4/spec/LOCAL_GIT_COMMIT_CONTROL.md
```

and the sealed Phase-3 base:

```text
f42531ddc654b6357c19bda1ffa507d560bdb523
```

Do not trust the builder summary as proof.

Verify from committed repository evidence:

1. Phase-3 ancestry and preservation.
2. No sealed Phase-4 prompt/spec/manifest drift.
3. No Phase-5 or remote-product scope expansion.
4. Stage/unstage/commit are separate from SAVE CHANGES and WRITE FILE.
5. Product Git mutation does not use `git add`, `git commit`, `git reset`, or `git restore`.
6. Mutating Git calls use argument arrays/plumbing and CAS guards.
7. Custom filters/working-tree-encoding and in-progress Git operations are safely rejected.
8. No hooks/editor/signing/remote operation is introduced by the commit path.
9. Typed endpoints/errors/logging match the contract.
10. Static export remains mutation-free.
11. Full Go/Node/race/vet checks pass.
12. Automated Playwright acceptance is non-interactive and proves index/HEAD/branch/remote invariants.
13. Intentional changes are committed and branch publication is fast-forward only.

Return one disposition:

```text
PASS
REPAIR
BLOCK
```

Then give evidence-linked findings and the smallest next action.

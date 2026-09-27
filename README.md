# Rick's LLM Builder Hub

The canonical human + agent workflow artifact is [`index.html`](./index.html).

Phase 0 bootstrap files live under [`phase_0/`](./phase_0/).

The Hub/project responsibility boundary is defined in [`HUB_BOUNDARY.md`](./HUB_BOUNDARY.md). Phase completion, host-acceptance, publication, and human-decision handling are defined in [`PHASE_COMPLETION_SOP.md`](./PHASE_COMPLETION_SOP.md).

Real execution records are indexed under [`runtime_tests/`](./runtime_tests/). The first recorded run is **RT-001 — Don-Dawg Phase-0**.

The three canonical lifecycle prompts are versioned under [`official_prompts/`](./official_prompts/).

## Local-first workstation

Phase 1 introduces the first local-authority workstation vertical slice under [`local_app/`](./local_app/). It is intentionally isolated from the sealed Phase-0 pack and proves explicit draft/save/refresh semantics, the 0:4 / 1:3 / 2:2 / 3:1 workstation geometry, integrated five-slot tool spines, semantic document panes, and read-only single-file HTML5 export.

This branch does **not** yet add Git mutation UI, OpenCode execution, remote synchronization, publication, or graduation automation.

Core invariant:

> GPT designs → Git records → OpenCode executes → Git records → GPT audits.

Boundary invariant:

> The Hub transports intent without owning project-specific intent. Project architecture remains payload.

Do not treat conversational memory as build state.

## License and AI/TDM use

This repository is source-available under the [3-Clause BSD NON-AI License](./LICENSE). AI-assisted **inference-time** use for the documented builder workflow is welcome; use for AI/ML training or model improvement is not licensed.

See [AI_USAGE_POLICY.md](./AI_USAGE_POLICY.md) for the human-readable policy and deployment notes for the repository's TDM/crawler signals.

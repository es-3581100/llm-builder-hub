# Rick's LLM Builder Hub

The canonical human + agent workflow artifact is [`index.html`](./index.html).

Phase 0 bootstrap files live under [`phase_0/`](./phase_0/).

Phase completion, host-acceptance, publication, and human-decision handling are defined in [`PHASE_COMPLETION_SOP.md`](./PHASE_COMPLETION_SOP.md).

Real execution records are indexed under [`runtime_tests/`](./runtime_tests/). The first recorded run is **RT-001 — Don-Dawg Phase-0**.

Core invariant:

> GPT designs → Git records → OpenCode executes → Git records → GPT audits.

Do not treat conversational memory as build state.

## License and AI/TDM use

This repository is source-available under the [3-Clause BSD NON-AI License](./LICENSE). AI-assisted **inference-time** use for the documented builder workflow is welcome; use for AI/ML training or model improvement is not licensed.

See [AI_USAGE_POLICY.md](./AI_USAGE_POLICY.md) for the human-readable policy and deployment notes for the repository's TDM/crawler signals.

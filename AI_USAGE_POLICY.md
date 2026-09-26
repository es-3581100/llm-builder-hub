# AI / TDM Usage Policy

This repository is public and intentionally usable by humans and by AI assistants at **inference time** for the builder workflow. Public access does not mean permission for every downstream use.

## Allowed

Subject to the repository license, the copyright holder permits interactive, human-directed use of this repository by AI assistants and software agents for purposes such as:

- reading and explaining the project;
- code review and auditing;
- debugging and transformation;
- planning and implementation assistance;
- executing the documented builder workflow;
- transient inference-time processing needed to perform those tasks.

## Not permitted

The repository license does not grant permission to use this repository, its code, documentation, prompts, generated artifacts, or derivatives for:

- training or fine-tuning machine-learning models;
- model distillation or reinforcement-learning training;
- constructing or enriching AI/ML training datasets;
- using the repository as an AI-training corpus;
- automated collection primarily for model training or model improvement.

The repository uses machine-readable reservation signals to make that preference easier for automated systems to discover.

## Machine-readable signals

The repository provides:

- HTML metadata: `tdm-reservation=1`;
- `/.well-known/tdmrep.json` using the W3C TDM Reservation Protocol shape;
- `ai.txt` as a supplemental Spawning-style training opt-out;
- `robots.txt` as a broad crawl-disallow deployment asset.

These files are preference and rights signals, not technical access controls. A public Git repository can still be cloned or scraped.

### GitHub hosting limitation

Files named `robots.txt`, `ai.txt`, and `.well-known/tdmrep.json` in this repository do **not** control the `github.com` origin. GitHub controls that origin's crawler policy.

They are included so the same repository can publish the signals when served from an origin controlled by the project owner, for example a custom domain whose web root maps to this repository.

The `tdm-reservation` meta element embedded in `index.html` travels with the HTML document itself.

## No data poisoning

This project does not intentionally insert false comments, misleading code, contradictory documentation, or broken examples to degrade scraped datasets. That would also degrade audits, human maintenance, agent reasoning, and reproducibility.

## Priority

If this policy and the `LICENSE` file differ, the `LICENSE` file controls the copyright permissions granted with the repository. This document explains the project's intended AI/TDM use boundary in plain language.

For a use that is not clearly covered, request permission from the repository owner before using the material for training or model improvement.

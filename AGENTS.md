# Contributor instructions

Read README.md and docs/repository-split.md for source ownership and migration
limits. Preserve unrelated changes. Do not commit or publish automatically.
Run workflows from this repository root using pinned mise tasks. `mise run test`
and `mise run build` are the minimum validation for module changes.

Keep handwritten generators and templates in Git. Do not edit generated host
bindings by hand; update the owning generator and regenerate. Project .karty/
and dist/ outputs are ignored. Leave local .codex/ and .agents/ directories alone.
Keep protocol bounds, complete-batch validation, generated-file ownership,
path confinement and allocation constraints intact. Public API/wire changes
need a versioned decision. Public language/SDK changes must update their pinned
documentation and coverage tests. Distinguish compilation, actual WASM
execution, browser graphics, and manual review in the handoff.

The engine owns core/api, core/internal/codegen and host. KartUI owns compiler,
schema and codegen project adapters. The CLI owns project building. The public SDK owns cartridge/level
format packages. Public repositories must remain buildable without private
source or credentials. Record exact checks, outcomes and remaining limits.

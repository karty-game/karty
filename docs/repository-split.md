# Repository split

| Source owner                                   | Contents                                                                                              |
| ---------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `karty-game/karty-engine` (private)            | Core API/WIT, SDK definitions/templates, binding generator, host/runtime/renderer and SDK publication |
| `karty-game/karty` (public; local `karty-cli`) | Project building, asset import/cache, staging, samples and customer integration                       |
| `karty-game/karty-sdk` (public)                | Shared cartridge/level/world formats, codecs and public SDK/host releases                             |
| `karty-game/karty-ui` (public)                 | Compiler, schema, language docs, project adapters and editor integration                              |
| `karty-game/karty-tools` (public)              | Native Materialize builds, releases and tool-specific patches                                         |

Public repositories build with versioned public modules and published artifacts,
without private source or credentials. Ignored local workspaces may select
sibling public modules; do not copy dependency trees.

SDK manifests pin Materialize release artifacts. The CLI owns installation and
asset execution, while native tool builds remain in karty-tools.

Keep handwritten generators/templates tracked. Update the engine generator for
protocol bindings and KartUI generators for UI adapters; never hand-edit generated
host bindings. The CLI copies selected SDK bindings into derived project output.
Project `.karty/` and `dist/` are ignored; leave local `.codex/` and `.agents/` alone.

Preserve path confinement, protocol bounds, complete-batch validation and allocation
limits. Public API/wire changes require a versioned decision; language/SDK changes
also update pinned docs and coverage. Published SDKs remain immutable.

Each module validates its contracts. The engine validates/publishes SDK and host
artifacts independently; the CLI owns customer integration before its release.
Use root mise tasks; distinguish compilation, execution, graphics and manual review.

Public CLI, KartUI, SDK formats and exported SDK materials use MIT. Engine source
is proprietary; its separate runtime license permits free/commercial game distribution.

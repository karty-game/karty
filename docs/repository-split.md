# Repository split

The original `kefniark/karty` working tree remains intact. This is a clean source
import into the existing destination repository; no Git history was rewritten.
Existing uncommitted source changes were included. The assistant has not committed, pushed, tagged, or published this
extraction.

| Source owner | Contents |
| --- | --- |
| `karty-game/karty-engine` (private) | WIT, SDK definitions and templates, engine binding generator, host/runtime and renderer, compatibility/release tools |
| `karty-game/karty` (public; local directory `karty-cli`) | CLI, project build/staging, samples, SDK installation |
| `karty-game/karty-sdk` (public) | Shared cartridge/level formats and public SDK/host releases |
| `karty-game/karty-ui` (public) | Compiler, schema, language reference, project Go adapter, VS Code integration |

The public CLI and engine both use KartUI. Both engine and CLI consume public
`karty-sdk/format` packages. The CLI never imports engine source. A ZIP SDK bundle
contains generated client bindings, pinned templates/docs and compatibility
metadata; native/web hosts are separate checksummed artifacts.

Each repository is one Go module, with versioned public module dependencies.
There are no copied dependency trees. During this unpublished migration, ignored
local `go.work` files connect sibling checkouts. Publish KartUI and Karty SDK v0.0.1 first,
then Karty v0.0.1, and resolve their module checksums before enabling standalone
CI/release builds. These versions are intended initial tags, not existing releases.
The public CLI needs only public KartUI and SDK source; engine access is never required.

Handwritten generators and templates are tracked: engine `core/internal/codegen`
owns protocol bindings, while KartUI `codegen` owns project UI adapters. Generated
host bindings are also tracked so CI can detect drift. Generated game `.karty/`
files and `dist/` outputs are ignored. Local `.codex/` and `.agents/` directories
are ignored and are not managed by repository workflows.

SDK 0.0.1 is the single supported baseline. Engine API, host, docs, and
project templates all use 0.0.1. Unreleased duplicate versions were removed
from the split repositories. The embedded bundle does not imply that public
host artifacts have already been released.

The CLI and KartUI can release independently. Bundle format 1 currently targets
UI schema 9 and project Go adapter 1. An SDK pins its API, host, docs, templates,
and compiler tools. Unsupported bundle/schema/adapter versions fail early.

Before public release: publish compatible immutable
SDK/host assets and initial public module tags, configure the signing secret,
and validate the hosted CI workflows. Existing repo visibility and Git history
are unchanged. Original issues/releases and archival are separate actions.

Validation ownership: each module checks its own contracts and implementation.
The engine publishes tested SDK/host artifacts without checking out the CLI.
The CLI owns end-to-end customer tests against published SDK/host artifacts and
must pass them before publishing a CLI release.

Public CLI, KartUI, SDK formats, and exported SDK materials use MIT. Engine
implementation source is proprietary; its runtime license permits distribution
with free and commercial games. See the repository license files.

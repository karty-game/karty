# Contributing to Karty

Contributions to CLI commands, project scaffolding, builds, development mode,
and samples are welcome. Bug fixes, documentation improvements and
[reproducible bug reports](https://github.com/karty-game/karty/issues/new?template=bug_report.md)
are good starting points.
For a substantial feature or a compatibility change, open an issue first so we
can agree on scope before implementation.

## Local setup

Fork and clone this repository, install [mise](https://mise.jdx.dev/), then run
these commands from the repository root:

```sh
mise install
mise run check-fmt
mise run test
mise run build
mise run lint
```

`mise install` also installs the repository hk pre-commit hook. Formatting and
lint use the same pinned `hk.pkl` configuration locally and in CI: golangci-lint
for Go, yamllint for YAML, Taplo for TOML and Prettier for web files and Markdown.
Generated files, dependencies and local investigation folders are excluded.

Public development needs only the public KartUI and SDK modules. You do not need
private engine access or release secrets. For local public-module changes,
use an ignored workspace:

```sh
go work init . # Skip if go.work already exists.
go work edit -replace=github.com/karty-game/karty-ui=../karty-ui
go work edit -replace=github.com/karty-game/karty-sdk=../karty-sdk
```

Use `GOWORK=off` to check the public dependencies pinned in `go.mod`.
Root tasks enable `GOEXPERIMENT=simd`; use `mise exec -- go ...` for ad hoc Go
commands. Tests prepare the SDK selected by `internal/release`; an uncached
published bundle needs a download. This checkout selects SDK 0.0.17 and its
matching public hosts. `KARTY_HOME` selects the SDK/toolchain cache root.

Repository tools use the Go version in `mise.toml`. Contributor sample and
integration checks resolve each installed SDK's own Go pin independently using
mise; `go run ./cmd/karty-test-sdk --go-path` prepares and prints the selected
SDK's compiler path. An explicit `KARTY_GO` is still checked exactly and is never
silently replaced. Normal CLI commands continue to require a user-installed
compiler as described in the toolchain documentation.

For build, browser, or development-flow changes, run the relevant integration
tasks listed by `mise tasks`. Common tasks stay in `mise.toml`; integration,
package, release and website tasks live in `tasks/`, with sample tasks under
`tasks/samples/`. Task names work from the repository root. The full
`mise run check-integration` suite consumes
published SDK/host artifacts; install browser dependencies with
`mise run install-browser` first. Linux native checks also need graphics libraries
and Xvfb. Record which checks you actually ran in your pull request.

## Source ownership

| Directory                         | Contents                                                               |
| --------------------------------- | ---------------------------------------------------------------------- |
| [`cmd/`](cmd/)                    | CLI entry point, release tooling and integration commands              |
| [`internal/`](internal/)          | Project building, scaffolding, SDK installation and development server |
| [`packages/`](packages/README.md) | Reusable game packages, each with its own Go module                    |
| [`samples/`](samples/README.md)   | Playable examples and their source assets                              |
| [`website/`](website/README.md)   | Public documentation, site templates and build tools                   |
| [`scripts/`](scripts/README.md)   | Integration checks and asset generators                                |
| [`tasks/`](tasks/)                | Focused mise task definitions                                          |
| [`testdata/`](testdata/README.md) | Shared test and integration fixtures                                   |

Website tools may invoke the CLI; CLI packages must not import website code.

The engine owns `core/api`, binding generators, the host/runtime and SDK
publication. KartUI owns its compiler, schema and project adapters. The public
SDK owns cartridge/level formats and codecs; `karty-tools` owns native
Materialize builds. The CLI installs released tools and builds projects.
Public repositories must build without private source or credentials.

Keep handwritten generators and templates in Git. Protocol bindings arrive in
the selected SDK bundle; change the owning generator and regenerate them.
Do not patch generated project `.karty/` files or commit `dist/` output.
Preserve path confinement, protocol bounds, complete-batch validation and
allocation limits. Published SDKs are immutable. Public API or wire changes
need a versioned decision alongside their owning source; public language/SDK
changes also update pinned documentation and coverage.

Root `docs/`, `spike/`, `spikes/`, `concept-art/` and `website/validation/` hold
local investigations and are ignored. Keep public documentation in
`website/content/`. Builds and tests must not depend on ignored files; promote
required fixtures into `testdata/`. Leave local `.codex/` and `.agents/` alone.

Keep `tmp/go.mod`, `dist/go.mod` and `spikes/go.mod` when clearing scratch files
or build output.
These module boundaries keep temporary Go sources out of root commands such as
`go mod tidy` and `go test ./...`; Git ignore rules do not affect Go package discovery.

## Preparing a CLI release

CLI versions are independent of SDK versions. First verify that the SDK selected
by `internal/release.CurrentSDK` has published signed metadata, its bundle and
matching hosts. Then synchronize sample pins and validate the public inputs:

```sh
mise run prepare-release
mise run check-release
GOWORK=off GOMAXPROCS=2 GOFLAGS=-p=1 mise run check
mise run install-browser
mise run check-integration
```

Linux CI uses these worker and package scheduling settings to keep the existing
test deadlines while checking public dependencies independently. Runtime checks
execute without Go test-result caching; compiler and download caches remain
available. Windows platform tasks use portable command quoting and also execute
their assertions on each runner.

Run integration against downloaded public hosts. `mise run check-release-config`
validates GoReleaser configuration; `mise run release-snapshot` builds local
archives and checksums in `dist/goreleaser/` without publishing.
Review the resulting source changes and release notes before choosing a new
CLI `vMAJOR.MINOR.PATCH` tag.
After maintainer approval, pushing that tag starts [Release CLI](.github/workflows/release.yml).
The workflow validates Linux amd64/arm64, Windows process cleanup and a packaged
Windows ARM64 game before GoReleaser publishes the CLI archives and checksums.
SDK/host publication remains owned by the engine release process.

## Sending a pull request

Keep each pull request focused on one problem. Explain the user-visible result,
include a reproduction for a bug where possible, and list your validation results.
Add regression coverage when it protects changed behavior. Run `mise run fmt`
for Go changes and review the result before submitting. Preserve unrelated work.

Dependabot proposes weekly Go module, GitHub Actions and npm updates. Minor and
patch updates are grouped; major updates stay separate. Changes to pinned tools in
`mise.toml` and SDK-selected toolchains are reviewed manually.

## License

Contributions are made under this repository's [MIT license](LICENSE.md).
Only submit material you have the right to contribute, and retain existing
third-party copyright and license notices.

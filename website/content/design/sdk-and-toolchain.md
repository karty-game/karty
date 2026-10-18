# SDK versions and toolchains

The CLI builds your game, but the SDK pinned in `karty.toml` decides which
engine API, asset processors and hosts it builds against. Keeping that version
explicit lets one game stay on a known release while another uses a newer one.

## The project chooses its SDK

A project's manifest contains an exact version, for example:

```toml
[sdk]
version = "0.0.13"
```

The version above is an example pin, rather than a request for the latest SDK.
The bundle supplies generated Go bindings, starter templates, asset processing
rules, tool versions and reference documentation. Compatible native and web
hosts are separate downloads selected through that release's metadata.

The build copies the matching reference into `.karty/docs/`. Read it for the
APIs and authoring features your project supports. A newer website example may
use a capability your pinned SDK does not have.

## CLI and SDK versions have different jobs

| Version                  | What it selects                                                   |
| ------------------------ | ----------------------------------------------------------------- |
| CLI release              | Project commands, build orchestration and supported SDK formats   |
| Project SDK pin          | Game bindings, templates, asset capabilities and compatible hosts |
| Tool versions in the SDK | The compiler and processing tools used to produce game files      |

Installing a newer CLI does not change an existing project's SDK pin. Likewise,
installing an SDK adds it to the local cache; it does not select it for a game.
Ordinary Go dependencies still follow the game's `go.mod` and `go.sum`.

## Prepare the build tools

Install the version declared by your project, then prepare its tools:

```sh
# Use the version from your game's [sdk] table.
karty sdk install 0.0.13

# Run this from the game project.
karty toolchain install
```

The toolchain includes Go, TinyGo, wasm-tools and, for SDKs with generated world
materials, Materialize. Install the SDK's required Go version yourself and make
it available on `PATH`, or select it with `KARTY_GO`. Karty checks the version;
`toolchain install` does not download Go. It downloads and verifies the managed
TinyGo, wasm-tools and Materialize artifacts selected by the SDK.

Builds prepare the tools they need, so explicit installation is useful when
setting up a machine or CI cache. `karty dev` defaults to the standard Go
compiler for its feedback loop. Regular builds follow `project.compiler`,
which defaults to TinyGo. Both produce a WebAssembly cartridge for a compatible
host; the compiler does not choose the destination's host executable.

Players need the packaged game files, not your compiler or CLI.

## Upgrade deliberately

To move a game to another SDK, install that released version, change `[sdk]` in
`karty.toml`, and rebuild. Review the regenerated `.karty/docs/`, update code
and sources for any changed contracts, then test the packaged game on its
target platforms. Commit the pin with those source changes. Generated files
under `.karty/` are recreated by the build.

SDK bundles and runtime downloads are verified before reuse. Tool versions
and checksums also participate in asset cache identity, so changed processing
inputs rebuild rather than silently reusing an older result.

Continue with the
[asset pipeline](asset-pipeline.html) to see what the selected tools produce.

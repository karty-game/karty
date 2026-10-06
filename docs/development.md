# CLI development

Run pinned tasks from this repository root; private engine access is unnecessary.

```sh
mise install
mise run fmt
mise run check-fmt
mise run lint
mise run test
mise run build
./dist/karty new my-game
```

`mise run test` checks sample pin drift and prepares the SDK selected by
`CurrentSDK` and SDKs 0.0.7/0.0.5 for compatibility coverage before unit tests: missing
cache entries download; corrupt ones fail.
Direct `go test` requires an installed SDK. The shared checks in `hk.pkl` use
golangci-lint for Go (including `govet`), yamllint for YAML, Taplo for TOML,
and Prettier for YAML layout, JSON, Markdown and web files. Taplo validates
syntax without downloading schemas.

`mise install` installs the pinned tools and repository pre-commit hook through
mise; use `mise run install-hooks` to reinstall it. The hook checks staged files
and applies formatting fixes while preserving unstaged work. `mise run fmt`
applies all formatters through hk without staging; `mise run check-fmt` checks
formatting without writing files. `mise run lint` runs all format and lint
checks. `mise run check` also runs tests and build.

Generated Go, `*.generated.*` snapshots and canonical `*.world.json` fixtures,
derived output and local contributor
directories are excluded from formatting. Regenerate owned outputs instead;
after editing the public SDK action schema, refresh its snapshot with
`mise run generate-action-contract`.

See [Releases](releases.md) for pin changes and [SDK bundles](sdk-bundles.md) for
published/local installs. `KARTY_HOME` changes only the SDK cache root; toolchain
and host caches remain under `~/.karty`. Generated `.karty/docs` matches the SDK.

The root mise environment enables `GOEXPERIMENT=simd` for Go 1.27 portable
SIMD in the public SDK offline baker. Use `mise exec -- go ...` for ad hoc
Go commands, or set that experiment explicitly for direct source builds.
Prebuilt CLI/host users do not need a runtime flag.

## Customer integration and overrides

Install the selected SDK with `karty sdk install VERSION`; `KARTY_TEST_SDK=VERSION`
overrides the default integration selection. Run `mise run install-browser`, then
`mise run check-integration`. Linux needs Xvfb and graphics libraries. The suite
covers native loading, actual browser execution, UI lifecycle/input, allocations
and dev rebuilds.

Set `KARTY_HOST_NATIVE` / `KARTY_HOST_WEB` to test prebuilt candidate hosts;
the web override requires its matching `wasm_exec.js` beside it. Unit tests need
neither host override. `karty dev` otherwise resolves hosts from the project SDK.
The separate [world-camera check](../samples/world-camera/README.md) requires
explicit checker/tool artifacts; it is not a public unit-test prerequisite.

### Materialize installation smoke

The default SDK 0.0.8 pins released Materialize 2.0.0. Install its tools with
`karty toolchain install`; see [SDK bundles](sdk-bundles.md). SDKs advertising
`world/material-atlas@1` generate material maps during world asset builds.
SDK 0.0.7 remains supported without atlas generation.
Normal Materialize tests use fixture transports, without network or execution.
For an explicit public-release download and native smoke, run from this root:

```sh
KARTY_TEST_MATERIALIZE_RELEASE=1 mise exec -- go test ./internal/toolchain -run '^TestMaterializeRelease$' -count=1 -v
```

This engine-free test uses inline release pins, an isolated temporary tool cache,
real checksum-verified installation through `Ensure`, and a network-disabled warm
install. Only `--version`, `--help` and `--list-maps` execute; no GPU/map generation
is tested. The temporary installation is removed after the test.

### Headless material generation in CI

Linux CI does not need a physical GPU or display. Ordinary `karty build` and
`karty dev` first use Materialize's default backend. If adapter discovery fails,
the Go asset executor automatically retries with software OpenGL and surfaceless
EGL. Explicit `MATERIALIZE_GPU_BACKEND` selections are respected; shader,
generation and cancellation failures are not retried or hidden.

The Build samples and Checks workflows install libegl1 and libgl1-mesa-dri.
Checks then runs
`mise run check-materialize-software` without backend flags or Xvfb.
It downloads the checksum-pinned release,
generates actual normal/height/AO maps from a packed extruded atlas, merges and
encodes both QOI textures, validates the pair, and checks warm-cache reuse.
Failures are errors, not skips or neutral-map substitutions.

For headless `karty build` with an atlas-capable SDK, provide Mesa EGL/DRI
libraries in the CI image; no special command wrapper or environment is needed.
This affects asset generation only; the game target may still be native or web.
No Python orchestration, GPU device
passthrough, Vulkan driver, or globally installed Materialize binary is needed
for this tested configuration. Other operating systems/backends need separate
validation; software graphics do not imply byte-identical GPU outputs.

## Project camera configuration

The optional `[project.camera]` table accepts `fov_y_degrees`, `near`, `far`
and `ortho_height`. Defaults are 60°, 0.05, 80 and 22 world units. With this
table, `[project.resolution]` must fit the current 1280×720 camera bound.
The CLI validates these values and KartUI emits typed constants in
`.karty/config/project.go`, including resolution and FOV converted to radians.
Game code must apply those constants to camera settings; the world-camera
sample does so for both projection modes, viewpoint changes and reloads.
Rebuild after changing `karty.toml`; `karty dev` watches it automatically.

## Optional sibling modules

Use an ignored local workspace; if one already exists, skip initialization:

```sh
go work init .
go work edit -replace=github.com/karty-game/karty-ui=../karty-ui
go work edit -replace=github.com/karty-game/karty-sdk=../karty-sdk
```

Use `GOWORK=off` when validating the released public dependencies in [go.mod](../go.mod).

## Repository tooling

`mise run build-samples` runs the Go sample-site builder in `cmd/karty-samples`.
The Pages workflow uses its `assemble` subcommand through `mise run assemble-samples`
to validate static artifacts, preserve other previews and prune closed PRs.
Validation rejects links, unexpected paths, incomplete samples and outputs above
10,000 entries or 200 MiB before replacing a preview; filesystem roots confine writes.
These checks run in the normal Go test suite.

The publisher disables Git hooks only for its generated-site commit and push.
That worktree shares source repository hooks but contains static previews rather
than a mise/hk project. Source commits retain the pinned hk checks.

The sample artwork generator lives in `cmd/world-camera-materials`; use the normal
world-camera sample for lighting and renderer checks. Native Materialize builds
and releases belong to
[karty-tools](https://github.com/karty-game/karty-tools); the CLI installs SDK-pinned
released binaries.
The remaining scripts run customer integration, native smoke and browser video checks.

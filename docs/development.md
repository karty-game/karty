# CLI development

Run pinned tasks from this repository root; private engine access is unnecessary.

```sh
mise install
mise run test
mise run build
./dist/karty new my-game
```

`mise run test` checks sample pin drift and prepares the SDK selected by
`CurrentSDK` and SDKs 0.0.7/0.0.5 for compatibility coverage before unit tests: missing
cache entries download; corrupt ones fail.
Direct `go test` requires an installed SDK. Use `mise run lint` for lint checks.
See [Releases](releases.md) for pin changes and [SDK bundles](sdk-bundles.md) for
published/local installs. `KARTY_HOME` changes only the SDK cache root; toolchain
and host caches remain under `~/.karty`. Generated `.karty/docs` matches the SDK.

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

The Checks workflow installs libegl1 and libgl1-mesa-dri, then runs
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

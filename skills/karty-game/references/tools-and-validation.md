# Tools and validation

Run game commands from the directory containing `karty.toml`. Repository
contributor mise gates are not game workflows.
Public setup, package and distribution guides are indexed in
[Public sources](public-sources.md#guides-and-api-behavior).

## Setup

If the CLI is absent, install a public [Karty CLI release](https://github.com/karty-game/karty/releases).
For an existing game, install its exact `[sdk].version`:

```sh
karty sdk install VERSION
```

The SDK ZIP contains `manifest.toml` with the exact `tools.go` compiler pin:
`${KARTY_HOME:-$HOME/.karty}/sdks/VERSION/sdk.zip`. Activate that Go version on
`PATH` and check `go version`. The `go.mod` language version lacks the patch pin.
`KARTY_GO`/`--go` select Karty's build compiler, but **do not select Go for
`fmt`, `lint`, raw `go test` or `go install`**; those need the correct `PATH`.

Formatting and linting require a separate golangci-lint v2 installation. The
current CLI game profile recommends v2.14.0; use a project's documented pin when
different. With the SDK Go active:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
golangci-lint version
```

Put Go's binary output directory on `PATH`, or pass
`--golangci-lint /absolute/path/to/golangci-lint` to `karty fmt` / `karty lint`.

`dev` and `build` prepare the managed tools they need. Optional
`karty sdk toolchain install` prepares all pinned TinyGo/wasm-tools/Materialize
tools in advance; it requires Go already installed and does not download Go.
`sdk current` reports the CLI default, not this game's pin. `KARTY_HOME` selects
SDK/tool caches. Reuse writable `GOCACHE` / `GOLANGCI_LINT_CACHE`; Materialize
tool/cache directories must be real directories rather than directory symlinks.

## Keep one development preview running

```sh
karty dev                 # web preview, Go compiler, http://127.0.0.1:4242/
karty dev --port 0        # alternative: choose an available loopback port
```

Run one of these in a persistent terminal/session. Open its printed URL; in remote
workspaces use the printed Workspace preview URL when available. The listener can
start before the first build finishes: wait for `Preview updated (build N)`.
Edit Go, `.kui`, world/manifest or asset source and save. No external watcher or
server restart is needed for ordinary source edits.

- Code, geometry and binding changes restart the game and reset session state.
  Compatible material/lighting/global UI updates can retain running state;
  details are in `.karty/docs/development.md`.
- A failed build reports diagnostics while retaining the last successful game.
  Correct the authored source and wait for a successful new build before testing.
- Dev refreshes enabled offline lightmaps incrementally before publishing.
  The first preparation/bake can be slower than later cached edits.
- Stop your session with Ctrl+C. Keep previews on loopback and preserve existing
  listeners. Dev supports web; use `karty build --target native` for desktop.

Do not run a second build on every save while dev is already rebuilding. To check
one change without leaving a server running, use the finite alternative:

```sh
karty build --target web --development --compiler go
```

This regenerates assets/UI/world/bindings and compiles Go, but uses a
geometry/cache-first lighting policy instead of refreshing dev's offline bakes.
It is not a production-lighting or TinyGo acceptance check.

## Format, lint and test

In another terminal with the same Go environment:

```sh
karty fmt                  # apply handwritten Go formatting
karty fmt --check          # non-writing check; fails on differences
karty lint                 # report cartridge-target Go diagnostics
karty lint ./src/...       # restrict diagnostics to these packages
go test ./internal/rules   # example: existing portable rules package
```

Run only tests for packages that exist. Test changed portable rules early; native
`go test ./...` does not validate WASM-only adapters. `fmt` / `lint` use
golangci-lint with WASI/wasm, CGO disabled and `GOWORK=off`. Lint needs current
generated inputs: wait for a successful dev rebuild, or run the finite build
after asset/UI/action changes. Fix the authored inputs causing generated-code
errors; do not edit `.karty/`.

Configuration precedence: explicit `--config FILE`, project-root `.golangci.*`,
then Karty's game default. A custom config replaces the default. `karty lint --fix`
opts into supported source fixes; review its diff. Generated `.karty/`, `dist/`
and agent directories are excluded by the game default.

For YAML/level changes, run `karty schema --check` before expensive processing.
Use dev **or** the finite Go build for iteration; finish with the configured
production build once the change is ready. Regular builds use `project.compiler`
(TinyGo by default); Go development success does not prove TinyGo compatibility.

## Bake lighting when its inputs change

The [level lighting recipe](assets-and-levels.md#lighting-and-baking) requires
authored lights and enabled `[lightmap]` settings in `levels/room/level.toml`.
While dev is running, let it refresh enabled offline bakes automatically. Use
the manual sequence for offline preparation or an explicit bake check. For an
unchunked level with `offline = true`, stop dev before a manual bake:

```sh
karty schema --check
karty render bake --level room --incremental
karty dev
```

`room` can be its directory name or logical level name; omitting `--level` selects
all enabled recipes. The command writes validated artifacts below
`.karty/bakes/<directory>/`; builds consume them without manual copying.
`--incremental` reuses complete matching results. Rebake after geometry, material,
UV or selected-light edits, not after unrelated Go/UI changes.

Persist quality in `[lightmap]`: `density`/`page_size` set receiver resolution;
`bake_samples`/`bake_bounces` set sampling and diffuse bounces. Start with the
reference's low-density recipe. `--workers 2` bounds CPU parallelism without
changing the authored quality. `karty render bake --help` lists quality overrides.
Stale/missing automatic bakes may fall back to live lights; inspect the build log
and scene rather than treating successful compilation as bake evidence.

Chunked levels use `karty build` to bake/package individual chunks; the standalone
bake command rejects them. Baking itself is CPU-only; material generation can
separately require OpenGL/EGL. Avoid concurrent dev/build/manual bakes on one cache.

## Inspect and ship the result

For a static level capture:

```sh
karty render screenshot --position 2,2,1.7 --yaw 0 --output dist/screenshots/room.png room
```

Then open the output PNG with the available image viewer and compare it with the
requested scene. Follow [Capture and inspect world content](visual-validation.md)
for camera placement, batch viewpoints, lighting/host requirements and the
inspect → correct → recapture loop. This captures authored content; inspect the
running game for client-created objects, UI and interactions.

At a completion point, stop dev and test the production output:

```sh
karty fmt --check
karty lint
karty build --target web
karty serve dist/web --addr 127.0.0.1:8080
```

`serve` is a persistent static server: it neither builds nor watches. Open its HTTP
URL and exercise the changed path; stop it with Ctrl+C. HTTP success alone does
not establish game execution. Inspect browser console/host logs; supported hosts
have F3 or a browser Debug button.

Native: `karty build --target native`, then run `./karty-host --cartridge game.kart`
from `dist/native/` (`karty-host.exe` on Windows). An explicit target such as
`--platform windows-arm64` writes `dist/native/windows-arm64/`.
Ship the whole output, including `content/`, runtime glue/catalogs, licenses and
asset notices. Cross-packaging is not execution on the destination platform.

Include relevant lifecycle paths such as scene re-entry, input/focus reset, modal
UI, save round-trips/failures and media startup when testing changed behavior.

Report exact commands/outcomes and SDK/compiler/runtime. Separate schema checks,
compilation, WASM execution, graphics, audio and physical-device performance.

# CLI releases

CLI archives/checksums publish from `karty` tags `vVERSION`. SDK bundles, signed
metadata and native/web hosts publish separately in
[karty-sdk](https://github.com/karty-game/karty-sdk/releases) as `sdk-vVERSION`.
SDK tags do not trigger CLI publication; SDK, CLI and module versions may differ.

## SDK pins

`CurrentSDK` in [internal/release/version.go](../internal/release/version.go) is the
released SDK selection for CLI defaults and ordinary tests. Update it once, then
run `mise run prepare-release` to synchronize the managed sample manifests.
Tests install only this SDK. Capability checks read its bundle metadata rather
than matching SDK or API release numbers.

Public format/compiler module dependencies are selected separately in [go.mod](../go.mod).
The SDK and matching hosts must already be available as immutable public assets;
changing this selection does not publish them.

From the repository root, prepare and validate the release:

```sh
mise run prepare-release
mise run test
mise run build
mise run lint
mise run install-browser
mise run check-integration
```

Preparation updates managed samples to `CurrentSDK`; tests reject drift and prepare
the selected SDK cache. `karty sdk current` prints the selection for scripts. Preparation
does not commit, tag or publish. See [Development](development.md) for overrides.

## SDK 0.0.12 selection

The public format module **v0.0.14** and signed
[SDK/host 0.0.12](https://github.com/karty-game/karty-sdk/releases/tag/sdk-v0.0.12)
are published. They are separate version selections: the format module resolves
through the ordinary public Go proxy and checksum database; SDK installation
verifies signed metadata and matching public bundle/host assets.

`CurrentSDK` and all managed samples select 0.0.12. Run
`mise run prepare-release` after a future selection change, then repeat
`GOWORK=off mise run check` and customer integration against published assets.
A local archive or host override does not qualify the published integration path.

## Publication gate

The [release workflow](../.github/workflows/release.yml) runs tests/lint and the
full customer suite on Linux amd64/ARM64 before GoReleaser publishes CLI archives
for Linux amd64/arm64, macOS arm64 and Windows amd64. It uses public dependencies
and the repository's `GITHUB_TOKEN`, not private engine source or credentials.
The packaged Windows ARM64 game also undergoes cartridge verification and
startup smoke on a Windows ARM64 runner before publication.
Record compilation, actual WASM execution, browser graphics and manual review
separately; a startup smoke is not gameplay or pixel validation.

## v0.0.2 preparation

Validated on Linux amd64 on 2026-10-11, selecting published
[SDK 0.0.11](https://github.com/karty-game/karty-sdk/releases/tag/sdk-v0.0.11),
public format module `v0.0.12` and KartUI `v0.0.6`. `CurrentSDK` and all five
managed sample manifests agree. Checks used `GOWORK=off`, pinned mise tools,
`GOCACHE=/tmp/karty-engine-go-cache` and
`GOLANGCI_LINT_CACHE=/tmp/karty-engine-lint-cache`.

The isolated `KARTY_HOME=/tmp/karty-cli-release-home` installed the signed public
SDK into an empty SDK cache; its archive matched the cached 64-byte SHA-256
digest. Managed tool files were reused in real directories. Host overrides and
`KARTY_TEST_SDK` were unset for customer checks, so native/web hosts came from
the public release, without private engine source or credentials.

| Check                                                                                                          | Outcome                                                                                                                                                                                                        |
| -------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mise run prepare-release`                                                                                     | Passed; managed sample SDK pins agree.                                                                                                                                                                         |
| `mise run check`                                                                                               | Formatting, lint, tests and CLI build passed.                                                                                                                                                                  |
| `mise run build-samples`, `mise run assemble-samples`                                                          | Built and assembled the four published demos with public SDK assets: 178,951,733 bytes (170.662 MiB), below the unchanged 200 MiB preview limit.                                                               |
| `mise run test` restricted to two CPUs                                                                         | Passed all packages and all 87 level-builder root tests in 11 batches; slowest batch 2.035 seconds.                                                                                                            |
| `mise run install-browser`, `mise run check-integration`                                                       | Passed native cartridge verification, actual TinyGo WASM launcher, Go/Ebiten browser host, desktop/high-DPI touch UI lifecycle, zero guest allocations across 1,000 updates and managed Air rebuild lifecycle. |
| `mise run check-materialize-software`                                                                          | Passed real normal/height/AO/metallic/smoothness generation, linear reflectance/mips and cold/warm atlas cache checks; fresh tool cache also passed in 0.881 seconds.                                          |
| `mise run check-renderer-debug-ui`                                                                             | Passed browser control layout and input isolation at desktop and 320-pixel mobile widths using the diagnostic host stub.                                                                                       |
| `mise run build-cli-linux-amd64`, `build-cli-linux-arm64`, `build-cli-darwin-arm64`, `build-cli-windows-amd64` | All four CLI binaries compiled.                                                                                                                                                                                |
| `karty build --target native --platform windows-arm64` in `samples/ui-demo` under pinned mise tools            | Packaged the cartridge, levels and checksum-verified public Windows ARM64 host.                                                                                                                                |

Preparation first found formatting/lint failures and an unclassified malformed
Materialize TOML error; these were corrected. The original unbatched test task
passed locally with an empty Go cache, but
[CI run 38156737693](https://github.com/karty-game/karty/actions/runs/38156737693)
on `224f6ae` exceeded the shared nine-second level-builder package deadline.
The same limit also failed locally on two CPUs after reducing repeated Node
launches. The final task discovers every test and runs batches of eight with
the unchanged nine-second timeout. Cache lifecycle checks validate canonical
passive modules; dedicated packaging tests retain actual WASM execution.
The final restricted-session check passed formatting/lint but hit
`socket: operation not permitted` in a Go HTTP fixture. Rerunning the pinned
test task with localhost socket access passed; the pinned CLI build also passed.

The first UI browser run timed out during repeated Tab navigation. Waiting for
two subsequent host updates after key release passed the desktop/touch lifecycle
and the complete customer-suite rerun; no assertion or execution timeout was
removed or increased.

The [sample publisher on `224f6ae`](https://github.com/karty-game/karty/actions/runs/38156908865)
rejected 219.152 MiB of extracted files, although the compressed artifact was
65,138,605 bytes. Five identical 37,713,465-byte host modules accounted for
179.832 MiB. The unfinished `environment` project is now excluded from publication;
its source and SDK pin remain managed. The rebuilt four-demo site and the original
CI artifact with environment excluded both passed the unchanged publisher validation.
Each published demo still includes its own matching web host.

Linux ARM64 native execution and Windows ARM64 startup remain release-workflow
checks. macOS/Windows CLI compilation does not establish execution on those
systems. Browser checks used headless Chromium; Materialize used software
graphics. No physical audio, manual gameplay or physical mobile-device review
was performed. Commit/push the final preparation and obtain green main-branch
checks before tagging `v0.0.2`; preparation does not commit, tag or publish.

## v0.0.2 release checks — 2026-10-13

The reviewed working tree selects published SDK/hosts 0.0.12, public format
module v0.0.14 and KartUI v0.0.6. `mise run prepare-release` synchronized all
seven managed samples, including the three remaining 0.0.11 pins. Signed public
SDK installation passed; its ZIP SHA-256 is
`2ff8af37a16d97e5dda37a9131b22283a1a274f189f4ac29d5269aa267d0bf83`, matching
the strict 64-byte cached digest.

Asset probes now supply their own sprite instead of depending on the starter's
removed `player.png`. Scaffold tests qualify the published vector/text starter;
build fixtures retain their typed texture references and stripping assertions.
Local docs snapshot complete validated project builds independently of SDK
release status, preserving assembled games when no local build exists. Existing
path, symlink, incomplete-build and previous-preview protection tests pass.

Commands ran from the CLI root with:

```sh
export GOCACHE=/tmp/karty-engine-go-cache
export GOLANGCI_LINT_CACHE=/tmp/karty-engine-lint-cache
export GOWORK=off GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
export GONOSUMDB= GONOPROXY=
export KARTY_HOME=/tmp/karty-cli-release-home
export XDG_CACHE_HOME=/tmp/karty-tinygo-xdg-cache
export PLAYWRIGHT_BROWSERS_PATH=/root/.cache/ms-playwright
unset KARTY_HOST_NATIVE KARTY_HOST_WEB KARTY_HOST_CHECK KARTY_TEST_SDK
```

- `mise run check`: PASS formatting, lint (zero Go issues), sample-pin checks,
  full unit and bounded level-builder tests (25.17 seconds), and CLI compilation.
  Log: `/tmp/karty-cli-release-fixes-check-complete.log`.
- `mise run check-integration`: PASS public-host native cartridge verification,
  native/web starter packaging, actual TinyGo WASM launcher execution with a
  host test double, actual Go/Ebiten host and cartridge execution in headless
  Chromium, desktop/high-DPI primary-touch UI lifecycle, zero guest allocations
  across 1,000 updates, and managed Air rebuild lifecycle (4.686 seconds).
  Log: `/tmp/karty-cli-release-fixes-integration-final.log`.
- `mise run build-cli-linux-amd64`, `mise run build-cli-linux-arm64`,
  `mise run build-cli-darwin-arm64` and `mise run build-cli-windows-amd64`: PASS
  all four distributed CLI target compilations. Logs:
  `/tmp/karty-cli-release-fixes-PLATFORM.log`.

The first customer-suite attempt passed smoke and the actual launcher check,
then could not find Chromium because writable `XDG_CACHE_HOME` moved Playwright's
default browser lookup. Setting `PLAYWRIGHT_BROWSERS_PATH` to the existing pinned
Chromium cache passed the complete rerun while retaining TinyGo's writable cache.
Initial log: `/tmp/karty-cli-release-fixes-integration.log`. Earlier release gates
caught fixture-helper lint findings; the corrected complete gate passed without
disabling rules. Logs: `/tmp/karty-cli-release-fixes-check.log` and
`/tmp/karty-cli-release-fixes-check-final.log`.

These results qualify this uncommitted Linux amd64 working tree and the named
headless fixtures. Linux ARM64 customer execution and Windows ARM64 packaged-game
startup remain release-workflow gates; cross-compilation does not qualify those
devices. Sample-site rebuilding, physical audio, manual gameplay and hardware
FPS were not rerun. Nothing was committed, tagged or published by this check.

# CLI releases

CLI archives/checksums publish from `karty` tags `vVERSION`. SDK bundles, signed
metadata and native/web hosts publish separately in
[karty-sdk](https://github.com/karty-game/karty-sdk/releases) as `sdk-vVERSION`.
SDK tags do not trigger CLI publication; SDK, CLI and module versions may differ.

## SDK pins

`CurrentSDK` in [internal/release/version.go](../internal/release/version.go) is the
single SDK selection for CLI defaults, samples and tests. Update it once, then
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

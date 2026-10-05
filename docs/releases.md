# CLI releases

CLI archives/checksums publish from `karty` tags `vVERSION`. SDK bundles, signed
metadata and native/web hosts publish separately in
[karty-sdk](https://github.com/karty-game/karty-sdk/releases) as `sdk-vVERSION`.
SDK tags do not trigger CLI publication; SDK, CLI and module versions may differ.

## One SDK pin

`CurrentSDK` in [internal/release/version.go](../internal/release/version.go) is
authoritative for defaults, stable samples, smoke tests and release integration.
All samples, including world-camera, select the released SDK 0.0.8.
Change it only after compatible immutable public SDK/host assets are available.
Public format/compiler module dependencies are selected separately in [go.mod](../go.mod).

From the repository root, prepare and validate the release:

```sh
mise run prepare-release
mise run test
mise run build
mise run lint
mise run install-browser
mise run check-integration
```

Preparation updates sample SDK pins; tests reject drift and prepare the external
SDK cache. `karty sdk current` prints the selection for scripts. Preparation
does not commit, tag or publish. See [Development](development.md) for overrides.

## Publication gate

The [release workflow](../.github/workflows/release.yml) runs tests/lint and the
full customer suite on Linux amd64/ARM64 before GoReleaser publishes CLI archives
for Linux amd64/arm64, macOS arm64 and Windows amd64. It uses public dependencies
and the repository's `GITHUB_TOKEN`, not private engine source or credentials.
Record compilation, actual WASM execution, browser graphics and manual review
separately; a startup smoke is not gameplay or pixel validation.

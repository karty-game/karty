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
Record compilation, actual WASM execution, browser graphics and manual review
separately; a startup smoke is not gameplay or pixel validation.

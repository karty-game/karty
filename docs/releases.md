# CLI and SDK releases

CLI and engine artifacts have separate public release homes:

| Repository / tag | Published by | Assets |
| --- | --- | --- |
| `karty` / `vVERSION` | This repository's Release CLI workflow | Platform CLI archives and checksums |
| `karty-sdk` / `sdk-vVERSION` | Private engine release workflow | Native hosts, browser WASM, SDK ZIP and signed manifest |

CLI tags run tests and lint, install the public SDK selected by `KARTY_TEST_SDK`
in the release workflow, then run the full customer integration suite. Only after
that gate passes does GoReleaser publish the CLI. The public workflow uses
its repository's `GITHUB_TOKEN`; it has no engine access. GoReleaser builds the
CLI for Linux amd64/arm64, macOS arm64, and Windows amd64. The full customer
integration suite runs on native Linux amd64 and ARM64 runners before publishing.

`karty sdk install VERSION` downloads signed public metadata, verifies the bundle,
and installs it atomically. Projects pin an SDK in `karty.toml`; cached builds
do not need engine source or GitHub authentication. SDK/CLI versions need not
match. SDK tags do not trigger the CLI release workflow.

Before the first split SDK release, the embedded 0.0.1 bundle supports builds
with explicit locally built host paths. It is not a published host release.
See [SDK format and installation](sdk-bundles.md) for details.

Initial module publication order: KartUI and Karty SDK `v0.0.1`, then Karty `v0.0.1`.
Resolve and commit the public dependency checksums before tagging dependent
repositories. These initial module tags have not been published by this migration;
local ignored workspaces currently supply the dependencies.

The four-platform SDK starts at `sdk-v0.0.4`. Publish engine `v0.0.4` first, then
release the CLI that selects SDK 0.0.4. Older SDK releases do not include Linux
ARM64 host/tool metadata. macOS amd64 and Windows arm64 are not CLI release targets. Windows arm64 is
a game host target, validated by a separate native startup job without TinyGo.

## SDK 0.0.6 integration

The CLI defaults, samples, and release integration gate select the published
SDK 0.0.6 bundle and hosts. The Go module dependency is separately pinned to
`karty-sdk v0.0.7`, which supplies the directed portal world format.
These two version numbers identify different releases and need not match.

SDK 0.0.6 includes the image/audio/video pipeline, perspective and isometric
cameras, and directed portals with host-authoritative transform updates.
Projects on older SDKs retain their pinned behavior. See
[Image and sound assets](assets.md) for the authoring surface.

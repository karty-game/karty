# Karty

Public CLI and samples for Karty. Remote: `karty-game/karty`; this workspace
uses the directory name `karty-cli` to distinguish the original monorepo.

```sh
mise install
mise run test
mise run build
./dist/karty new my-game
```

The CLI builds/tests independently of the private engine using a public KartUI module
and an embedded SDK bootstrap archive. Project bindings are copied
from the selected SDK; the CLI does not generate the engine protocol.

Install a published SDK with `karty sdk install VERSION`. To inspect a locally
built candidate, use `karty sdk install --archive sdk-VERSION.zip --sha256 HASH VERSION`.
`KARTY_HOME` selects the SDK cache (default `~/.karty/sdks`). Existing toolchain
and host caches retain their original `~/.karty` location.

Scaffolded `.karty/docs` is the exact SDK-matched authoring reference.
See [the SDK format](sdk-bundles.md), [contribution boundaries](repository-split.md)
and [sample instructions](../samples/README.md).

Customer-facing integration checks belong here. Install the pinned published SDK
with `karty sdk install VERSION`, set `KARTY_TEST_SDK=VERSION`, install browser
dependencies with `mise run install-browser`, then run `mise run check-integration`.
The suite covers native cartridge loading, actual browser execution, UI input and
lifecycle, guest allocations, and development rebuilds. Linux native checks need
Xvfb and graphics libraries.

For local artifact testing, optionally set `KARTY_HOST_NATIVE` and `KARTY_HOST_WEB`
to prebuilt files. A web host override must have its matching `wasm_exec.js`
beside it. No engine checkout or private test executable is used. Public
unit tests require none of these artifacts.

Before the initial public module tags are published, connect local sibling
checkouts with an ignored workspace (run once from this repository root):

```sh
go work init .
go work edit -replace=github.com/karty-game/karty-ui=../karty-ui
go work edit -replace=github.com/karty-game/karty-sdk=../karty-sdk
```

The local workspace is already configured in this migration workspace. After
publication, standalone builds resolve the versions declared in `go.mod`.

`karty dev` resolves managed host and `wasm_exec.js` artifacts from the project's
pinned SDK on each rebuild. Only explicit or discovered local host overrides
are forwarded as `--host`; those require a matching sibling `wasm_exec.js`.

Image and sound authoring, transforms, the project cache, and SDK 0.0.4
candidate behavior are described in [Image and sound assets](assets.md).
`karty dev` watches supported image and WAV sources and uses the same processed
bytes as `karty build`.

# SDK bundle format 1

An SDK is selected by its exact manifest version. `sdk-VERSION.zip` contains:

```text
manifest.toml
bundle.json
bindings/go/engine/{game,components,protocol,renderers}.go
bindings/tinygo/engine/{game,components,protocol,renderers}.go
templates/game/VERSION/...
templates/ui/VERSION/...
docs/VERSION/...
```

Template/docs paths follow their manifest-selected versions, which need not
equal the SDK version. `bundle.json` contains `format: 1`, `ui_schema: 9`, and
`project_codegen: 1`. The CLI rejects unsupported values. These fields express
compatibility, not independent package release versions. The browser launcher
is CLI-owned and must be tested against every candidate host before release.

The engine produces ZIPs deterministically with sorted paths and stable headers.
No WIT or host implementation is shipped in the bundle. Generated Go is created
by the engine generator, then copied without regeneration into projects.
Project-specific asset references and UI code remain public build-time work.

The public release location is
`https://github.com/karty-game/karty-sdk/releases/download/sdk-vVERSION/`.
It contains `sdk-VERSION.toml`, its raw detached Ed25519 `.sig`, the SDK ZIP,
and native/web hosts plus the matching `wasm_exec.js`. The signed manifest adds a `[bundle]` table with `url`
and lowercase hex `sha256`; it also identifies checksummed host artifacts.
The ZIP's manifest matches this signed manifest except for the bundle table
(to avoid a checksum cycle). Only the existing embedded public verifier is
trusted. Do not introduce a signing private key into any repository.

`karty sdk install VERSION` verifies the signature before following the bundle
URL, verifies SHA-256 and matching metadata, checks archive bounds and paths,
and atomically installs beneath `~/.karty/sdks/VERSION`. `KARTY_HOME` can select
a different SDK cache. Existing versions cannot be replaced with different
bytes. Cache contents are rehashed on use. ZIPs are read as filesystems rather
than extracted, so archive symlinks cannot become host filesystem paths.
Limits: 32 MiB compressed/expanded, 2,048 regular entries, no duplicate paths,
absolute paths, traversal, symlinks or backslashes.

Local candidates use `karty sdk install --archive FILE --sha256 HASH VERSION`.
That explicitly supplied checksum is the trust input for a local install;
it does not claim release-signature verification. Public fork tests use the
checked-in generated bootstrap and synthetic/host test fixtures without keys.
The embedded 0.0.1 bootstrap does not include downloadable host metadata; pass
candidate host paths until an actual signed public release exists.

## Public module dependencies

KartUI and Karty formats are consumed as Go modules. Local development can use
ignored `go.work` files to select sibling public checkouts. Engine release CI
uses workspaces for the selected SDK/KartUI dependency revisions without copying
source. See [development](development.md) for initial-tag prerequisites.

## Preparing signed release assets (engine)

After engine checks succeed, collect raw host files named
`karty-host-linux-amd64`, `karty-host-darwin-arm64`, `karty-host-windows-amd64`,
`karty-host-web.wasm`, and `wasm_exec.js` in a directory. With the existing signing key supplied
securely in the environment, run:

```sh
bash scripts/prepare-release.sh VERSION /path/to/hosts /path/to/output
```

This computes host checksums locally, puts that metadata in the SDK ZIP,
then adds the ZIP checksum to the detached signed manifest. It does not
publish assets. Upload the prepared manifest, signature, ZIP and hosts to the
immutable `sdk-vVERSION` public release only after review and authorization.
The engine release workflow validates its own contracts and runtime before
publishing. The CLI release workflow runs customer integration checks against
those public artifacts before publishing CLI tags.

The signed `[artifacts.host.web-runtime]` entry pins the URL and SHA-256 of
`wasm_exec.js`. It comes from the Go toolchain that compiled the web host and
includes the Go license notice. The CLI downloads it with the selected SDK and
stages it with the host; it never substitutes a local Go installation's script.
Local `--host` overrides require matching `wasm_exec.js` in the same directory.
Older releases without this metadata need a new SDK release before automatic
web staging can use this flow; published assets must not be overwritten.

Release manifests also include `[artifacts.host.web-optimized]` for
`karty-host-web.optimized.wasm` and `[artifacts.host.web-brotli]` for
`karty-host-web.optimized.wasm.br`. Both have independent SHA-256 hashes;
the compressed hash covers the compressed bytes. These are optional production
choices for a future CLI build mode; current CLI builds still use the regular
web host. Serve Brotli bytes with `Content-Encoding: br` and
`Content-Type: application/wasm`; do not pass compressed bytes directly to the
WebAssembly API. All variants share the engine-produced `wasm_exec.js`.

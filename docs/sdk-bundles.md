# SDK bundle format 1

Projects select an exact SDK version. Its ZIP contains:

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
equal the SDK version. Compatibility fields are `format: 1`, `ui_schema: 9`
and `project_codegen: 1`; unsupported fields/processors fail early. The minimum
SDK is 0.0.5. Generated bindings are copied, not regenerated; hosts are separate
artifacts. No private implementation or embedded fallback SDK is required.

## Install and trust

`karty sdk install VERSION` reads the public `karty-sdk` release `sdk-vVERSION`.
It verifies the detached Ed25519 manifest signature with the embedded public
verifier before following the bundle URL, then checks SHA-256 and metadata.
The ZIP manifest matches signed metadata except for `[bundle]`, which carries
the URL/checksum. Native/web artifacts have their own checksums.

Installation is atomic beneath `~/.karty/sdks/VERSION`; `KARTY_HOME` changes the
SDK cache root. Existing versions cannot be replaced with different bytes.
Cached contents are rehashed on use; corruption is an error, not a fallback.
Do not track SDK ZIPs or signing keys in this repository.

Local candidates use `karty sdk install --archive FILE --sha256 HASH VERSION`.
The supplied checksum is their trust input, not a release-signature claim.

## Optional Materialize pin

SDK 0.0.8 selects the released Materialize 2.0.0 toolchain with
`tools.materialize`, a full lowercase `tools.materialize-revision`, and
`artifacts.materialize` records for darwin-arm64, linux-amd64, linux-arm64 and
windows-amd64. Each record requires the version-matched public karty-tools ZIP
URL, lowercase SHA-256, `format = 'zip'` and `executable = 'bin/materialize-cli'`
(`.exe` on Windows). The group is optional but partial/malformed pins are rejected.

The CLI defaults to released SDK 0.0.8; SDK 0.0.7 remains supported without a
Materialize pin. `karty toolchain install` installs the selected SDK's Materialize
binary and logs its executable path. World builds with `world/material-atlas@1`
invoke it to generate material maps; projects without world atlases do not.
Installation itself does not invoke a GPU, search PATH or build tool sources.

Managed installation verifies the full archive, confines extraction, preserves
notices and rechecks archive bytes and every installed file on warm use. Corrupt
caches fail closed; remove the damaged installation before retrying. Materialize
uses the tool cache under `~/.karty/tools/materialize-cli/REVISION/PLATFORM`.

## Bounds and web delivery

ZIPs are read as filesystems, not extracted: at most 32 MiB compressed/expanded
and 2,048 regular entries; no duplicates, absolute/traversal paths, symlinks or
backslashes. Compatibility and archive checks apply to every installed version.

`[artifacts.host.web-runtime]` pins the host-matched `wasm_exec.js` and Go notice;
never substitute a local Go script. Web `--host` overrides need it beside the WASM.
Current builds use the regular web host. Optimized/Brotli selection is
[proposed](proposals.md); Brotli delivery needs `Content-Encoding: br` and
`Content-Type: application/wasm`, not compressed bytes passed to WebAssembly.

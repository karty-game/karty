# Local host bundles

An explicit local WebGPU host override selects its matching build path:

```sh
karty build --target web --host /path/to/web-bundle
karty dev --host /path/to/web-bundle
karty build --target native --platform linux-amd64 --host /path/to/linux-amd64-bundle
karty render screenshot levels.room --host /path/to/native-bundle
karty render reflections levels.room --host /path/to/native-bundle
```

The bundle directory contains `host-bundle.toml`, its executable or WASM entry,
and the runtime dependencies listed by that descriptor. Selecting the entry file
instead of the directory also works. Bundles are an explicit qualification path;
ordinary builds continue to use their selected SDK's released host and runtime.
A file override without a sibling descriptor is rejected.

## Descriptor version 1

```toml
schema = 1
renderer = "webgpu"
target = "web"
entrypoint = "karty-host.wasm"

[files."karty-host.wasm"]
size = 1234
sha256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
```

Every dependency needs a `files` entry containing its exact size and lowercase
SHA-256. A web bundle declares `karty-host.wasm`, its byte-identical
`runtime.bin` alias, `wasm_exec.js`, `client.mjs`, `runtime.mjs`, and
`bootstrap.mjs`, the texture-codec modules and their licenses. The CLI creates the game's HTML shell and keeps the complete
runtime in its existing offline-release inventory. The engine's bootstrap owns
guest startup, WebGPU setup, and live update hooks.

Native targets are `linux-amd64`, `linux-arm64`, `darwin-arm64`,
`windows-amd64`, and `windows-arm64`. Their engine entrypoint is
`karty-host-webgpu` (`.exe` on Windows). The CLI stages it as `karty-host`
(`.exe` on Windows), with an adjusted descriptor so later capture commands can
recognize the same runtime. These are the only accepted native entry names.
GPU facade, wgpu-native and SDL libraries stay adjacent to the executable.
Declared Windows DXC/WARP dependencies, their provenance, and `licenses/`
files are retained.

The CLI verifies all declared inputs before staging. Bundles reject unknown
descriptor fields, traversal, symlinks, colliding filenames, missing dependencies,
target mismatches, and changed bytes. Limits are 128 files, 256 MiB per file and
512 MiB in total. The existing browser offline-delivery limit remains 128 MiB
per response. Bundle assets cannot replace the game's cartridge or content.

The current CLI accepts only complete verified WebGPU host bundles; the old
CDN launcher is removed. Capture commands use the bundle's
offline material preparation and stage each root and verified streamed child
under `content/<sha256>.kld` before invoking the native host.

Capture commands default to `--backend hardware`. Use `--backend software` with
a validated WebGPU native bundle for [CPU-only capture tooling](../guide/software-capture.html).

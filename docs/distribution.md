# Build games for another platform

Developer tooling and game runtimes have independent platform support.

| Platform | CLI and developer tools | Game host |
| --- | --- | --- |
| Linux amd64 | Yes | Yes |
| Linux arm64 | Yes | Yes |
| macOS arm64 | Yes | Yes |
| Windows amd64 | Yes | Yes |
| Windows arm64 | No native TinyGo release | Yes |

From any supported development machine, select a native distribution target:

```sh
karty build --platform windows-arm64
karty build --platform linux-arm64
karty build --platform darwin-arm64
```

Install the project's pinned SDK first; see [SDK bundles](sdk-bundles.md).
The cartridge is compiled using tools for the developer's machine. The host is
selected from the SDK's native host map using the requested destination, then
downloaded with checksum verification. No destination compiler is needed.

Explicit destinations stage into separate directories:

```text
dist/native/windows-arm64/karty-host.exe
dist/native/windows-arm64/game.kart
dist/native/linux-arm64/karty-host
dist/native/linux-arm64/game.kart
```

Distribute the entire selected directory, including any level/content files,
alongside the engine release's `RUNTIME_LICENSE.md` and applicable SDK notices.
On Windows, launch `karty-host.exe --cartridge game.kart`; on Linux/macOS, use
`./karty-host --cartridge game.kart`. Players need neither the CLI nor TinyGo.

Without `--platform`, native builds keep using the current machine and stage
into `dist/native/`. `--target web` remains architecture-independent and rejects
`--platform`. Explicit `--host` overrides can be combined with `--platform`.
The CLI checks native hosts for the destination's executable format and CPU
before staging, including explicit overrides and cached hosts. macOS arm64
requires a Mach-O arm64 executable; a browser `.wasm` host cannot run natively.
If a build rejects a host, supply the matching native host with `--host` or
replace the incorrect cached artifact from the selected SDK release.

Linux integration covers amd64/ARM64. Windows ARM64 cartridge/startup smoke
detects early process failure, not pixels or gameplay; renderer changes still
need graphics testing on relevant hardware.

# Build games for another platform

Developer tooling and game runtimes have independent platform support.

| Platform | CLI and developer tools | Game host |
| --- | --- | --- |
| Linux amd64 | Yes | Yes |
| Linux arm64 | SDK 0.0.4+ | SDK 0.0.4+ |
| macOS arm64 | Yes | Yes |
| Windows amd64 | Yes | Yes |
| Windows arm64 | No native TinyGo release | SDK 0.0.4+ |

From any supported development machine, select a native distribution target:

```sh
karty sdk install 0.0.4
# Set [sdk].version = "0.0.4" in karty.toml first.
karty build --platform windows-arm64
karty build --platform linux-arm64
```

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
`--platform`. Explicit `--host` overrides can be combined with `--platform`;
the caller is responsible for supplying a host matching that destination.

The release gate runs Linux integration on amd64/ARM64 and a Windows ARM64
cartridge validation/startup smoke. The Windows smoke detects early process
failure; it does not assert pixels or gameplay. Hardware graphics testing remains
necessary when assessing renderer changes.

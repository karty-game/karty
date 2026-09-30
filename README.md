# Karty

**Build and run games with Go, WebAssembly, and KartUI.**

Karty is a game development toolkit built on **Ebiten (Ebitengine)** and
**EbitenUI**, designed to make building games in Go easier and more accessible.
It brings project setup, game builds, declarative interfaces, and native/browser
runtimes together behind one command-line tool.

- **Create** a game from a ready-to-use template.
- **Build and develop** with managed tools, automatic rebuilds, and cached image/audio processing.
- **Learn** from Pong, the KartUI demo, and the media pipeline lab in `samples/`.

The CLI downloads versioned SDK and host artifacts from
[Karty SDK](https://github.com/karty-game/karty-sdk). Contributors do not need
access to the private engine repository.

```sh
mise run test
mise run build
./dist/karty new my-game
```

[Assets](docs/assets.md) · [Development](docs/development.md) · [Samples](samples/README.md) · [Releases](docs/releases.md)

Part of **Karty**: [KartUI](https://github.com/karty-game/karty-ui) provides the
language and editor tools; the private Karty Engine provides the runtime.

[Contributing](CONTRIBUTING.md) · [MIT license](LICENSE.md)

The MIT license covers the CLI source. Engine runtime binaries use the separate
[Karty Runtime License](https://github.com/karty-game/karty-sdk/blob/main/RUNTIME_LICENSE.md),
which permits distribution with free and commercial games.

CLI development platforms: **Linux amd64 and arm64, macOS arm64, Windows amd64**.
Linux ARM64 requires SDK **0.0.4** or newer. Browser builds share the same WASM host.

Game distribution additionally supports **Windows arm64** with SDK 0.0.4.
Use `karty build --platform windows-arm64` from any supported development machine.
See [cross-platform distribution](docs/distribution.md).

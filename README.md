# Karty

Build small games in Go, with a CLI for creating projects, previewing changes
and packaging releases.

[Play the demos](https://karty-game.github.io/karty/main/) ·
[Game guides](https://karty-game.github.io/karty/main/guide/) ·
[CLI releases](https://github.com/karty-game/karty/releases/latest)

- Make retro first-person games, isometric adventures, 2D games and [visual novels](samples/last-train/README.md).
- Build 3D rooms from floor plans, heights and reusable shapes.
- Package a WebAssembly cartridge for the browser, Linux, macOS or Windows.

Built on [Ebitengine](https://ebitengine.org/) and [EbitenUI](https://github.com/ebitenui/ebitenui).

> [!WARNING]
> Karty is under active development. APIs, project formats and features may change.

| First-person Hangar                                                                                                                       | Dimetric Vigil                                                                                                   |
| ----------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| [![The Hangar's lit reactor room, viewed at player height.](samples/hangar/preview.jpg)](https://karty-game.github.io/karty/main/hangar/) | [![A pilgrim at the candlelit altar of a dark abbey crypt.](samples/vigil/preview.jpg)](samples/vigil/README.md) |

## Start a game

Download the [CLI release](https://github.com/karty-game/karty/releases/latest)
for your development machine, extract it, and put `karty` on your `PATH`.
See [supported platforms](https://karty-game.github.io/karty/main/guide/share.html)
for developer tool requirements.

This checkout's starters and samples use [SDK 0.0.13](https://github.com/karty-game/karty-sdk/releases/tag/sdk-v0.0.13).
The CLI downloads the pinned SDK and matching hosts when needed. See
[contributor setup](CONTRIBUTING.md#local-setup) for working from source.

```sh
karty new my-game
cd my-game
karty dev
```

In a terminal, `new` offers blank, 2D, first-person, isometric and UI starters, plus
optional Git and GitHub Actions setup. Use `--yes --template 2d` for scripts.

Open the preview address printed by `karty dev`. The first setup downloads the
project's pinned SDK and tools; later builds reuse them. Save changes to update
the preview. Compatible lighting, material and global UI edits update live;
code and structural changes reload the game. Build errors leave the previous
game running. See [the setup guide](https://karty-game.github.io/karty/main/guide/start.html)
for details.

Press Ctrl+C to stop the preview, then package your game:

```sh
karty build --target web
karty build --target native
```

Share the complete `dist/web/` or `dist/native/` directory. Keep your source and
`karty.toml` in Git; the CLI recreates `.karty/` and `dist/`. Your project's
`.karty/docs/` contains the reference for its exact SDK.

## Explore Karty

| Looking for             | Start here                                                                                                                            |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| A working example       | [Sample projects](samples/README.md)                                                                                                  |
| Reusable game code      | [Controllers, dialogue, saves and Tiled maps](packages/README.md)                                                                     |
| API behavior and design | [API reference](https://karty-game.github.io/karty/main/technical/) · [Design notes](https://karty-game.github.io/karty/main/design/) |
| CLI development         | [Contributing and repository layout](CONTRIBUTING.md)                                                                                 |

[KartUI](https://github.com/karty-game/karty-ui) ·
[SDK](https://github.com/karty-game/karty-sdk) ·
[MIT source license](LICENSE.md) ·
[Runtime license](https://github.com/karty-game/karty-sdk/blob/main/RUNTIME_LICENSE.md)

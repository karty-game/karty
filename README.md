# Karty

Build retro-inspired 2D and 3D games.
Karty handles live reloading, asset preparation, and packaging for you.

[Getting Started](https://karty-game.github.io/karty/main/guide/) ·
[Play the demos](https://karty-game.github.io/karty/main/) ·
[Releases](https://github.com/karty-game/karty/releases/latest)

> [!WARNING]
> Karty is under active development. APIs, project formats and features may change.

## Features

- 🐹 **Your game, written in Go.** Build game logic with Go’s simplicity, speed and
  type safety, powered by [Ebitengine](https://ebitengine.org/) and
  [EbitenUI](https://github.com/ebitenui/ebitenui).
- 👾 **Retro looks. Modern workflow.** Create 2D and 3D games inspired by the ’90s
  and early 2000s, with tools built for today’s development workflow.
- ✨ **Give your game a signature look.** Shape the mood with built-in
  post-processing effects and presets, from CRT-inspired visuals to handheld
  palettes.
- 📦 **One cartridge. Web and desktop.** Package your game as a portable WebAssembly
  cartridge and run it with Karty’s hosts in browsers or on Linux, macOS and
  Windows.
- 🌐 **Less asset plumbing. More game.** Let the build pipeline prune unused assets,
  pack atlases and handle optimization, chunking and caching ahead of time.
- 🚀 **From terminal to playable.** Create projects, preview changes and package
  releases from the CLI, locally or in CI.

## Samples

| [FPS](https://karty-game.github.io/karty/main/hangar/)                                                                                                | [Isometric Adventure](https://karty-game.github.io/karty/main/vigil/)                                                                               |
| ----------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| [![The Hangar's lit reactor room, viewed at player height.](samples/hangar/preview.jpg)](https://karty-game.github.io/karty/main/hangar/)             | [![A pilgrim at the candlelit altar of a dark abbey crypt.](samples/vigil/preview.jpg)](https://karty-game.github.io/karty/main/vigil/)             |
| **[3D World Exploration](https://karty-game.github.io/karty/main/terrain/)**                                                                          | **[Visual Novel Dialogue](https://karty-game.github.io/karty/main/last-train/)**                                                                    |
| [![A sandy coastal cove with a wooden jetty, palms and a lighthouse.](samples/terrain/preview.jpg)](https://karty-game.github.io/karty/main/terrain/) | [![Mara aboard a warmly lit train carriage on a rainy night.](samples/last-train/preview.jpg)](https://karty-game.github.io/karty/main/last-train/) |

## CLI commands

Run `karty --help` for an overview, or append `--help` to any command for its
options.

| Command                    | Purpose                            |
| -------------------------- | ---------------------------------- |
| `karty new`                | Create a game project              |
| `karty dev`                | Preview your game as you edit      |
| `karty build`              | Build your game for web or desktop |
| `karty serve`              | Preview a built web game           |
| `karty fmt` / `karty lint` | Format and check your Go code      |
| `karty package`            | Share and reuse game packages      |
| `karty render bake`        | Bake level lighting                |
| `karty render screenshot`  | Save level screenshots as PNGs     |

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

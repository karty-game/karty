# Karty

**Start building Go games in minutes.**

- Make 2.5D retro shooters and isometric action RPGs.
- Create 3D world by stacking 2D shapes without learning external tools like Blender.
- Create, develop, preview, and package your game with one CLI.
- Build one kartridge and run it anywhere (on the web, Linux, macOS, or Windows).
- Built on [Ebitengine](https://ebitengine.org/) and [EbitenUI](https://github.com/ebitenui/ebitenui).

> [!WARNING]
> Karty is a work in progress under active development. APIs and features are
> subject to change.

| First-person Hangar                                                                                                                       | Isometric Roman court                                                                                                                                                    |
| ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [![The Hangar's lit reactor room, viewed at player height.](samples/hangar/preview.jpg)](https://karty-game.github.io/karty/main/hangar/) | [![The Roman court and adjoining gallery, viewed with an isometric camera.](internal/guide/assets/isometric.jpg)](https://karty-game.github.io/karty/main/world-camera/) |

## Start a game

Download the [CLI release](https://github.com/karty-game/karty/releases/latest)
for your development machine, extract it, and put `karty` on your `PATH`.
See [supported platforms](docs/distribution.md) for developer tool requirements.

```sh
karty new my-game
cd my-game
karty dev
```

Open the preview address printed by `karty dev`. The first setup downloads the
project's pinned SDK and tools; later builds reuse them. Press Ctrl+C to stop
previewing, then package your game:

```sh
karty build --target web
karty build --target native
```

Share the complete `dist/web/` or `dist/native/` directory. Keep your source and
`karty.toml` in Git; the CLI recreates `.karty/` and `dist/`. Your project's
`.karty/docs/` contains the reference for its exact SDK.

## Explore Karty

- [Play the demos](https://karty-game.github.io/karty/main/)
- [Read the game guide](https://karty-game.github.io/karty/main/guide/)
- [Browse sample code](samples/README.md)
- [CLI docs and reference](docs/README.md)
- [Supported platforms and publishing](docs/distribution.md)
- [KartUI](https://github.com/karty-game/karty-ui) and [SDK](https://github.com/karty-game/karty-sdk)
- [MIT source license](LICENSE.md) and [runtime license](https://github.com/karty-game/karty-sdk/blob/main/RUNTIME_LICENSE.md)

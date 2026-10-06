# Karty

<p align="center">
  <strong>Write the game, Karty handles the rest</strong><br>
  Go, KartUI, and one CLI for browser and desktop games.
</p>

Karty is an open source game development toolkit built around Go, WebAssembly,
and [KartUI](https://github.com/karty-game/karty-ui). The Karty CLI creates a
ready-to-build project and takes care of the toolchain, asset processing, and
game packaging, so you can focus on making the game.

Games run on the Karty runtime, available for native platforms and the browser.
The CLI downloads versioned SDK and host releases from
[Karty SDK](https://github.com/karty-game/karty-sdk), so you can build without
access to the private engine source.

## Documentation

See the [documentation index](docs/README.md),
[implemented behavior](docs/implemented.md), and
[proposals](docs/proposals.md).

## Get started

Install the [latest Karty CLI release](https://github.com/karty-game/karty/releases/latest),
then create and run a project:

```sh
karty new my-game
cd my-game
karty dev
```

`karty dev` builds and serves the game for the browser, then rebuilds as you
edit. To make a native desktop build instead, run:

```sh
karty build --target native
```

The first project setup downloads the SDK-pinned tools and runtime assets. See
the [development guide](docs/development.md) for setup details and the
[distribution guide](docs/distribution.md) for supported targets.

## Make it yours

|                            | What you can do                                                                                                           |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| **Build with Go**          | Write game logic in Go and use Karty's game runtime and typed APIs.                                                       |
| **Design with KartUI**     | Create game interfaces with a declarative UI language and reusable components.                                            |
| **Bring in assets**        | Package levels, textures, sounds, music, and video with your game. Image and audio processing is built into the pipeline. |
| **Develop in the browser** | Run a live web build that watches your source, UI, levels, and supported assets for changes.                              |
| **Ship across platforms**  | Build browser games and native desktop distributions. The CLI and game hosts have separate platform support.              |

## Explore the samples

Try the [browser demos](https://karty-game.github.io/karty/main/), then make them your own:

- [Orbital flight deck](samples/ui-demo) — choose a ship, tune its thrusters and manage your inventory.
- [Roman court & galleries](samples/world-camera) — walk through a lit 3D world or explore it from above.
- [Media lab](samples/media-lab) — mix effects over music and play a video in the same scene.
- [Sprite playground](samples/pong) — move a character among animated sprites, shapes and text.

See [how to build and run the samples](samples/README.md).

## Commands

```sh
karty new <name>          # Create a game project
karty dev                 # Build and serve a live browser preview
karty build               # Build a native game
karty build --target web  # Build a browser game
karty bake                # Bake static directional lighting and diffuse radiosity
karty schema              # Refresh world YAML editor completions
karty schema --check      # Validate world YAML without building assets
```

Run `karty --help` or `karty <command> --help` for options. The CLI also
includes commands to install an SDK and manage its pinned toolchain.

## License

The CLI source is available under the [MIT license](LICENSE.md). Runtime
binaries use the separate [Karty Runtime License](https://github.com/karty-game/karty-sdk/blob/main/RUNTIME_LICENSE.md),
which permits distribution with free and commercial games.

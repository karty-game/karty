# Getting Started

[Why Karty?](../design/why.html) · [How Karty works](../design/)

## Install the CLI

Download the [Karty CLI](https://github.com/karty-game/karty/releases/latest)
archive for your operating system and CPU, then extract it.

- On Linux or macOS, put `karty` in `~/.local/bin`. Create the folder if needed.
  Add `export PATH="$HOME/.local/bin:$PATH"` to your shell profile.
- On Windows, put `karty.exe` in a folder such as `C:\Karty`.
  Add that folder to your user `Path` in Environment Variables.

Open a new terminal and run `karty --help` to check the installation.

## Create a game

Create a project and start its browser preview:

```sh
karty new my-game
cd my-game
karty dev
```

Open the address printed by `karty dev`. The first setup downloads the SDK and
build tools. Later builds reuse them.

Your project pins an exact SDK. Keep `karty.toml`, Go source, and source assets
in Git. The CLI recreates `.karty/` and `dist/`. Use `.karty/docs/` for the API
reference that matches your project.

## Choose your game

Choose the [2D game](2d.html), [first-person game](fps.html), or
[isometric game](isometric.html) guide.

The 2D path covers sprites, vector shapes, and text on a flat canvas.
[Menus and layouts](ui.html) and [sound and video](media.html) work with every game path.

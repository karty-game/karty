# Build and preview

Start with the latest Karty CLI, create a project, and open its live preview:

```sh
karty new my-game
cd my-game
karty dev
```

Choose a starter in the terminal wizard, then open the address printed by
`karty dev`. The first setup prepares generated game bindings and downloads
the build tools and runtime files Karty manages. Later builds reuse verified
downloads and cached assets. If a prerequisite is missing, follow the message
printed in the terminal.

## Edit and try again

Keep game rules in `src/`, interfaces in `ui/`, and world content in `levels/`.
Change a movement speed, a color or a room boundary, save, and try the preview
again. Karty watches those sources, rebuilds what changed, and updates the game
or reloads it when needed.

The development preview favors fast iteration. It uses Go for game code and
reuses matching processed assets. A release build applies the project's normal
build settings, including its compiler and lighting configuration.

Use the [game guides](../guide/index.html) for complete examples. The generated
`.karty/docs/` directory provides the game API and authoring reference.

## Package a game

Build for your current desktop platform:

```sh
karty build
```

Or create a browser distribution:

```sh
karty build --target web
```

The result appears in `dist/`. Share the complete target directory, including
its runtime, levels and media. Players need those packaged files; they do not
need the CLI or your build tools.

See [Build and share](../guide/share.html) for hosting, other desktop platforms
and continuous integration.

## Keep authored files

Commit `karty.toml`, `go.mod`, `go.sum` when present, and the code and assets you
edit. Karty recreates `.karty/` and `dist/`; leave those generated directories
out of version control. Package declarations record the exact reusable content
your project uses, so another checkout can restore it.

Continue with the [asset pipeline](asset-pipeline.html) to see how source files
become the textures, levels and media loaded by the game.

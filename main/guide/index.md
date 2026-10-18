# Choose your game

[Why Karty?](../design/why.html) · [How Karty works](../design/)

Create a project and open its browser preview:

```sh
karty new my-game
cd my-game
karty dev
```

Install the [Karty CLI](https://github.com/karty-game/karty/releases/latest)
first if needed. The first setup downloads tools. Later builds reuse them.
Open the address printed by `karty dev`, then choose the kind of game you want
to write below.

Your project pins an exact SDK. Keep `karty.toml`, Go source, and source assets
in Git. The CLI recreates `.karty/` and `dist/`. Use `.karty/docs/` for the API
reference that matches your project.

Choose the [2D game](2d.html), [first-person game](fps.html), or
[isometric game](isometric.html) guide.

The 2D path covers sprites, vector shapes, and text on a flat canvas.
[Menus and layouts](ui.html) and [sound and video](media.html) work with every game path.

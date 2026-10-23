# Willowbrook

A sunny village treasure hunt: follow the cottage note, explore the garden and
find the hidden cache across the bridge. It shows a 2D game built with Tiled
maps, animated scenery, click-to-move navigation and a following camera.

![Willowbrook's village and river.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/willowbrook/) · [All samples](../README.md)

## Try it

Use **WASD / arrows** to walk and **Shift** to run, or **click / tap** your
destination. Tap a note or cache to approach it, or press **E / Space** nearby.
**R** restarts the hunt. Open **Camera settings** to try different follow behavior.

## Built with

- [Tiled](https://karty-game.github.io/karty-packages/packages/tiled.html): loading maps and displaying animated scenery.
- [Top-down Controllers](https://karty-game.github.io/karty-packages/packages/controllers-topdown.html): movement, pointer targeting and camera follow.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
cd samples/willowbrook
../../dist/karty dev
```

Explore [the game](src/main.go), [quest](src/quest.go) and [Tiled map](internal/village/village.tmj).

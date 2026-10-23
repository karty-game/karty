# Crate room

A physics playground full of stacked wooden crates. Shoot, push and blast them
around to see collisions, bouncing and tumbling in action, with desktop and
touch controls.

![Crate stacks surrounded by ramps and red bumpers.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/crate-room/) · [All samples](../README.md)

## Try it

Use **WASD** to move and **Shift** to run. Click to capture the mouse and look
around; **Escape** releases it. **Click / Space** shoots, **B / right-click**
blasts, and **R** resets the crates. On touch screens, use the movement, look
and weapon controls.

## Built with

- [FPS Kit](https://karty-game.github.io/karty-packages/packages/fps-kit.html): crate models, room materials and bumpers.
- [3D Controllers](https://karty-game.github.io/karty-packages/packages/controllers-3d.html): first-person movement, mouse look and touch controls.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
cd samples/crate-room
../../dist/karty dev
```

Explore [the game](src/main.go), [crates](src/crates.go) and [weapons](src/gun.go).

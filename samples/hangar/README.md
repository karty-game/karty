# Hangar

Explore an industrial complex of tunnels, machinery and glowing acid pools.
The sample brings together first-person movement, physics, lighting, particles
and spatial audio in a connected 3D level.

![Green-lit machinery inside the Hangar.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/hangar/) · [All samples](../README.md)

## Try it

Use **WASD** to walk and **Shift** to run. Click to look with the mouse;
**Escape** releases it. **F** toggles the torch. On touch screens, use the two
joysticks. Push crates, ride the elevator and use **Camera** to jump to landmarks.

## Built with

- [3D Controllers](https://karty-game.github.io/karty-packages/packages/controllers-3d.html): first-person movement, mouse look and touch controls.
- [Go YAML](https://github.com/yaml/go-yaml): prepares particle effects from their recipes.
- [go-toml](https://github.com/pelletier/go-toml): reads level asset lists when preparing effects.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
cd samples/hangar
../../dist/karty dev
```

Explore [the game](src/main.go), [scene](src/scene.go) and [level](levels/hangar/world.yaml).

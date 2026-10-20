# Sprites and particles

Sprites draw image assets. The host simulates authored particles.

## Sprites

`Sprite2D` draws on the canvas or in a scrolling 2D world.
`Sprite3D` occupies a position in a mounted 3D level.

| API                                  | Use                                                   |
| ------------------------------------ | ----------------------------------------------------- |
| `NewSprite2D(textureID, position)`   | Creates a sprite from a project texture               |
| `SetTexture(textureID)`              | Changes its project image                             |
| `SetLevelTexture(asset)`             | Changes its level texture                             |
| `NewSprite3D(asset, pose, settings)` | Creates a sprite in a 3D level                        |
| `SetSprite(settings)`                | Replaces a world sprite's settings, including `Asset` |

### World sprite settings

`textureID` below is the numeric ID from the ready level's metadata:

```go
func worldHero(game *engine.Game, level engine.LevelHandle, textureID uint32) *engine.Sprite3D {
    return game.NewSprite3D(
        engine.LevelTexture(level, textureID),
        engine.NewTransform3D(engine.NewVec3(4, 4, 0)),
        engine.SpriteRenderer3D{
            Facing: engine.WorldSpriteFacingUpright,
            AlphaMode: engine.WorldSpriteAlphaCutout,
            Size: engine.NewVec2(0.8, 1.6),
            Anchor: engine.NewVec2(0.5, 1),
        },
    )
}
```

`Size` uses world units. Normalized origin `(0.5, 1)` anchors the bottom centre.
Upright facing keeps characters vertical. Cutout preserves solid silhouettes;
blend mode allows soft transparency.

2D sprites use source-image pixels before scaling. Keep frame dimensions and
origins consistent during animation. Destroy unused sprites, and discard level
texture references when their level is released.

[Image setup](../guide/2d.html) · [Willowbrook animation](https://github.com/karty-game/karty/blob/main/samples/willowbrook/src/tiled.go) · [Vigil world sprites](https://github.com/karty-game/karty/blob/main/samples/vigil/src/main.go)

## Particles

The host simulates authored emitters on the mounted level's clock. Presets
define particle behavior; emitters place and schedule a preset.

### Preset and placement

This `world.yaml` section requires `world/particles@2`. `spark` is a declared
sprite texture, such as this [particle mask](assets/particle-spark.png).

```yaml
particles:
  version: 2
  presets:
    - name: sparks
      capacity: 16
      emission: {rate: 8, duration_seconds: 1}
      lifetime_seconds: {min: 0.4, max: 0.8}
      spawn: {shape: point}
      velocity_min: {x: -0.2, y: -0.2, z: 0.5}
      velocity_max: {x: 0.2, y: 0.2, z: 1}
      acceleration: {z: -0.5}
      sprite:
        frames: [spark]
        playback: cycle
        facing: camera-facing
        blend: additive
      size_over_life: [{age: 0, value: 0.12}, {age: 1, value: 0.02}]
      color_over_life:
        - {age: 0, rgba: [1, 0.7, 0.2, 1]}
        - {age: 1, rgba: [1, 0.2, 0, 0]}
  emitters:
    - id: vent
      preset: sparks
      position: {x: 4, y: 4, z: 1}
      seed: 42
      playback: {mode: cycle, period_seconds: 2}
```

| Field                               | Meaning                                                                    |
| ----------------------------------- | -------------------------------------------------------------------------- |
| `capacity`                          | Available particle slots, validated against lifetime and emission schedule |
| `size_over_life`, `color_over_life` | Curves with age zero at birth and one at death                             |
| `sprite.playback`                   | Image-frame playback within each particle                                  |
| Emitter `playback`                  | When the emitter creates particles                                         |
| `seed`                              | Repeatable variation                                                       |

Additive blending suits luminous sparks. Version 2 also supports alpha and
premultiplied modes. All cameras see the same effect age. Level release removes
effects; remounting restarts them. There is no per-particle Go update callback.

[Particle lab recipes and setup](https://github.com/karty-game/karty/blob/main/samples/particle-lab/README.md) · [Hangar placement](https://github.com/karty-game/karty/blob/main/samples/hangar/README.md)

Exact fields and budgets are in `.karty/docs/particles.md`.

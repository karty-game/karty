# Vigil — the abbey below

Explore a candlelit abbey crypt and rekindle three vigil lights: remembrance,
watchfulness and hope. Vigil combines 2:1 dimetric world geometry, a pre-rendered
pilgrim, click-to-move navigation and spatial audio.

![The pilgrim in the candlelit abbey crypt.](preview.jpg)

[All samples](../README.md) · [Checks](validation.md) · [Art sources](../../scripts/vigil/assets/README.md)

## Play

Complete the [sample setup](../README.md#run-a-sample), then run from the repository root:

```sh
mise run build
cd samples/vigil
../../dist/karty dev
```

Open the preview address printed by the CLI. SDK 0.0.16 and its matching public
host supply navigation, sprites and foreground occluder fading. For packaging,
run `mise run build-vigil-web` or `mise run build-vigil-native` from the repository
root. Run those builds sequentially because they share generated UI staging.

| Input                          | Action                                                  |
| ------------------------------ | ------------------------------------------------------- |
| Click / tap floor              | Walk around obstacles to the nearest accessible point   |
| WASD / arrows                  | Move in screen directions; diagonal speed is normalized |
| Shift                          | Hasten                                                  |
| F / Fireball button            | Throw a fireball in the current facing direction        |
| E / Space / interaction button | Rekindle a nearby vigil light                           |
| Tap nearby vigil light         | Rekindle it                                             |
| + / − / on-screen buttons      | Zoom                                                    |
| H / ?                          | Toggle help                                             |
| Escape / focus loss            | Stop movement and clear held input                      |
| R                              | Return to the entrance and reset the lights             |

## Explore the project

- `internal/dungeon/` defines the cruciform footprint, blocking furniture,
  clearance, sight and shrine positions.
- `src/` owns gameplay, the SDK adapter and waypoint following. The shared
  [isometric controller](../../packages/controllers/README.md) handles input and
  camera calculations; the engine supplies clearance-tested A* routes.
- `levels/crypt/` contains the authored world and runtime textures. Furniture
  uses depth-writing 3D solids; the pilgrim and small devotional accents use
  upright sprites. Walls conceal inaccessible rooms while low furniture blocks
  movement without hiding them.
- The [authoring sources](../../scripts/vigil/assets/README.md) retain original
  environment sheets and the selected character atlases outside the playable project.
  The playable character has 24 complete-body phases in eight directions plus
  eight idle frames. Animation follows accepted travel rather than elapsed time.
- `assets/sounds/` contains the sample's original synthesized ambience and effects;
  see [sound authoring](assets/sounds/README.md).

Fireballs demonstrate moving lights and impact effects, with four bounded
projectiles and no damage system. There are no enemies, combat or inventory.
The listener follows the accepted player position and uses the camera's right
axis for stereo placement. Browser audio starts after an interaction.

## Authoring and animation review

From the repository root:

```sh
mise run author-vigil
mise run check-vigil
mise run review-vigil-walk
```

The author reads the retained environment sheets and `scripts/vigil/assets/pilgrim-walk.png` /
`scripts/vigil/assets/pilgrim-idle.png`. It produces one set of 200 compact runtime frame aliases
in `levels/crypt/`, with their order recorded in `levels/crypt/walk.generated.json`.
That generated validation metadata stays with the frames for coherent project
snapshots; it is not a packaged game asset.
Regenerate texture IDs and sight silhouettes through this author; do not edit
`src/textures.generated.go` or `internal/dungeon/vision_art.generated.go` by hand.
Ordinary authoring needs no Blender. The [model source workflow](../../scripts/vigil/source/README.md)
contains credits, licenses and instructions for changing the character render.

The walk preview exports `dist/validation/vigil/animation/index.html` with pause,
scrubbing, normal speed and haste controls. It displays the selected current
artwork. An explicit `--baseline=/path/to/previous-art-export` adds a comparison
using that export's `walk.generated.json` and referenced PNGs. A preview export
supports manual review; [browser checks](validation.md) separately exercise
actual WASM gameplay and rendering.

See [design notes](design.md) for materials, visibility and presentation choices.

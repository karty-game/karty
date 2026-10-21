# Karty samples

[Play the demos](https://karty-game.github.io/karty/main/) or choose a project
below to explore its source, controls and authoring tools.

| Sample                                          | What it demonstrates                                                            |
| ----------------------------------------------- | ------------------------------------------------------------------------------- |
| [Hangar](hangar/README.md)                      | First-person movement, a portal loop, physics, baked lighting and spatial audio |
| [Physics lab: crate room](crate-room/README.md) | Shooting and pushing physics crates, with desktop and touch controls            |
| [Terrain: coastal cove](terrain/README.md)      | Imported terrain, coastal vegetation and static structures                      |
| [Vigil](vigil/README.md)                        | Dimetric exploration, pathfinding, animated sprites and occluder fading         |
| [Willowbrook](willowbrook/README.md)            | A 2D treasure hunt with Tiled maps, following camera and touch controls         |
| [Last Train](last-train/README.md)              | Yarn Spinner dialogue, three endings, persistent saves and reusable UI          |
| [Orbital flight deck](ui-demo/README.md)        | KartUI menus, tabs, settings, tooltips and inventory                            |
| [Media lab](media-lab/README.md)                | Images, overlapping sound effects, looping music and video playback             |
| [Retro effect lab](effect-lab/README.md)        | FPS material/light room and development-host effect comparisons                 |
| [Particle lab](particle-lab/README.md)          | Reusable fire and smoke recipes                                                 |

## Run a sample

These sources target released SDK 0.0.16 and its matching hosts, including support
for their build-time image atlases.
The CLI installs the pinned SDK and matching hosts as needed. See
[contributor setup](../CONTRIBUTING.md#local-setup) for toolchain and cache settings.

From the repository root:

```sh
mise install
mise run build
cd samples/ui-demo
../../dist/karty dev
```

Open the address printed in your terminal. Edit and save to rebuild the preview.
Replace `ui-demo` with another sample's folder name. Check its README first:
Last Train needs the Yarn compiler, and some samples have asset-authoring or
light-baking steps.

To package a sample, run `karty build --target web` or
`karty build --target native` from its directory, using the CLI you built above.
Keep the complete output directory when sharing a game.

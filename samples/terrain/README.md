# Terrain: coastal cove

Explore a tropical beach, follow trails to a cliff bunker and lighthouse, or
swim beneath the ocean. This sample shows outdoor terrain, vegetation,
first-person exploration and underwater effects.

![Palms beside the cove, bunker and wooden jetty.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/terrain/) · [All samples](../README.md)

## Try it

Use **WASD** to walk and **Shift** to run. Click to look with the mouse;
**Escape** releases it. **Space / C** rises or dives while swimming. On touch
screens, use the two joysticks. **V** cycles landmarks and **R** returns to the beach.

## Built with

- [3D Controllers](https://karty-game.github.io/karty-packages/packages/controllers-3d.html): first-person movement, mouse look and touch controls.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
cd samples/terrain
../../dist/karty dev
```

Explore [the game](src/main.go), [explorer](src/explorer.go) and [landscape](levels/ground/world.yaml).

## Fixed lighting candidate

This sample targets SDK **0.0.21** and enables `fixed = true` in its chunked
offline lightmap recipe. SDK 0.0.21 is currently a local release candidate;
install its checksummed bundle before building. Use a matching candidate host
with `karty build --host /path/to/host/webgpu --target web`.

Sun, sky and static surfaces share one fixed bake. Geometry, terrain lighting
and vegetation probes stream in bounded chunks. Connected sectors share a page;
the chunk recipe bounds its mesh and cropped terrain data at 8 MiB and its
encoded child module at 16 MiB.
Terrain retains per-vertex directional lighting. The bake uses 64 transport
samples and two diffuse bounces to improve shaded recesses. Static solid
lightmaps use a coarse density of 0.125 texels per metre so the distant ocean
fits one page. This also affects the bunker; small surfaces still receive at
least two texels per axis.

`karty build` produces the required chunk bakes. Rebake after changing the
authored geometry, materials or static lighting.

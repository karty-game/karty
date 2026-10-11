# 🏛️ Roman court & galleries

**A world to walk through. A different way to see it.**

Walk between marble columns, circle a mosaic-lined basin and explore sloping
brick galleries. Then switch from first person to isometric and see the whole space.
Karty brings the textures, lighting and characters together in one connected 3D world.

**[▶ Explore the court](https://karty-game.github.io/karty/main/world-camera/)** · [More demos](../README.md)

## Take a look around

- **Roman court / Gallery:** jump between the courtyard and the galleries.
- **FPS / Isometric:** walk through the world or orbit above it.
- **On-screen arrows:** move and look. **■** stops motion.
- **Gallery → Full:** stop and watch the warm moving light sweep across the walls.
- **Albedo / Normal / Depth:** peek behind the final image to see what makes it work.

On a keyboard, **W/S** moves forward/back, **A/D** strafes in first person or
orbits in isometric, and **Q/E** turns. Prefer touch? Use the on-screen controls.
Tap the scene or press **C** to bring hidden controls back.

## Run it

[Set up once](../README.md#make-it-yours), then from this sample's folder,
build its chunk lightmaps and open the preview:

```sh
../../dist/karty dev
```

The build creates six authored areas: the complete courtyard, hall, three
galleries, and north turn. Each contains its own prebuilt mesh and baked lightmap;
the shared textures and complete portal graph remain in the root level. Rebuild
after changing the world, its materials or its static lights. Area hints can
split when their mesh or lightmap exceeds the configured budget; see
[chunk settings](../../docs/chunk-levels.md).

The courtyard uses a 1024 square lightmap and the five smaller areas use 512
square lightmaps, all at the same authored density of 16 texels per unit.

## Edit the corridor trim

The galleries use separate [top](levels/showcase/wall-top.png) and
[bottom](levels/showcase/wall-bottom.png) brick strips, declared as ordinary
textures in [level.toml](levels/showcase/level.toml). Room `wall_bands` settings
in [world.yaml](levels/showcase/world.yaml) select each texture, height and
horizontal repeat distance; edge `bands` overrides those defaults.

The 128×32 images cover 2×0.5 metres, matching the main wall's 64 pixels per
metre and 0.5×0.25-metre bricks. Their darker colour and alpha masks remove
whole bricks along the inner edge. Bands follow the sloping floors and ceilings.
Change the artwork or YAML and rebuild to update them.

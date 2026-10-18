# 🏛️ Roman court & galleries

**A world to walk through. A different way to see it.**

Walk between marble columns, circle a mosaic-lined basin and explore sloping
brick galleries. Then switch from first person to isometric and see the whole space.
Karty brings the textures, lighting and characters together in one connected 3D world.

**[▶ Explore the court](https://karty-game.github.io/karty/main/world-camera/)** · [More demos](../README.md)

## Take a look around

Use **Roman court / Gallery** to jump between the courtyard and the galleries,
and **FPS / Isometric** to change camera. **Hide** clears the navigation panel;
press **C** to restore it, or touch the upper edge of the scene.

In first person, click the scene for **mouse look**, use **WASD** to walk and
strafe, and hold **Shift** to run. **Escape** releases the mouse. Arrow keys
look around without mouse capture. On touch screens, touch the lower left to
activate the movement joystick and swipe on the right half to look; both contacts
work together. The joystick accepts touches near the bottom of the screen and
has a small dead zone for steady movement. Its indicator stays inside the visible
scene when your thumb is below the picture in portrait mode.

In isometric, **WASD / arrow keys** pan, **Q/E** orbit, **+/-** zoom and
**Home** resets the view. Drag the scene to pan or use two fingers to pan and
pinch to scale. The on-screen orbit, zoom and center buttons also work with touch.
Changing the isometric view leaves the first-person player's position intact.
**V** switches camera; **1/2** selects the court/gallery.

The court uses secondary mineral colour variation, polished marble and mosaic
floor materials, and smaller baked lights around the colonnade and galleries.
Its shallow basin has rounded corners, a low ledge, a thick stone rim, a real mosaic bottom and
translucent water with animated height waves and planar reflections. Three bronze braziers hang above it on iron chains, with glowing coals, fire and warm static lights. Slim wall torches mark the static-light stations along the corridor, with flames and warm lights above their small metal collars.

## Run it

[Set up once](../README.md#run-a-sample), then from this sample's folder,
build its chunk lightmaps and open the preview:

```sh
../../dist/karty dev
```

The build creates six authored areas: the complete courtyard, hall, three
galleries, and north turn. Each contains its own prebuilt mesh and baked lightmap;
the shared textures and complete portal graph remain in the root level. Rebuild
after changing the world, its materials or its static lights. Area hints can
split when their mesh or lightmap exceeds the configured budget; see
the selected SDK's `.karty/docs/` for chunk settings.

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

See [the texture guide](https://karty-game.github.io/karty/main/guide/textures.html) for the asset
inventory, Materialize presets, nonmetallic overrides and generation ownership.

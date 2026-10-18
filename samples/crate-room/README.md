# Physics lab: crate room

Shoot and blast 80 wooden crates, walk into them to push, then reset the stacks.
Sloped side and rear banks return flying crates toward the middle, with three
low bumpers for ricochets. Every crate uses one box collider. The engine owns gravity, collisions and
angular momentum; this regular TinyGo WASI cartridge only creates props and sends input.

![Wooden crate stacks surrounded by hazard-striped ramps and red pinball bumpers.](preview.jpg)

Desktop: WASD move, Shift run, mouse look after clicking, arrows aim,
click or Space shoot, B/right-click blast, R reset, Escape release the cursor.
Touch: drag MOVE to walk, drag LOOK to aim, hold SHOOT, tap BLAST or RESET.
Independent fingers can move, aim and shoot together. Landscape gives the
largest view on a phone.

Build and preview the project with:

```sh
cd samples/crate-room
../../dist/karty build --target web
../../dist/karty serve
```

Normal `karty build`/`karty dev` installs the pinned public host automatically.

Use the same project for native builds, with `--target native`.
There are no private engine imports
and no cartridge physics dependency. Generated bindings live in ignored
`.karty/`; level sources, procedural material PNGs and the stdlib-only asset
generator are tracked. From the CLI root, `mise run generate-crate-room` rebuilds
textures. The room uses source version 11's reusable solid renderer prefab.

The static room lighting uses 32 hemisphere samples, three diffuse bounces and
medium denoising at 16 texels per world unit. Movable crates use live lighting.

Automatic website builds include this sample; publication is separate from a
local build. The local docs gallery includes it too: after building the web target, run
`mise run docs` from the CLI root and open `http://127.0.0.1:8081/`. The task
copies the full web build and screenshot into the gallery. Use
`mise run preview-docs` to refresh an already running docs server.

See [the texture guide](https://karty-game.github.io/karty/main/guide/textures.html) for the asset
inventory, Materialize presets, nonmetallic overrides and generation ownership.

## Reusable controller

This sample uses the [first-person preset](../../packages/controllers/README.md)
from the shared `controllers` package. Input state and camera calculations are
portable; this game owns its SDK adapter, entities and gameplay. Normal builds
restore the declared package automatically.

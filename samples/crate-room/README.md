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

The project pins unreleased SDK 0.0.12. With that candidate installed and a matching browser host artifact:

```sh
cd samples/crate-room
../../dist/karty build --target web --host /path/to/karty-host.wasm
../../dist/karty serve
```

Until SDK 0.0.12 is published, supply the prepared host with `--host`; its
matching `wasm_exec.js` must sit beside the browser WASM. After release, normal
`karty build`/`karty dev` installs the pinned public host automatically.

Use the same project for native builds, with `--target native --host
/path/to/karty-host`. There are no private engine imports
and no cartridge physics dependency. Generated bindings live in ignored
`.karty/`; level sources, procedural material PNGs and the stdlib-only asset
generator are tracked. From the CLI root, `mise run generate-crate-room` rebuilds
textures. The room uses source version 11's reusable solid renderer prefab.

This sample is ready for a later website showcase. Automatic website builds
exclude it until SDK 0.0.12 is released; publication is separate from this work.
The local docs gallery includes it now: after building the web target, run
`mise run docs` from the CLI root and open `http://127.0.0.1:8081/`. The task
copies the full web build and screenshot into the gallery. Use
`mise run preview-docs` to refresh an already running docs server.

See [material classifications](../../docs/sample-materials.md) for the asset
inventory, Materialize presets, nonmetallic overrides and generation ownership.

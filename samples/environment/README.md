# Open Air / environment lab

One playable layout, four styles: daylight, cartoon, fantasy, and space.
Start in an observation room looking through two recessed windows into the real
courtyard. Walk right through the covered passage to reach it, inspect the crate
and canopy shadows, and look back through those same windows.

The north ground is deliberately open: 48 by 40 world units for later terrain
generation and vegetation tests. This sample does not generate terrain or plants.

From the CLI repository root, with SDK 0.0.11 candidate installed:

```sh
mise run build
mise run check-environment-schema
mise run build-environment-web
mise run serve-environment
```

For local engine development, set `KARTY_HOST_WEB` or `KARTY_HOST_NATIVE` to the
matching compiled host when running the build/capture tasks. The sample's standard
build uses the public SDK's pinned host, without engine source or credentials.
No offline bake is needed. See [SDK bundles](../../docs/sdk-bundles.md) for installing
a local candidate archive and [environments](../../docs/environments.md) for authoring.

Click the scene to capture the mouse. WASD walks, mouse/arrow keys look, Shift
runs, Escape releases the pointer, V cycles viewpoints, P cycles presets, and H
hides/restores the controls. On touch screens, the left stick walks and the right
stick looks. Buttons select a style or viewpoint on either device.
T cycles 09:00, 12:00, 17:00 and 00:00; C cycles 0%, 35%, 70% and 100% clouds;
R resets the environment. From the default 15:00 / 35%, the first T selects
09:00 and the first C selects 70%.

Style switches load another level with the same layout and retain the chosen
viewpoint. The four view buttons are Windows, Courtyard, Look back, and Test ground.
The renderer debug menu also exposes runtime renderer controls.

The environment card changes the mounted level through the public guest
`SetEnvironment` API. Morning, Noon, Evening and Night select 09:00, 12:00,
17:00 and 00:00. Fewer and More change cloud amount by 10%; Clear and Max
select 0% and 100%. More from the default 35% selects 45%.
The card displays the current hour and cloud percentage. Reset environment and
style switches restore 15:00 with 35% clouds while preserving the viewpoint.
These controls require the SDK 0.0.11 candidate with API 0.0.9 / wire protocol 12.

The tracked [generator](../../scripts/environment-assets.go) owns all four
worlds and seven small procedural material images:

```sh
mise run generate-environment
```

Edit its shared layout once to change every preset. The generated worlds/materials
are kept in Git so builds do not require running the generator. Runtime skies are
procedural and use no panorama or cubemap. The crate, monolith and roof give obvious
static shadow references. Keep the open test ground clear when extending the entry.

Native screenshots use named batches, with the camera outside and facing the room
as well as inside facing the courtyard:

```sh
mise run capture-environment-daylight
mise run capture-environment-cartoon
mise run capture-environment-fantasy
mise run capture-environment-space
```

Compilation, actual WASM execution, native screenshots, and browser graphics
are separate validation steps. Screenshot output goes under ignored `dist/`.

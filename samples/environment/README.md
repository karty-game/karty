# Open Air / environment lab

One playable daylight layout, with a mountain backdrop and separate sparse and
dense cloud layers. Clouds extend down toward the horizon; increasing coverage
fills the distant sky as well as the overhead sky. The cloud and mountain layers
have different distances, adding parallax while you walk.
The sun has its own soft core and glare, becoming subtly larger and brighter
when you look toward it. Clouds and buildings still occlude it.
Start beside the observation room's west dining table, looking obliquely through
one recessed window into the real courtyard and mountain backdrop. The windows
have lightly tinted, static transparent glass with smooth
dielectric reflections and narrow metal mullions. Walk right through the covered
passage to reach the furnished courtyard, inspect the crate and canopy shadows,
and look back through those same windows.

Warm brick walls and oak furniture sit above quiet cool-grey paving, with charcoal
window frames and slate roofs. Subtle mortar and timber grain keep the building
and its silhouette more prominent than repeating surface patterns. The crate
remains a directional-shadow reference beside the west seating; the Look back
view centers the facade without that crate covering the windows.

Reusable table, chair and bench prefabs furnish the observation room and courtyard.
Small clipped corners soften their tabletops and seats while retaining their outer
bounds and the chairs' established box colliders.
Tables and benches expand into static solids during the build. Each of the three
tables has four runtime chairs with gravity and box collisions. Walking pushes
them; they can turn and tip, and viewpoint changes preserve their positions.
Reloading the level restores the initial seating layout. The main route from
the room through the side passage and all four viewpoint positions remain clear.
The building has a decorative upper storey with repeated windows, stone cornices,
a pitched slate roof and a chimney. Its closed wall and roof shells do not add
stairs or another playable room. The corridor lean-to has opaque wall-head closures
on both sides and both ends, following its roof slope without an open sunlight slit.
Opaque exterior panels enclose the ground floor around its two recessed windows.
A raised stone footing joins the facade to the courtyard paving. Low stone
boundary walls follow the exposed courtyard and grass-field edges, with gently
uneven tops and continuous corners. They leave the side passage and the broad
courtyard-to-field connection clear.

The north ground has an open interior: 48 by 40 world units for later terrain
generation and vegetation tests. This sample does not generate terrain or plants.
Thirty-six connected grass cells preserve that uninterrupted rectangle and its
world-anchored texture while bounding each floor's baked-lightmap page footprint.

From the CLI repository root, with SDK 0.0.12 candidate installed:

```sh
mise run build
mise run check-environment-schema
mise run build-environment-web
mise run serve-environment
```

For local engine development, set `KARTY_HOST_WEB` or `KARTY_HOST_NATIVE` to the
matching compiled host when running the build/capture tasks. The sample's standard
build uses the public SDK's pinned host, without engine source or credentials.
After building the web target, run `mise run docs` to open the Environment card
in the local sample gallery. Without a build, the card shows setup instructions.

The first build bakes chunk lightmaps offline; later builds reuse matching cached
results. See [SDK bundles](../../docs/sdk-bundles.md) for installing a local candidate
archive and [environments](../../docs/environments.md) for authoring.
Use the geometry preview for normal layout iteration:

```sh
mise run generate-environment
mise run check-environment-geometry
mise run build-environment-geometry-web
mise run serve-environment
```

The geometry task uses `--development`: it reuses valid cached lighting and
previews changed geometry with mesh-only chunks when its lighting is stale or
missing. It runs no new offline bakes. Inspect the layout, collisions and wall
joins in this preview, then run `mise run build-environment-web` once the geometry
is ready for its baked lighting.

Click the scene to capture the mouse. WASD walks, mouse/arrow keys look, Shift
runs, Escape releases the pointer, V cycles viewpoints, and H opens the controls.
On touch screens, touch the lower left of the scene to walk and the lower right
to look. Both sticks work together and stop independently on release or cancel.
The Controls button opens the viewpoint/time/cloud panel on either device.
Close controls returns to the scene; selecting a viewpoint closes it too.
The panel pauses movement input while open and uses touch targets at least
44 CSS pixels wide/high, including the button that reopens it.
T cycles 09:00, 12:00, 17:00 and 00:00; C cycles 0%, 35%, 70% and 100% clouds;
R resets the environment. From the default Noon 12:00 / 35%, the first T selects
17:00 and the first C selects 70%.

The four view buttons are Windows, Courtyard, Look back, and Test ground.
The normal preview keeps renderer diagnostics out of the touch controls.

The environment card changes the mounted level through the public guest
`SetEnvironment` API. Morning, Noon, Evening and Night select 09:00, 12:00,
17:00 and 00:00. Fewer and More change cloud amount by 10%; Clear and Max
select 0% and 100%. More from the default 35% selects 45%.
The controls and compact scene HUD display the current hour and cloud percentage.
Reset restores Noon 12:00 with 35% clouds while preserving the viewpoint. Daylight is
the sample's only level; the other engine presets remain available for authoring.

The authored lightmaps use 24 texels per world unit and adaptive pages up to
1024px. The outdoor layout packs into six chunks, using 512px pages when their
receivers fit and 1024px pages for larger groups. Exactly
four hours (00:00, 09:00, 12:00 and 17:00) supply the Night, Morning, Noon and Evening
lighting states. Each hour has one baked map per chunk. Cloud cover changes their
brightness without adding weather maps or changing the baked shadow direction.
Two pixels of chart padding allow the current layout to fit six pages while
protecting bilinear samples; denoising stays within each chart. The offline recipe
uses sixteen hemisphere samples, one diffuse bounce and medium denoising.
Static buildings and furniture use live lighting while a new condition downloads,
then switch to its complete prepared pages. The previous condition’s baked shadows
are suppressed during that transition.

The tracked [generator](../../scripts/environment-assets.go) owns the daylight
world, reusable furniture prefabs and eleven procedural material images. The seamless grass/dirt
[master](material-sources/grass-soil-2048.png) is 2048×2048 with broad muted
patches. The generator derives the existing 256×256 material tile offline, so
the full master does not ship in each level. Its 12m repeat keeps variation visible
across the reserved field without adding terrain or plants. Other starter materials are
128×128. Runtime sky panoramas retain their own full resolution:

```sh
mise run generate-environment
```

Edit the tracked layout to change the sample. The generated worlds/materials
are kept in Git so builds do not require running the generator. Sky and backdrop
textures belong to this sample in [levels/daylight/sky](levels/daylight/sky/README.md).
Its world YAML references the sparse/dense clouds and mountains, with UV/parallax/haze
settings in the tracked generator. The CLI compiles only those referenced PNGs into
the sky sidecar; replacing them requires a sample rebuild and leaves the host unchanged.
The crate, monolith, canopy and pitched roof give obvious static shadow references.
Keep the open test ground clear when extending the entry.

Native screenshots use named batches, with the camera outside and facing the room
as well as inside facing the courtyard:

```sh
mise run capture-environment-daylight
```

Compilation, actual WASM execution, native screenshots, and browser graphics
are separate validation steps. Screenshot output goes under ignored `dist/`.

Phone-sized browser checks run from the engine root after building the sample:

```sh
mise run check-environment-mobile
```

The check covers 320px/390px portrait and 740px/844px landscape viewports, painted
touch targets, public weather controls, simultaneous walk/look, independent
release, cancellation and closing/reopening controls. These are synthesized
browser touches; physical-phone performance remains a separate check.

See [material classifications](../../docs/sample-materials.md) for the asset
inventory, Materialize presets, nonmetallic overrides and generation ownership.

The indoor fills are restrained so window light and recess shading remain
visible. Prepared lighting uses sixteen hemisphere samples and one bounce;
changing materials or geometry takes longer on the first bake and then reuses
matching caches. Furniture has narrow edge bevels, lighter end grain and separate
bare-steel supports, while the twelve chairs retain their physical box sizes.

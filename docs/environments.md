# Environments

SDK 0.0.12 supports `world/environment@1` with source-v9 worlds and
authored source-v12 skies. It supplies a project-authored textured sky dome with separate cloud and scenic backdrop layers.
One preset supplies sky, directional sun and coordinated fill colors. Time of day
and cloud coverage can be authored or changed on a mounted level through the
public game API.

```yaml
version: 12
environment:
  preset: fantasy
  backdrop: mountains
  time_of_day: 15
  cloud_amount: 0.35
  sky:
    cloud_sparse: sky/clouds-sparse.png
    cloud_dense: sky/clouds-dense.png
    horizon: sky/mountains.png
    settings:
      cloud_uv_scale: [1, 0.95]
      horizon_uv_scale: 1.5
      horizon_haze: 0.35
```

Presets are `daylight`, `cartoon`, `fantasy`, and `space`. Optional `palette`
choices are `default`, `violet`, and `twilight`. Leaving palette out selects the
preset's own palette. Preset changes leave the geometry and physical route intact.

The host supplies sky rendering and an asset-free gradient fallback. Source v12
`environment.sky` supplies optional PNG paths for `cloud_sparse`, `cloud_dense`,
`background`, and `horizon`, confined to the level directory. Changing cloud amount
blends the authored cloud coverage. Only referenced images are packaged into the
`@world/sky.bin` sidecar; packaging requires SDK capability `world/sky@1`.
The CLI preserves image dimensions and alpha and compiles deterministic gzip
linear premultiplied RGBA8. Images are bounded to 4096 per dimension and 4,194,304
pixels each; decoded totals and encoded sidecar size also have SDK bounds.

Optional `settings` fields override defaults individually: `cloud_uv_scale`
(two positive values through 4), `cloud_dense_scroll_scale` (0–4),
`cloud_dense_offset` (two values in −1–1), `cloud_parallax` (two values in
−0.01–0.01), `horizon_parallax` (0–0.01), `horizon_uv_scale`
(positive through 4), `horizon_haze` (0–1), and
`horizon_emission` (three values in 0–1). UV and scroll scales default to 1;
other settings default to 0. Omit optional fields instead of supplying null.

Clouds have no elapsed-time wind. They move through camera parallax while you
walk; `cloud_dense_scroll_scale` scales that camera-driven offset for the dense
layer. A stationary camera sees stationary clouds.

Optional `backdrop` retains the legacy `mountains`, `city`, or `none` metadata.
An authored `sky.horizon` draws independently of these names; omit its path to
leave out the scenic layer. Explicit backdrop selection
requires source v11 and SDK 0.0.12 advertising
`world/environment-backdrop@1`. These scenic layers have no collision and do not
replace the level's walkable ground, terrain, or buildings.

Optional `time_of_day` is a finite hour in `[0, 24)`; zero means midnight.
Optional `cloud_amount` is a finite coverage in `[0, 1]`; zero means clear.
The default cloud amount is `0.35`. Leaving time out preserves the preset's
original sun pose or an authored `sun`. The sample starts at 15:00.

Override the fixed sun only when needed:

```yaml
environment:
  preset: cartoon
  sun:
    azimuth: 135
    elevation: 40
```

Both angles must be supplied. Azimuth follows world XY coordinates: 0 degrees
points toward +X and 90 toward +Y. Elevation is degrees above the horizon.
Azimuth lies in `[0, 360)`; elevation lies in `[0, 90]`. Explicit null is rejected.
Choose either `time_of_day` or `sun`; authoring both is rejected.

The guest can update both controls without loading another level or moving its
camera. The setter retains the level's preset, palette and backdrop:

```go
game.SetEnvironment(level, engine.EnvironmentSettings{
    TimeOfDay: 15,
    CloudAmount: 0.35,
})
```

This requires SDK 0.0.11 or later. The level must already have an authored
environment. Both values are supplied together;
invalid handles or values reject the command batch. Cloud changes reuse the sun
shadow map; a changed sun direction refreshes it once.

Set `sky: true` on an outdoor room to make its ceiling a view of the environment.
Keep a finite authored ceiling plane: it still bounds the room and camera
collision. The sky dome surrounds the camera; layered clouds and backdrops add
distance cues while walking.
Adjacent sky rooms show a continuous sky across ceiling-height changes.

```yaml
rooms:
  - id: courtyard
    sky: true
    floor: {c: 0}
    ceiling: {c: 8}
    floor_material: paving
    ceiling_material: ceiling
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 8, y: 0}, material: wall}
      - {id: east, start: {x: 8, y: 0}, end: {x: 8, y: 8}, material: wall, sky: true}
      - {id: north, start: {x: 8, y: 8}, end: {x: 0, y: 8}, material: wall, sky: true}
      - {id: west, start: {x: 0, y: 8}, end: {x: 0, y: 0}, material: wall}
```

Boundary `sky: true` replaces that whole outer wall with the environment to
reveal the horizon. The edge remains a physical world boundary. Sky edges must
belong to a sky room and cannot participate in a portal connection. Real
building walls keep their regular material. Material fields remain required
for compatibility with the bounded world representation.

Indoor windows are real portal openings to the same outdoor geometry. Use short
recess rooms with raised floors and lowered ceilings for sills and lintels; a
separate connected doorway or passage provides the walking route outside.
Sunlight is blocked by regular walls, roofs and static solids. Sky ceilings and
horizon walls do not cast shadows. Existing local lights can supplement interiors.

The renderer uses static casters for cached directional shadows. Moving objects
receive those shadows; moving-object shadow casting, an automatic clock, terrain
generation and vegetation remain extensions. SDK 0.0.12 also supports prepared
sun and sky diffuse lightmaps, including indirect bounces, through
`[lightmap.environment]`; see [lightmaps](lightmaps.md). The nearest authored
clock selects baked shadows without interpolating their direction. Point lights,
authored ambient, actors, glass reflections and live highlights remain runtime
contributions. While a requested condition loads, the visible scene uses current
live lighting until all required maps are ready; failed conditions retain that
fallback. The host performs no bake during loading or time changes.

The [environment sample](../samples/environment/README.md) uses one daylight
level and four prepared clock states. Its first normal build bakes them offline;
matching subsequent builds reuse cached images. Additional clocks, samples and
bounces increase build time and content size. Use its geometry development task
for layout iteration without new bakes, then prepare final lighting with a normal
build.

`karty schema --check` validates enums, control and angle bounds, source version and required
sky-room/environment declarations. Compilation additionally rejects connected
sky edges. Packaging requires the selected SDK to advertise `world/environment@1`;
the cartridge records that required runtime feature. An explicit backdrop also
requires and records `world/environment-backdrop@1`, so SDK 0.0.11 rejects that
new payload. Omitting the backdrop retains the original environment payload.

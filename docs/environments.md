# Environments

SDK 0.0.11 candidate supports `world/environment@1` with source-v9 worlds.
Authored environment presets provide a procedural sky and one directional sun
with coordinated fill colors. Time of day and cloud coverage can be authored
or changed on a mounted level through the public game API.

```yaml
version: 9
environment:
  preset: fantasy
  time_of_day: 15
  cloud_amount: 0.35
```

Presets are `daylight`, `cartoon`, `fantasy`, and `space`. Optional `palette`
choices are `default`, `violet`, and `twilight`. Leaving palette out selects the
preset's own palette. Preset changes leave the geometry and physical route intact.

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
camera. The setter retains the level's preset and palette:

```go
game.SetEnvironment(level, engine.EnvironmentSettings{
    TimeOfDay: 15,
    CloudAmount: 0.35,
})
```

This requires SDK 0.0.11 candidate with API 0.0.9 and wire protocol 12. The level
must already have an authored environment. Both values are supplied together;
invalid handles or values reject the command batch. Cloud changes reuse the sun
shadow map; a changed sun direction refreshes it once.

Set `sky: true` on an outdoor room to make its ceiling a view of the environment.
Keep a finite authored ceiling plane: it still bounds the room and camera
collision. The sky follows viewing direction, so it looks distant while walking.
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

The first renderer uses static casters for cached directional shadows. Moving
objects receive those shadows; moving-object shadow casting, an automatic clock,
sky indirect-light baking, terrain generation, and vegetation are separate work.
No offline lighting bake is required by the [environment sample](../samples/environment/README.md).

`karty schema --check` validates enums, control and angle bounds, source version and required
sky-room/environment declarations. Compilation additionally rejects connected
sky edges. Packaging requires the selected SDK to advertise `world/environment@1`;
the cartridge records that required runtime feature. A released SDK without it
cannot silently accept an authored environment.

# Authored GPU sprite particles

Candidate SDK 0.0.11 supports world source version 10 with `particles.version: 1`.
Particle presets and placed emitters are YAML authored; the renderer calculates
birth/death, movement, billboard animation and appearance on the GPU from emitter
time and seed. The CPU manages emitter resources and submits fixed particle
slots; it does not update individual particle positions every frame.

```yaml
version: 10
particles:
  version: 1
  presets:
    - name: acid-sparkle
      capacity: 48
      emission: {rate: 12, duration_seconds: 3}
      lifetime_seconds: {min: 0.6, max: 1.2}
      spawn: {shape: box, half_extent: {x: 0.2, y: 0.2, z: 0.01}}
      velocity_min: {x: -0.04, y: -0.04, z: 0.2}
      velocity_max: {x: 0.04, y: 0.04, z: 0.5}
      acceleration: {z: 0.08}
      drag: 0.12
      sprite:
        frames: [acid-sparkle]
        playback: cycle
        facing: camera-facing
        blend: additive
      size_over_life: [{age: 0, value: 0.08}, {age: 1, value: 0.01}]
      color_over_life:
        - {age: 0, rgba: [0.2, 1, 0.3, 0.8]}
        - {age: 1, rgba: [0.1, 0.5, 0.1, 0]}
  emitters:
    - id: pool-sparkles
      preset: acid-sparkle
      position: {x: 4, y: 4, z: 1}
      seed: 42
      playback: {mode: cycle, period_seconds: 3}
```

This block accompanies the level's existing rooms and materials. Declare each
frame in `levels/<level>/level.toml`, including frames used only by particles:

```toml
[[textures]]
name = "acid-sparkle"
source = "acid-sparkle.png"
profile = "sprite"
```

Use transparent square source images. Each frame is a complete texture, not a
rectangle selected from an authored sprite sheet. The ordinary asset pipeline
produces straight-alpha QOI; see [asset processing](assets.md). All frames within
a preset must have matching **processed** square dimensions. Apply the same
resize settings to all frames if their original sizes differ.

## Preset fields

All numeric values must be finite. Vectors use world distance and seconds, with Z
up; omitted vector components are zero. Ranges require both `min` and `max`, with
`min <= max`. Optional fields may be omitted; explicit null is rejected throughout
the particle block. Required fields remain required even when their value is zero.

| Field                          | Requirement and meaning                                                                                                                                                                                         |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`                         | Required unique preset identifier, 1–128 UTF-8 bytes.                                                                                                                                                           |
| `capacity`                     | Required integer, 1–4,096 slots **per placed emitter** using this preset.                                                                                                                                       |
| `emission`                     | Required object. `rate` defaults to 0, `duration_seconds` to 0, and `bursts` to `[]`. At least one birth is required.                                                                                           |
| `emission.rate`                | 0–4,096 births/second. A positive rate requires positive duration.                                                                                                                                              |
| `emission.duration_seconds`    | 0–3,600 seconds. Rate births occur at offsets `n/rate`, starting with 0, strictly before this duration.                                                                                                         |
| `emission.bursts`              | Up to 16 additional timed bursts. Every item requires `at_seconds` in 0–3,600 and integer `count` in 1–4,096.                                                                                                   |
| `lifetime_seconds`             | Required `{min, max}`, each 0.001–3,600 seconds. Lifetime is sampled uniformly.                                                                                                                                 |
| `spawn`                        | Required `shape: point` or `shape: box`. Optional `half_extent` defaults to zero. Each component is 0–1,000,000; point spawn requires all zero. A box samples each local axis uniformly within its half extent. |
| `velocity_min`, `velocity_max` | Optional vectors, default zero. Each component is −1,000–1,000 world units/second, with minimum no greater than maximum on each axis. Each axis is sampled independently.                                       |
| `acceleration`                 | Optional constant local vector, default zero. Components are −1,000–1,000 world units/second².                                                                                                                  |
| `drag`                         | Optional linear velocity damping, default 0; allowed 0–1,000 per second. Motion follows `dv/dt = acceleration - drag * velocity`.                                                                               |
| `sprite`                       | Required object; its fields are listed below.                                                                                                                                                                   |
| `size_over_life`               | Required 2–4 `{age, value}` keys. Both fields are required. Age is normalized lifetime in 0–1; value is square billboard edge length in 0–1,000 world units.                                                    |
| `color_over_life`              | Required 2–4 `{age, rgba}` keys. Both fields are required. `rgba` has exactly four components in 0–1.                                                                                                           |
| `rotation_degrees`             | Optional `{min, max}`, default `{min: 0, max: 0}`; endpoints −360,000–360,000 degrees around the billboard normal.                                                                                              |
| `rotation_speed_degrees`       | Optional `{min, max}`, default zero; endpoints −360,000–360,000 degrees/second.                                                                                                                                 |

Size and colour curves are piecewise linear, start at age 0 and end at age 1.
Keys must remain strictly increasing after float32 conversion, and every adjacent
uploaded age difference must be at least `2^-126` (about `1.17549435e-38`). This
prevents collapsed or zero interpolation intervals on portable GPUs.

| `sprite` field | Requirement and meaning                                                                                                                 |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `frames`       | Required ordered list of 1–16 declared texture names.                                                                                   |
| `fps`          | Optional, default 0; allowed 0–1,000. Multiple frames require an uploaded float32 value of at least `2^-126`.                           |
| `playback`     | Required `once` or `cycle`. `once` holds the final flipbook frame; `cycle` wraps frames. This controls each particle's image animation. |
| `facing`       | Required `camera-facing` or `upright`. Upright billboards remain vertical around world Z.                                               |
| `blend`        | Required `additive` or `cutout`. Additive suits sparks/fire. Cutout uses hard alpha edges; smooth alpha smoke is outside this version.  |

The host packs each preset's frames together before readiness. A connected group
of multiple frame textures must fit a 2,048×2,048 sprite atlas page, including two
pixels of total padding on each frame dimension. Presets that share frames merge
into one connected group. Reduce frame resolution or avoid sharing frames across
large flipbooks if packing fails. A single unique frame may use a larger standalone
page, within the current renderer's 4,096×4,096 texture storage limit. Asset profile
defaults are separately configurable through image transforms. CLI builds check processed dimensions;
the host checks this renderer-specific packing constraint before accepting the level.

Particles are unlit: their texture is multiplied by the colour curve, without
world lighting. Cutout sprites use a 0.5 alpha test and write depth
before transparent geometry. Additive sprites draw after ordinary translucent
geometry, test opaque depth, and receive no foreground-alpha attenuation. They
do not cast shadows, appear in reflection passes, illuminate the world, or affect
lighting bakes. Add a separate authored point light when a fire should light its room.

## Placed emitter fields

| Field                     | Requirement and meaning                                                                                                            |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `id`                      | Required identifier, 1–128 UTF-8 bytes; unique after expansion.                                                                    |
| `preset`                  | Required root preset name.                                                                                                         |
| `position`                | Required vector; components −1,000,000–1,000,000. The final world envelope must also fit those coordinate bounds.                  |
| `yaw_degrees`             | Optional, default 0; −360,000–360,000. Rotation around local Z, normalized after prefab composition.                               |
| `scale`                   | Optional uniform scale. Omission or zero means 1; otherwise 0.000001–1,000. The final composed scale must also stay in that range. |
| `seed`                    | Required integer, 0–4,294,967,295; selects repeatable particle variation.                                                          |
| `playback.mode`           | Required `once` or `cycle`; controls the **emission schedule**.                                                                    |
| `playback.period_seconds` | Default 0. Must remain 0 for `once`; `cycle` requires 0.001–3,600 seconds.                                                         |
| `playback.delay_seconds`  | Optional, default 0; 0–3,600 seconds before the first schedule starts.                                                             |

Rooms and prefabs can contain `particle_emitters` arrays referencing root presets.
An emitter-only prefab is valid; no marker or geometry is required inside it.
Their positions, yaw and scale follow the composed prefab transform. Velocity,
acceleration and spawn support are emitter-local: yaw rotates them around Z;
uniform scale scales displacement and billboard size. Expanded IDs follow the
existing room/instance path convention. Room-local emitters must fit sectors from
that expanded room; root and prefab-level emitters select a containing sector.

## Once, cycle and capacity

Rate and burst births add together, including births at the same timestamp.
A cycle requires `duration_seconds <= period_seconds`, and every compiled
float32 birth offset must be strictly less than the uploaded float32 period.
Thus a burst exactly at the period is rejected. For a one-shot burst, duplicate
`acid-sparkle` as `vent-sparks`, set its capacity to 24, and replace its emission:

```yaml
emission:
  bursts: [{at_seconds: 0, count: 24}]
```

Then add this item to `particles.emitters` or a room/prefab `particle_emitters` array:

```yaml
- id: vent-burst
  preset: vent-sparks
  position: {x: 4, y: 4, z: 1}
  seed: 7
  playback: {mode: once, delay_seconds: 2}
```

This effect emits once, two seconds after mount, and drains after its final
particle dies. A cycling emitter repeats its finite schedule at each period.
Particles from earlier cycles can remain alive. Emitters share the mounted-level
clock across cameras and continue aging while hidden; a fresh mount restarts time.

Each finite schedule contains at most 4,096 births. Once playback needs at least
that many slots. A cycle reserves a complete schedule for every overlapping
lifetime group:

```text
groups = max(1, ceil(float64(float32(max_lifetime)) /
                     float64(float32(period))))
required_capacity = births_in_schedule * groups
```

The float32 conversions match uploaded GPU values. For example, lifetime 0.3 and
period 0.1 require four groups because of uploaded rounding. The compiler rejects
insufficient capacity, without killing a surviving particle or silently dropping
births. Spare slots stay inactive. Each emitter reserves its preset's full capacity,
so two emitters using capacity 48 consume 96 level slots.

| Resource               | Budget                                                                                               |
| ---------------------- | ---------------------------------------------------------------------------------------------------- |
| Presets                | 1–64 when particles are present.                                                                     |
| Emitters               | 1–256 after expansion; authored root/room/prefab emitter definitions also have a combined 256 limit. |
| Capacity               | 1–4,096 per emitter; 65,536 slots total per level.                                                   |
| Births                 | 1–4,096 per finite rate-plus-burst schedule.                                                         |
| Frames / bursts / keys | 16 frames and 16 bursts per preset; 2–4 keys per size/colour curve.                                  |
| Encoded resource       | At most 1 MiB.                                                                                       |

## Resolving confinement errors

The complete spawn, motion and billboard envelope must stay inside one compiled
convex sector, including sloped floor and ceiling. Bounds include maximum lifetime,
trajectory extrema, maximum billboard radius, transforms and GPU precision margin.
The emitter's centre being inside a room is insufficient. Static solids do not
participate in this envelope check; particles can visually pass through a barrel
or other prop.

A build error reports the emitter ID, its envelope minimum/maximum and, for
room-local emitters, the required room. Move the emitter inward, or reduce box half
extent, velocity, acceleration, maximum lifetime, billboard size or scale. Leave
clearance above the floor and below the ceiling for the entire billboard. Positive
drag can reduce travel, but maximum lifetime and all initial velocities still count.

Concave rooms and rooms with collinear portal edges can become several convex
sectors. A wide box can straddle that split even when it stays inside the authored
room. Split it into smaller emitters placed inside the resulting convex regions;
increasing capacity cannot fix a confinement error. Hangar's acid emitters use
small spawn strips for this reason.

## Validation and ownership

`karty schema --check` verifies fields, versions, scalar bounds, enums and named
references. Builds also check related lifecycle values, float32 curve/frame
precision, required capacity, processed frame dimensions and sector confinement.
The CLI generates typed `@world/particles` data and declares `world/particles@1`
with `world/sectors@1`; do not declare this generated resource in `[[data]]`.
The host repeats complete validation and checks texture identity/atlas packing
before readiness. Source versions 1–9 reject particle fields, including null;
SDKs without the capability cannot build or mount these effects.

The public SDK owns source types, binary codec, budgets, capacity and bounds.
The public CLI owns YAML/schema, prefab expansion, texture-name resolution and
level staging. Those repositories remain independent of private engine source.
The engine owns readiness and GPU rendering. Version 1 provides automatic visual
playback, without gameplay emitter commands, stateful simulation, general force
fields, collisions, cross-sector motion, lighting contribution or smooth alpha/OIT.

See the [versioned decision](decisions/world-particles-v1.md) and
[Hangar's authored world](../samples/hangar/levels/hangar/world.yaml) for the working
acid sparkles, barrel fire and ember presets.

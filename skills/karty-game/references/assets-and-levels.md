# Assets and levels

For public authoring guides and complete worlds, use
[Public sources](public-sources.md#guides-and-api-behavior).

## Asset sources and scope

| Source                                 | Discovery / declaration                                                   | Input                                                        |
| -------------------------------------- | ------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `assets/textures/**`                   | Automatic; sparse `[[assets.texture]]` overrides                          | Static PNG, JPEG, WebP; PNG supports alpha                   |
| `assets/sounds/**`                     | Automatic; `[[assets.sound]]` overrides                                   | RIFF WAV mono/stereo PCM8/16/24/32 or float32; effects ≤30 s |
| `assets/fonts/`                        | Conventional `body`, `display`, `mono`; `[[assets.font]]` overrides       | `.ttf` / `.otf`, one font per role                           |
| `ui/**/*.kui`, `assets/ui/**/*.kui`    | Automatic components; `[[assets.ui]]` overrides                           | KartUI components                                            |
| `ui/layouts/**/*.kui`                  | Automatic; `[[assets.layout]]` for other locations                        | Build-time shells with slots                                 |
| `ui/theme.toml`                        | Fallback `assets/ui/theme.toml`; `[assets.theme]` override                | Tokens, typography, images, nine-slices                      |
| `assets/music/`, `assets/environment/` | Suggested folders; explicit `[[assets.music]]` / `[[assets.environment]]` | WAV streaming music / environmental audio                    |
| `assets/videos/`                       | Suggested folder; explicit `[[assets.video]]`                             | Encoded `.mpg`, MPEG-1 video, optional MP2 audio             |
| Level directory                        | Explicit `[[textures]]`, `[[ui]]`, `[[data]]`                             | Mount-owned textures, UI, binary data                        |

Root declarations belong in `karty.toml`. Override discovered assets only for
names, profiles, transforms, retention or other locations. Root paths resolve
from the project; level paths resolve from their level directory. Both require
confined regular files; `../../assets/...` and escaping symlinks are rejected.
Use game scope for shared HUD/player resources, level scope for scene content,
and package exports for reuse. Level material namespaces are independent.
`assets/environment/` is audio; sky/weather use the world `environments.md` contract.

`assets/textures/player/idle.png` becomes `player.idle`. Names are unique within
scope and cannot start `karty.`. Read generated Go names from `.karty/assets/`;
numeric IDs are build-local. Keep non-runtime artwork outside discovery folders.

- Profiles: `sprite` uses nearest filtering; `interface` / `environment` smooth.
  Defaults fit within 4096×4096, preserve aspect ratio and never upscale.
- Images become 8-bit straight-alpha QOI; WAV effects become lossy QOA, normally
  48 kHz with channels preserved. Explicit mono suits spatial effects.
- Animated PNG/WebP and MP4/H.264 inputs are unsupported. Use supported flipbooks
  for sprite animation. Particle source PNGs remain straight-alpha.
- Transform caps can change sprite regions and intrinsic UI size. World sprite
  density caps can additionally downsize placements; inspect `karty-toml.md`.
- `dist/raw/asset-report.toml` records retention, dimensions, encoded/decoded cost
  and cache hits. Source file size does not predict runtime cost. Typed references
  retain game textures; use `keep = true` for deliberate otherwise-unreachable use.

Details: `.karty/docs/assets.md`, `karty-toml.md`.

## Level and schema

`levels/<directory>/level.toml` declares the logical name passed to `RequestLevel`.
Level content is passive; behavior stays in Go. Minimal source-v19 world example
(use the existing world's version unless a supported feature needs an upgrade):

```toml
# levels/room/level.toml
[level]
name = "levels.room"
kind = "world"
[world]
source = "world.yaml"
[[textures]]
name = "surface"
source = "surface.png"
profile = "environment"
```

Put the image at `levels/room/surface.png`:

```yaml
# levels/room/world.yaml
# $schema: ../../.karty/schemas/levels/room.schema.json
version: 19
rooms:
  - id: room
    rect: {width: 8, depth: 8, material: surface}
    floor: {c: 0}
    ceiling: {c: 3}
    floor_material: surface
    ceiling_material: surface
```

XY is the floor plan in metres; Z is up. `rect` creates counterclockwise `south`,
`east`, `north`, `west` edges for connections. Use `boundary` for irregular rooms
or per-edge settings. YAML accepts one document, known fields, no anchors/aliases.
Root `[[assets.texture]]` entries do not declare level material textures.
`[[data]]` carries named runtime bytes; compile editable text during the build.

`karty schema` generates `.karty/schemas/worldsource.base.schema.json` and
`.karty/schemas/levels/<directory>.schema.json`; the latter includes resource-name
suggestions. `karty schema --check` checks shape/references without image processing
or Go compilation, potentially restoring packages. Build afterward to validate
expanded geometry, budgets and packaging. Correct authored inputs, not schemas.

## Materials and prefabs

Immediately after a level `[[textures]]` entry, select Materialize settings:

```toml
[textures.materialize]
preset = "concrete"
```

Karty prepares maps/atlases using its pinned tool. Presets such as `wood`, `stone`,
`metal`, `concrete` generate normal/height/AO and supported reflectance maps;
they do not add liquid motion or emission. `[textures.material_strengths]` adjusts
normal/height/AO/rim display strength separately. Root world UV example:

```yaml
uv:
  mode: triplanar
  anchor: world
  scale: {x: 2, y: 2}
```

Scale is metres per repeat; larger means a larger pattern. Rooms override
`floor_uv`, `ceiling_uv`, `wall_uv`. Details: `materialize.md`,
`material-mapping.md`, `material-advanced.md`.

Reuse world `prefabs` / `instances` or package exports. Solid footprints are
convex and counterclockwise; split concave objects. Instances apply translation,
yaw, uniform scale and material substitutions; expanded IDs like `crate-a/body`
are stable discovery names. Walkable platforms use rooms: solid collision alone
does not create a walkable floor. Visual animation leaves collision/query geometry
static. Root lights are global, independent of prefab placement.

Movable props use solid-only `renderable: true` prefabs with collision-disabled
renderer geometry, `NewSolid3D` and a host body. See `solid-renderers.md` and the
[physics recipe](code-and-capabilities.md); static world instances are not bodies.

## Lighting and baking

Add lighting to the level's world source:

```yaml
lighting:
  version: 1
  ambient: {x: 0.05, y: 0.05, z: 0.06}
  lights:
    - id: room-lamp
      position: {x: 4, y: 4, z: 2.5}
      color: {x: 1, y: 0.75, z: 0.45}
      radius: 6
```

Authored colors are linear RGB in [0,1]; position/radius use world units. Omitted
`lighting` is unlit. `lighting.actors: true` opts world sprites into lighting;
canvas sprites/UI remain unlit. `[textures.emission]` makes surfaces glow but
requires explicit lights to illuminate nearby geometry. Use runtime point/spot
components for moving/game-controlled sources. For a whole-level static bake:

```toml
# level.toml
[lightmap]
enabled = true
offline = true
density = 4
page_size = 512
bake_samples = 16
bake_bounces = 1
```

Run `karty render bake --level room --incremental`, then rebuild/restart preview.
Steady fixed sources bake automatically; moving/pulsing sources stay live.
Completed bakes remove selected lights from the shared live budget (currently 50).
Rebake after relevant geometry/material/UV/selected-light edits. Stale automatic
bakes may fall back to live lighting: inspect logs and visuals. Chunked builds
produce independent bake pages. Details: `lighting.md`, `dynamic-lights.md`,
`world-actors.md`, `lightmaps.md`, `chunks.md`.

Before adding terrain, environments, liquids, vegetation or particles, read its
pinned feature reference for source versions, required lighting and runtime limits.
Streaming media/chunks can create hashed `content/` sidecars; distribute the whole
output. Mount readiness/release rules are in [Code and capabilities](code-and-capabilities.md).

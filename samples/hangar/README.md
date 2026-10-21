# Hangar

Walk through service tunnels, climb to the control deck and cross a zigzag
above a recessed trench. This industrial 3D sample combines baked lighting,
box physics, animated machinery, particles and spatial audio.

[Play the demo](https://karty-game.github.io/karty/main/hangar/) · [All samples](../README.md)

![Green-lit reactor machinery inside the Hangar.](preview.jpg)

## Run it

The sample's [karty.toml](karty.toml) pins SDK 0.0.16. Its matching public host
supports the reciprocal physics portal loop. The CLI installs and verifies the
host and its matching `wasm_exec.js` when building.
After [sample setup](../README.md#run-a-sample), run from the CLI repository root
to build a browser preview:

```sh
mise run check-hangar-schema
mise run bake-hangar
mise run build-hangar-web
mise exec -- ./dist/karty serve samples/hangar/dist/web --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080/`. To build and run the native version instead:

```sh
mise run build-hangar-native
mise exec -- ./samples/hangar/dist/native/karty-host --cartridge samples/hangar/dist/native/game.kart
```

Engine contributors can select local hosts with
`KARTY_HOST_NATIVE=/path/to/karty-host` and
`KARTY_HOST_WEB=/path/to/karty-host.wasm`; keep its matching `wasm_exec.js`
beside the web host. The selected SDK, host and cartridge must agree.

## Controls

**WASD** walks; click the scene to look with the **mouse**, and press **Esc** to
release it. Hold **Shift** to run; arrow keys also look. On a smartphone, use the
**left joystick** to walk and the **right joystick** to look. They appear on first
touch. Open **Camera** at the top right to choose a landmark; the panel starts
collapsed and closes after a selection. **V** jumps to the next landmark, and
**H** hides the interface.

**F** toggles the torch; its on-screen button also works on touch devices.
Walk into crates to push them, or board the observation-room elevator to ride it.

## Authoring

[Layout and liquids](#layout-and-liquids) · [Crates](#crates) · [Audio](#spatial-audio) ·
[Step support](#step-support) · [Torch](#torch) · [Materials](#world-and-materials) ·
[Animation](#machine-and-surface-animation) · [Effects](#reused-industrial-effects)

### Layout and liquids

The acid pool is 2.4 metres deep, with its bed at -3.5 and its animated surface
at -1.1. Both trench exits use continuous ramps and sloping ceilings; their steel
trim follows the ceiling without bars across the passage. The acid surface fills
the low end of each passage up to its intersection with the slope. The east ramp rises to
the 0.2-metre service floor and the west ramp to the ground-level return gallery.
Shoreline foam and crate buoyancy make the pool interactive. Push the freight
box beside the east ramp into acid to see it float. Walking down into the pool
adds green distance fog and animated world distortion; the UI stays clear. Four
smaller crates float around the basin. The acid density is 25, so the demo crate
rests about 60% submerged; fog density is 0.45 and distortion is 0.006. The
player follows gravity and can walk back up either ramp. Choose **Camera + →
Under acid** for a submerged view, or **Float demo** to start beside the freight.
Swimming, acid damage, wakes and dynamic crate-contact foam remain deferred.
`mise run capture-hangar-pool` captures both ramps, their waterlines and a basin overview.

The acid chamber has a nine-metre ceiling. An elevated control room on its south
wall, opposite extraction, overlooks the basin and zigzag through two five-metre
windows. Reach it by branching south from the reactor-to-acid tunnel, or from the
eastern trench stairs and their service tunnel. The east approach climbs in
0.2-metre steps to the 3.4-metre control floor. The Camera menu includes
**Acid control**; the screenshot batch includes the overlook, windows and entrances.
`mise run capture-hangar-control` captures those views plus the west elevator and east stairway and the
consoles. The lightmap uses four texels per metre to fit the taller chamber and
new passages into the existing single-page budget; the control lamps stay local.
The pillars reach the ceiling, and the beam nearest the windows is removed to
keep the view open. The sample controller anticipates these risers with the full player-box footprint
and keeps the higher window sills impassable. Its generated floor/portal support
in `internal/fps/walk_world.generated.go` comes from the same compiled authored
world rather than a second handwritten stair map.

### Portal loop

The three-metre opening in extraction's north wall leads directly into the
window bay left of arrival, through its west wall. Walk through in either
direction: the non-Euclidean connection turns your heading by a quarter turn
and aligns the floors, closing a loop through the level. From the bay, the
south-east passage returns to arrival. The view through the opening shows the
destination room before you cross; no interaction key is needed.

Both endpoints and their `non_euclidean: true` connection live in
`levels/hangar/world.yaml`. The host resolves camera movement and physics;
`generate-hangar-walk` excludes transformed links from local step assistance.

### Crates

Walk into the freight boxes to push them. The original five crates now use
dynamic box colliders. In the loading room beside arrival, each of the three
large crates supports a 0.8-metre crate and a 0.5-metre crate. Push a base to
shift or topple the smaller stack. The host owns gravity and collisions; keyboard
and touch walking use the same bodies. Changing landmarks keeps the crates where
you left them; reloading the game restores the stacks.

### Spatial audio

The reactor, lower control-deck computers, upper acid-control computers, burning
barrel and acid pool have positional ambience. The upper console bank uses the
same fan recording and settings as the other computers; fire and bubbling stay
quiet and local. Three separated acid emitters cover the south basin and both
trench branches; each has reduced gain and range so overlapping bubbling stays
subtle. Eight ambient emitters leave room for movement-driven crate scrapes.
Crates create a voice only after accepted horizontal movement, stay
silent during vertical settling, and release idle voices after half a second.
When many crates move, the closest seven share the live crate budget. A silent
idle voice gives up its slot immediately when another moving crate needs it.
Retiring voices still count during their stop fade, keeping all ambience and
crate playback within sixteen persistent voices.

Distance uses XYZ, stereo panning follows camera yaw, and collidable walls or
obstacles between a source and the listener lower its volume and soften high
frequencies. Opening a path restores the clear sound. Browser audio begins after
a user gesture. These are simple obstruction checks; the sample has open passages
and no working door mechanisms. Crate speed and retirement timing use the host's
ordinary 60 updates per second; playback itself runs on the audio device. Moving
crate voices repeat their idempotent upserts during catch-up updates, so a voice
temporarily deferred by a host stop fade can recover without restarting an
already playing clip.

`scripts/hangar-sound-assets.go` prepares seamless WAV loops from retained recordings.
Source credits and licenses live in `assets/sounds/README.md`. Regenerate with
`mise run generate-hangar-sounds` and verify with `mise run check-hangar-sounds`
from the CLI root. Playback setup and lifetime live in `src/audio.go`; no engine
source is needed to build the sample.

### Step support

Walking uses `Body.SetHorizontalVelocity` for horizontal velocity, including zero velocity
when input stops, while the host retains vertical momentum, gravity and crate
contacts. Ordinary movement and descent do not teleport the camera box. Only a
grounded upward riser of at most 0.25 metre receives a lift at the current X/Y,
before the full 0.6-metre-wide footprint reaches its edge. A short supported
transfer holds 0.15 metre of contact clearance while the full footprint crosses
the riser; this prevents the solver's speculative edge contacts from stopping
walk/run movement. Static speculative contacts include the distance travelled
in one update: at running speed their reach is 0.13 metre, so the gap keeps the
player clear until crossing. Further risers are measured from actual floor heights,
independently of that gap. The lift ends when the full footprint reaches the
target floor. A bounded support witness then lets gravity settle the gap without
position updates, while preserving eligibility for the next actual riser; it
retires once grounded. Assistance cancels on stopping, reversing, losing support,
input reset or changing views. Twelve updates without forward progress, or 120 updates on one transfer,
also cancel assistance so a dynamic obstacle cannot hold the player aloft.
A grounded box straddling an upper stair corner can restart its transfer when
walking toward that higher floor through its open portal. Walking back down the
edge receives no lift. A stalled or unreached target remains suppressed until
input stops/resets or the full footprint actually crosses it, preventing repeated
lifts against a movable obstruction.
Open portal lookahead includes 0.025 metre for speculative contact margin and
slop, so it identifies the next tread before horizontal collision stops the box.
The support helper checks connected directed portals, solid walls, whole-footprint headroom and
static collision solids; airborne poses, high ledges and window sills receive no
lift. Both walking and running use the same checks. The box dimensions stay
unchanged; this sample assistance is not a general capsule character controller.

`scripts/hangar-walk-assets.go` owns `internal/fps/walk_world.generated.go`.
Regenerate with `mise run generate-hangar-walk` after changing authored geometry
and verify with `mise run check-hangar-walk`. The generator uses the public CLI
world compiler and SDK formats, and retains authored floor and ceiling planes,
including slopes. The host resolves continuous ramp motion without positional
step assistance; collision-solid clearance still requires flat caps. Transformed portals are ineligible for local step
assistance. Runtime support queries allocate no memory.
`mise run check-hangar-fps-allocations` verifies the elevator and stepping helpers
with actual TinyGo WASM heap counters, including checks that native Go misses.

### Torch

**F** toggles a warm camera-mounted torch; its **Torch on/off** button also works
on touch devices. The torch sits 0.35 world units to the camera's right, 0.20
forward and 0.30 below it, giving its beam a handheld perspective.
The wider beam follows your position immediately while its direction eases
behind turns with a 0.25-second smoothing time constant.
Changing landmarks or stopping the game removes the torch; it keeps its on/off
setting across landmarks. The torch adds live illumination alongside the
existing baked room lighting. Torch shadows are disabled because spotlight
shadow maps require ordinary Euclidean portals. The non-Euclidean loop keeps
the live beam and the room lighting.

### World and materials

Change the rooms and lights in [world.yaml](levels/hangar/world.yaml), materials
in [level.toml](levels/hangar/level.toml), or the nearby texture PNGs. Rebuild to
see your changes. The reactor bands and green acid emit their albedo color with gentle pulses.
Tune `[textures.emission]` intensity and `[textures.emission.pulse]` in
`level.toml`; the acid keeps its liquid ripples. The unlinked `reactor-base` light
contributes cool white/blue reactor light, while `acid-base`, `acid-east-base` and
`acid-south-base` contribute green acid light across both banks, the right wall and
the south wall opposite extraction, with spill onto the ceiling.
The offline bake carries those colors onto surrounding surfaces through three diffuse
bounces. The three local reactor lights and two acid fill lights add
illumination that follows the corresponding emission pulse. The reactor dims to
20% of peak over a 3.2-second cycle; its lower emission intensity avoids a
permanently clipped white/cyan appearance. Emission
has no bloom halo. Use `lights` under `[textures.emission]` to link existing light
IDs. The offline build automatically bakes all 28 steady sources; the
six linked pulsing sources, including the barrel lamp, remain live alongside the torch. No manual
static-light allowlist is needed.

### Material detail

Eight suspended strip fixtures give the main rooms visible overhead light sources.
Seven working fixtures have brushed housings, end caps, two ceiling hangers and
steady emissive opal diffusers, including the adjacent window bay. The arrival
fixture by the ventilation fan is broken: cracked dark diffuser remnants flank
an exposed channel, with no emission or point-light contribution.
`scripts/hangar-ceiling-lights.go` places each working diffuser just above
its existing point source and mounts the hangers to that room's ceiling. The
arrival's ventilation fill moves up to the same suspension height as its other
strip; the broken fixture's source is zeroed. Other source colors, radii and
intensities stay intact. These fixtures are nonblocking static geometry and
participate in the offline bake. Diffuser
emission makes the strip visible; the existing point lights illuminate the room.
Run `mise run generate-hangar-ceiling-lights` to regenerate placements and
`mise run check-hangar-ceiling-lights` to check drift. Builds and bakes regenerate
the fixtures before using their geometry.
`mise run capture-hangar-ceiling-lights` captures arrival, reactor, loading and
window-bay views, including the damaged fixture, with the selected native host.

The barrel and reactor separate painted panels from bare brushed trim. The
barrel's upper hoop has small sloped edge bands; the reactor cap has vertical
corner chamfers with flat collision caps. `mise run generate-hangar-heroes`
updates only those owned blocks and material bindings; `mise run
check-hangar-heroes` verifies them. The reactor's broad fills are narrower and
quieter, preserving the local lamps and emission as the visual focus.

Ordinary floor plates, ceiling grids, grates and ribbed panels use restrained
procedural variants with softer seams, lower material relief and finer wear.
Their original PNGs remain alongside the variants. The checked-in
`scripts/hangar-material-assets.go` owns the four `*-quiet.png` files; regenerate
with `mise run generate-hangar-materials` and verify with
`mise run check-hangar-materials`. Ceiling repeats span three metres, while
hazard stripes, machinery accents and emissive surfaces retain their contrast.
The reactor vessel alone uses `reactor-ribbed`, an alias of the original ribbed
artwork and full material relief, so its vertical structure remains the focal
point while ordinary casings and door surrounds use the quieter variant.
Two green console pools and two warm loading-hall lamps keep those rooms
readable with the torch off; ambient lighting stays low.

The main olive wall panels use a larger 3-by-4-metre texture repeat. Twenty-three
rooms layer the neutral `wall-weathering` secondary over their existing wall
materials, with an independent 9-by-7-metre repeat for broad stains and uneven
wear. The reactor uses a lower blend strength; the brick loading and acid areas
use stronger weathering. Six main rooms also use a 6-metre floor secondary.
These settings live in each room's `wall_uv`, `wall_secondary` and
`floor_secondary`; scale specifies world metres per texture repeat.

Top and bottom `wall_bands` reuse the existing steel material and its generated
normal/height/AO data. Larger rooms have 0.28-metre cornices and 0.48-metre lower
protective strips; narrow service passages use 0.18/0.30-metre strips. Bands
follow the visible wall spans around portal openings. Stairs, recessed window
frames and liquid surfaces keep their existing materials.

The level caps sprite detail at the standard 128 texels per world unit.
Fan, rust and piston images are downsized
during building using their smallest placement; source files stay intact.
The three sign textures opt out to keep lettering readable.

Five fixed, alpha-blended `rust-drips` sprites add localized marks beneath the
arrival/loading vents, beside the reactor service pumps, and on two acid walls.
They sit 0.01 metre inside the room and use actor lighting. Their placements are
authored once; they send no per-frame transform updates. Their source PNGs stay
beside the world source.

After changing materials, run `mise run bake-hangar` from the CLI repository
root before rebuilding. `mise run capture-hangar-materials` captures five detail
viewpoints in one native session for visual review; set `KARTY_HOST_NATIVE` to
select a local matching host.

The checked-in sample keeps unbounded lighting transport. When opting into
chunk builds, an explicit
`bake_max_ray_distance = 16` in `[lightmap]` enables local pixel-cache reuse;
it also deliberately limits each transport segment to 16 world units. Restore
the project `.karty/bakes/` directory in CI to retain historical pixel entries.

### Machine and surface animation

The reusable animation library
in `levels/hangar/world.yaml` gives terminal screens a short pre-authored glitch
burst every three seconds, ripples the stationary green acid surface,
spins compact fans inside recessed steel cases in arrival, control and loading,
and cycles three service pumps along the reactor's west wall. Their moving heads
retract downward by 0.25 units into the floor housings, with each rod remaining
embedded throughout the stroke. They share one preset with phases 0, 0.6 and
1.2 seconds over a 1.8-second cycle.
The floor housings connect to a shared wall manifold. A shared case prefab
provides the rigid sides, grille and lamp; fan phases vary by room. Sprite lighting
is enabled so moving parts respond to the room lights. Each terminal has a
different phase. Motion is visual; collision and gameplay positions remain static.
Glitch frames are full textures packed into ordinary material slots. Acid waves
move only the rendered slab tops. The acid uses smooth alpha blending and normal
texture filtering so the basin floor shows through without the stipple/pixel grid
that produced moiré. Nine convex slabs fill the south basin and both trench sides
at a rest level of -1.10 above basin floors at -3.50. Opacity is 0.70, so the floor
remains visible through the green liquid. Their sides and undersides use a dedicated
opaque atlas material with a zero-opacity liquid preset, leaving an open visual
surface without dark strips between slabs. The 0.045 wave height stays below the raised zigzag deck. Exit ramps pass
through the waterline; collision and baked lighting use the authored rest geometry.

From the CLI repository root, regenerate the sample animation assets with
`mise exec -- go run scripts/hangar-animation-assets.go`. The generator reads the
original `screen.png` and reproduces the glitch frames and piston without external
tools. The [detailed rotor artwork](levels/hangar/fan-rotor-detailed.png) is a
separate source transparent PNG.

### Reused industrial effects

The [Particle Lab](../particle-lab/README.md) supplies the barrel's compact fire
and particle-driven dark smoke, a periodic blue discharge on the reactor's front-left
vessel surface, and four bubbling acid patches around the basin. These use the
same single-image masks as the reusable library. The Hangar importer widens the
barrel flame/smoke and tongue spawn footprints across the coal bed and enlarges
the tongue masks; their vertical velocities and lifetimes remain unchanged.
The shared library and its other consumers retain their original recipes.
The reference fire remains a visual prototype awaiting appearance acceptance;
its successful builds and renderer checks do not establish final visual approval.
The low barrel ceiling limits its complete small-fire placement to scale 0.75;
its smoke ends below 2.60 metres. Acid patches remain above the rippling liquid
and fit their individual convex sectors. Reactor arcs, charge, flash and fallout
share the existing 3.2-second reactor cycle.

[generate-effects.go](generate-effects.go) copies the selected recipes and all
particle dependencies from the handwritten library into marked
blocks of `world.yaml` and `level.toml`. Edit the lab library for shared recipe
changes, or this importer for Hangar placements. Room geometry, other prefabs,
liquid animation and the coal texture's emission-linked barrel lamp remain
handwritten. The imported PNGs live under the level's confined `textures/effects/`
directory. Fire and smoke source textures carry CC0 provenance in the lab;
shared acid/discharge masks are generated by its checked-in Go source.

From the CLI repository root:

```sh
mise run generate-particle-lab
mise run generate-hangar-effects
mise run check-hangar-effects
mise run check-hangar-schema
mise run check-hangar-fps-allocations
mise run build-hangar-web
```

Choose **Burning barrel**, **Reactor** or **Zigzag** in the Camera menu and observe
the live preview for several seconds. **F** disables the torch when comparing
the authored effect colours. `mise run capture-hangar-particles` captures the
barrel and basin viewpoints in a native session; those startup stills supplement
the live animation review. See [particle authoring](particle-authoring.md) for
placements, resource budgets, source ownership and current rendering limits.
Keyboard and touch checks use synthesized browser input. Physical-phone frame
rate and real touch-device behavior remain separate qualification work.

### Observation-room elevator

The west stair flight is now one hazard-striped sector floor,
`acid-control-west-lift#0`, between the reactor-side service tunnel and the
observation room above the acid. It rises from 0.20 to 3.40 metres, pauses for
three seconds at each landing, and eases through each four-second trip. Walk
aboard while it is at your landing; host physics carries you. The east stairs
remain available. The shaft walls and ceiling stay fixed.

`src/elevator.go` sends `World.SetFloorZ` and updates the local step-support map
from the same Z. The mesh and collider translate without rebuilding. This needs
the project's selected SDK and a host with sector-Z support. The moving floor
uses live lighting; the room's baked lighting and static shadow proxies retain
their authored state.

## Capture the views

From the sample's folder, capture the included viewpoints in one native session:

```sh
../../dist/karty render screenshot hangar --batch screenshots.toml
```

Each capture prints its image path. Use `karty render screenshot --help` to choose
your own angle.

## Reusable controller

This sample uses the [first-person preset](../../packages/controllers/README.md)
from the shared `controllers` package. Input state and camera calculations are
portable; this game owns its SDK adapter, entities and gameplay. Normal builds
restore the declared package automatically.

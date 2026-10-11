# 🏭 The Hangar

**Follow the light into the reactor.**

Look through the service windows, climb to the control deck and cross a zigzag
above a recessed trench. Warm lamps, cool reactor light and deep shadows shape
an original Doom-inspired industrial world. Every passage is open for exploration.

The acid chamber has a nine-metre ceiling. An elevated control room on its south
wall, opposite extraction, overlooks the basin and zigzag through two five-metre
windows. Reach it by branching south from the reactor-to-acid tunnel, or from the
eastern trench stairs and their service tunnel. Both approaches climb in
0.2-metre steps to the 3.4-metre control floor. The Camera menu includes
**Acid control**; the screenshot batch includes the overlook, windows and entrances.
`mise run capture-hangar-control` captures those views plus both stairways and the
consoles. The lightmap uses four texels per metre to fit the taller chamber and
new passages into the existing single-page budget; the control lamps stay local.
The pillars reach the ceiling, and the beam nearest the windows is removed to
keep the view open. The sample controller follows these stair heights and keeps
walking inside the window sill; its floor footprints in `internal/fps/acid_control.go`
match the authored rooms.

[More demos](../README.md)

## Take a look around

**WASD** walks; click the scene to look with the **mouse**, and press **Esc** to
release it. Hold **Shift** to run; arrow keys also look. On a smartphone, use the
**left joystick** to walk and the **right joystick** to look. They appear on first
touch. Open **Camera** at the top right to choose a landmark; the panel starts
collapsed and closes after a selection. **V** jumps to the next landmark, and
**H** hides the interface.

**F** toggles a warm camera-mounted torch; its **Torch on/off** button also works
on touch devices. The torch sits 0.35 world units to the camera's right, 0.20
forward and 0.30 below it, giving its beam and shadows a handheld perspective.
The wider beam follows your position immediately while its direction eases
behind turns with a 0.25-second smoothing time constant.
Changing landmarks or stopping the game removes the torch; it keeps its on/off
setting across landmarks. The torch adds live illumination alongside the
existing baked room lighting. Opaque walls and static solids, including crates
and machine bodies, cast approximate height-aware shadows from its beam.
These affect the torch's contribution while the baked room lighting remains
visible. Transparent acid, sprite artwork and animated parts do not cast torch
shadows.

## Run it

This sample currently requires candidate **SDK 0.0.11** and a matching host
advertising `world/particles@1`. After [setting up the tools](../README.md#make-it-yours)
and [installing that SDK](../../docs/sdk-bundles.md#install-and-trust), run from the
CLI repository root:

```sh
mise run check-hangar-schema
mise run bake-hangar
mise run build-hangar-native
mise exec -- ./samples/hangar/dist/native/karty-host --cartridge samples/hangar/dist/native/game.kart
```

For a browser preview, from the same root:

```sh
mise run build-hangar-web
mise exec -- ./dist/karty serve samples/hangar/dist/web --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080/`. Local candidate builds can select hosts with
`KARTY_HOST_NATIVE=/path/to/karty-host` and
`KARTY_HOST_WEB=/path/to/karty-host.wasm`; keep its matching `wasm_exec.js`
beside the web host. The selected SDK, host and cartridge must agree.

## Capture the views

From the sample's folder, capture the included viewpoints in one native session:

```sh
../../dist/karty screenshot hangar --batch screenshots.toml
```

Each capture prints its image path. See [camera options](../../docs/screenshot.md)
to choose your own angle.

## Make it yours

Change the rooms and lights in [world.yaml](levels/hangar/world.yaml), materials
in [level.toml](levels/hangar/level.toml), or the nearby texture PNGs. Rebuild to
see your changes. The reactor bands and green acid emit their albedo color with gentle pulses.
Tune `[textures.emission]` intensity and `[textures.emission.pulse]` in
`level.toml`; the acid keeps its liquid ripples. The unlinked `reactor-base` light
contributes cool white/blue reactor light, while `acid-base`, `acid-east-base` and
`acid-south-base` contribute green acid light across both banks, the right wall and
the south wall opposite extraction, with spill onto the ceiling.
The offline bake carries those colors onto surrounding surfaces through two diffuse
bounces. The three local reactor lights and two acid fill lights add
illumination that follows the corresponding emission pulse. The reactor dims to
20% of peak over a 3.2-second cycle; its lower emission intensity avoids a
permanently clipped white/cyan appearance. Emission
has no bloom halo. Use `lights` under `[textures.emission]` to link existing light
IDs. The offline build automatically bakes all 24 steady sources; the
six linked pulsing sources, including the barrel lamp, remain live alongside the torch. No manual
static-light allowlist is needed.

### Material detail

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
[chunk builds](../../docs/chunk-levels.md), an explicit
`bake_max_ray_distance = 16` in `[lightmap]` enables local pixel-cache reuse;
it also deliberately limits each transport segment to 16 world units. Restore
the project `.karty/bakes/` directory in CI to retain historical pixel entries.

### Machine and surface animation

The sample selects SDK 0.0.11 and world source v10. Its reusable animation library
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
at a rest level of -1.10 above basin floors at -1.20. Opacity is 0.70, so the floor
remains visible through the green liquid. Their sides and undersides use a dedicated
opaque atlas material with a zero-opacity liquid preset, leaving an open visual
surface without dark strips between slabs. The 0.045 wave height leaves at least
0.055 clearance from the floor and lowest stair tread; collision and baked lighting use
the authored rest geometry.

From the CLI repository root, regenerate the sample animation assets with
`mise exec -- go run scripts/hangar-animation-assets.go`. The generator reads the
original `screen.png` and reproduces the glitch frames and piston without external
tools. The [detailed rotor artwork](levels/hangar/fan-rotor-detailed.png) is a
separate source transparent PNG.

### GPU sprite particles

Green sparkles float above the acid pool. A burning steel barrel in the inspection
alcove, immediately east of arrival, gives off short orange flames and rising
embers. Choose **Burning barrel** in the Camera menu to inspect it. The alcove
stays open for walking, and its particle envelopes fit below the low ceiling.

The `particles` library in `world.yaml` owns the presets, seeded emitters, cyclic
emission, particle lifetime, velocity ranges, acceleration, drag, flipbook frames
and size/color curves. The GPU evaluates each sprite from its age and seed; the
game client sends no particle movement commands. Each acid emitter stays in one
basin sector. The drum's hot-coal top links the nearby warm lamp to the existing
emission pulse independently of the particle animation.

The original white alpha masks and hot-coal texture are generated entirely by
the checked-in [asset generator](generate-particle-assets.go). Regenerate them
from the CLI repository root:

```sh
mise exec -- go run samples/hangar/generate-particle-assets.go
```

From the CLI repository root, capture the barrel, its arrival view and both
sides of the acid pool:

```sh
mise run capture-hangar-particles
```

On headless Linux, use `mise exec -- xvfb-run -a mise run capture-hangar-particles`.
The five views in `particle-screenshots.toml` are passive startup captures;
observe the live preview for several seconds to review established birth/death,
upward movement and flame flipbooks. See [particle authoring](particle-authoring.md)
for tuning, reproducible assets and the current rendering limits.

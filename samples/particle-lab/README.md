# Particle lab

An empty 24 by 24 metre room for studying reusable industrial particle effects.
The ceiling is eight metres high. Plain materials and neutral ambient lighting
keep the particles readable. There are no props or camera torch.

Every emitter layer uses **one static sprite**, and each reusable effect has
**at most ten layers**, including smoke. Particle births, motion, growth, spin,
colour and fading create the detail over time. Effects use no animated sprite
actors or flipbook animation resources.

The sample requires **SDK 0.0.12**, source world version 11 and
`world/particles@2`. SDK 0.0.12 is a release candidate; use its matching local
bundle and host.

## Controls and review rooms

The sample opens on the adapted Brackeys **Combined** fire. Use the visible
**Fire**, **Smoke** and **Collection** buttons. Fire offers **Combined**,
**Separate** and **Compare references** views; Smoke offers **Thin**, **Rolls**
and **Compare smoke**. Collection offers **Sparks**, **Acid**, **Reactor**,
**Discharge**, **Explosion** and **Overview**.

**Restart** remounts the current level and restarts its effects; **Center**
resets the camera. **Left/Right** rotates around the selected effect, and
**Near/Far** zooms. Drag in the scene to rotate or pinch with two fingers to
zoom. Comparison views collapse the panel; **Controls** reopens it and
**Hide controls** clears the view. The responsive panel supports portrait and
landscape. The host evaluates particles; the cartridge does not update them
individually.

[room.yaml](levels/lab/room.yaml) owns the empty collection room. Generated
levels provide the collection, reference fire and standalone smoke scenes. Smoke has a neutral grey
backdrop for judging transparency. Combined and Separate occupy the same
reference level: selecting either close view changes the camera, so all six
layers remain mounted. Its actual finite schedules use **177 GPU slots**;
their declared capacities sum to 180.

From the CLI repository root:

```sh
mise run generate-particle-lab
mise run check-particle-lab
mise run check-particle-lab-schema
mise run build-particle-lab-web
mise run serve-particle-lab
```

Open <http://127.0.0.1:4245>, or use `VSCODE_PROXY_URI` in a remote code-server
workspace. Regenerate and rebuild after changing authored inputs.
[room-screenshots.toml](room-screenshots.toml),
and [smoke-screenshots.toml](smoke-screenshots.toml) retain passive native viewpoints.

## Authored inputs and reusable recipes

Edit [library.base.yaml](effects/library.base.yaml) for retained recipes and the
following fragments for the revised collection. **Do not edit
[effects/library.yaml](effects/library.yaml) by hand**: [generate-library.go](generate-library.go)
composes it from the base, these four fragments and the reference fire.
[generate-lab.go](generate-lab.go) then assembles the rooms, texture manifests
and selected assets. The generated library contains the composed reusable recipes.

| Fragment                                                   | Reusable prefab             | Behaviour                                                                         | Layers | Declared capacity |
| ---------------------------------------------------------- | --------------------------- | --------------------------------------------------------------------------------- | ------ | ----------------- |
| [acid.yaml](effects/acid.yaml)                             | `industrial-acid`           | Small growing bubbles, brief pops, ballistic droplets and restrained green vapour | 5      | 46                |
| [electric-sparkles.yaml](effects/electric-sparkles.yaml)   | `industrial-weld`           | Short contact flashes, scattered dashes, cooling chips and faint filaments        | 7      | 54                |
| [electric-discharge.yaml](effects/electric-discharge.yaml) | `industrial-discharge`      | Charge, brief contact flashes, staggered segments, branches and fallout           | 7      | 64                |
| [explosion.yaml](effects/explosion.yaml)                   | `industrial-discharge-once` | Flame impulse, expanding smoke, fragments, debris and outward sparks              | 5      | 162               |

The Sparks and Explosion selectors retain existing prefab IDs. Capacity is the
preset's declared upper bound; it is not the number of visible particles.
Discharge's finite schedules use 63 slots within its declared capacity of 64.
It repeats every 3.2 seconds and starts flashing near 0.82 seconds. Explosion
starts 1.5 seconds after level readiness, with smoke at 1.65 seconds; tap
**Restart** to replay it.

These are authored adaptations using static masks and primary emitter
references, rather than engine-equivalent imports. Acid disturbances are small
camera-facing rims, not simulated surface ripples. Electric discharge uses
independent sprite segments, not a connected ribbon or a live lightning solver.
Explosion omits the source's mesh shock sphere, collisions and turbulence.

## Reference fire and smoke

The reusable `brackeys-fire-combined` prefab is composed once from the
[converted Brackeys recipe](effects/brackeys-fire-combined.yaml)
and [fire-refinement.yaml](effects/fire-refinement.yaml), then reused by the
reference room, Collection and Hangar. It has three layers, declared capacity 114 and a
finite schedule of 111 slots. Late smoke contribution is reduced and an
additional static tongue gives the rising fire a less circular silhouette.
The reference fire remains a visual prototype awaiting appearance acceptance.
Compilation and renderer checks establish execution, rather than final visual approval.
The retained Separate reference has three layers and 66 slots. These are portable recipes with documented runtime limits, not a visual-parity claim.

Combined uses `sprite.blend: premultiplied`: emitted RGB can remain bright
while curve alpha controls background coverage, allowing one layer to change
from luminous fire into smoke. The supplementary tongue is derived from the
[Pixi Fire.png reference](https://particle-emitter-editor.pixijs.io/assets/images/Fire.png).
Its original file, pinned revision/checksum, static crop-and-padding preparation
are retained in [static-assets.json](effects/sources/static-assets.json); the full
**MIT / CloudKid** notice is in [LICENSE-PARTICLES.txt](effects/sources/LICENSE-PARTICLES.txt). It has no baked animation.

Standalone `industrial-smoke-thin` uses two layers and 20 slots;
`industrial-smoke-rolls` uses three layers and 43 slots. Their comparison
mounts five layers and 63 slots. Source-over alpha, live trajectories, growth,
rotation and fading form the changing plume from two static masks.
The base also retains the earlier authored fire profiles for passive comparisons;
they do not replace the default Combined reference view.

## Reusing effects

Copy the chosen prefab and its referenced presets from the generated library
into your world's `prefabs` and `particles.presets`. Set source `version: 11`
and `particles.version: 2`, then add instances with the desired transforms.
Declare every referenced texture in `level.toml` and copy its PNG into that
level's confined directory. Keep names unique and validate the transformed
motion envelopes. The world format has no YAML import syntax; the handwritten
generators perform composition.

Keep one `sprite.frames` entry per preset, omit image animation FPS and count
every expanded emitter toward the ten-layer limit. Source-over smoke uses
`alpha`; the reference fire uses `premultiplied`, and luminous sparks use
`additive`. No animation preset or ordinary sprite actor is needed.

The check task verifies source hashes offline, reproducible generated
files, source ownership, layer counts, texture packaging, confinement,
capacity and particle codec round trips. Compilation, actual WASM execution,
browser graphics and manual visual review remain separate qualifications.

## Retained sources and runtime limits

The [Brackeys VFX Bundle](https://brackeysgames.itch.io/brackeys-vfx-bundle) is
released under **CC0**. The eleven used masks and two fire recipes are retained directly under
`effects/`, with hashes and attribution in
[static-assets.json](effects/sources/static-assets.json).
The [public fire scene](https://github.com/Brackeys/vfx-in-godot/blob/23b58f272b828b0a894ed7ef72fbf9033ee9d177/vfx/scenes/fire.tscn)
is the reference source; the pack credits Picster and Kenney for textures.
[generate-static-assets.go](generate-static-assets.go) prepares selected masks,
auxiliary static sprites and room textures offline. It does not bake motion.
Pixi-derived tongue artwork retains its separate MIT license and provenance.

The host evaluates deterministic seeded trajectories from velocity, constant
acceleration and linear drag. Point/box spawn, up to four size/colour keys,
random rotation and spin support bounded layers. Alpha, premultiplied, additive
and cutout materials support camera-facing or upright billboards.

The runtime has no particle collision, fluid simulation, turbulence field,
velocity-aligned stretching, ribbons or soft intersection fading. Emitters
belong to their mounted level and sector envelope. Gameplay does not
individually move, trigger or stop them; **Restart** remounts this sample.

Alpha and premultiplied particles draw after ordinary transparency and additive
particles. Emitters keep authored order without per-particle depth sorting or
interleaving with transparent meshes, so overlapping layers and glass/liquid
intersections can show ordering errors. Particles emit no light, cast no shadows
and do not appear in planar reflections. Author separate lights for illumination.
Browser touch checks use synthesized input; physical-phone performance and
real touch-device behavior remain unqualified.

Generated sprites, per-level texture copies, assembled worlds/manifests and
`effects/library.yaml` are ignored. The pinned sample build and test tasks
regenerate them. Before running `karty dev` from a fresh checkout, run
`mise run generate-particle-lab` once from the CLI repository root.

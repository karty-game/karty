# Hangar particle authoring

Hangar reuses the reviewed industrial recipes from
[Particle Lab's library](../particle-lab/effects/library.yaml). World source v17,
`particles.version: 2` and the sample's selected SDK support these
GPU particle presets. Smoke and acid vapour use the GPU particle alpha path.
Version 2 preserves the KWP1 binary layout and adds no guest API commands.
The reference fire remains a visual prototype awaiting appearance acceptance;
the retained recipes and successful execution checks do not imply final visual approval.

## Effects and placements

| Effect                   | Placement origin `(x, y, z)` | Scale     | Layers                                                               | Declared capacity |
| ------------------------ | ---------------------------- | --------- | -------------------------------------------------------------------- | ----------------- |
| Reference barrel fire    | `(14.8, -1.55, 1.1)`         | 0.75      | Combined flame/smoke, embers and shaped tongues                      | 114               |
| Reactor discharge        | `(6.3, 23.05, 3.7)`          | 0.70      | Charge, contact flash, segments, two branch passes, fallout and echo | 64                |
| Twenty-four acid patches | Wet basin polygons below     | 0.80–1.10 | Micro shells, pocket bubbles, droplets, thin vapour and surface pops | 46 per patch      |

The scene declares **1,282 capacity across 130 particle emitters**, with fourteen
shared presets, twenty-six effect prefabs and twenty-six effect instances. Its
finite schedules use **1,278 GPU slots**: barrel fire 111, discharge 63 and acid
1,104. Every layer uses one static image and keeps the reviewed recipe values.
Twenty-three acid adapters vary only phase and seed; instance scales widen the
footprints. Patch phases span 0 to 1.633 seconds; every layer has started by
about 2.53 seconds. Discharge repeats every 3.2 seconds, matching the reactor's
existing emission pulse period. Particle and material-light clocks share the
mounted-level time while remaining independent.

The barrel rim reaches Z = 1.10, and its alcove ceiling is Z = 2.60. The imported
reference fire uses a foot origin at the rim. Its complete moving particle and smoke
billboard envelopes fit inside the alcove. The existing hot-coal solid
surface and warm emission-linked lamp retain their authored behaviour.

The discharge originates just outside the reactor's front-left ribbed vessel
surface, where it is visible from the normal reactor landmark camera. A larger
front-centre candidate crossed a compiled sector seam, so the placement uses a
0.70-scale recipe whose complete envelopes fit one reactor sector. The reactor
uses discharge only; fire remains at the barrel. Opaque vessel
geometry hides sprites behind it; particles do not collide with solids.

The acid rests at Z = -1.10 and waves by 0.045 metres. The wider patches start
above the crests at Z = -0.93, and their full sprite/motion bounds stay above each
basin's Z = -1.20 floor. Keep the liquid's existing animation, transparency and
lighting when tuning the effects. An authored room can decompose into multiple
convex sectors; every emitter must fit one sector for its entire lifetime.

All acid sources use Z = -0.93. The twenty-four patches cover the wet solid
polygons at scales 0.80–1.10, compared with the earlier twelve 0.35-scale sources.
The original bubbles are larger and distributed across substantially wider
footprints; the additional independently phased patches increase bubble density.

| Pool region       | Patches | Scales    |
| ----------------- | ------- | --------- |
| South-west trench | 3       | 0.80–1.10 |
| South-east trench | 4       | 0.80–1.10 |
| East entry        | 4       | 1.05–1.10 |
| West lower bend   | 2       | 0.80–1.10 |
| West upper bend   | 2       | 1.05–1.10 |
| East lower bend   | 3       | 1.05–1.10 |
| East upper bend   | 2       | 0.80–1.10 |
| North pool        | 4       | 1.05–1.10 |

Exact centres, scales, phases and seeds live in [generate-effects.go](generate-effects.go).
Each layer's conservative motion and billboard envelope was checked inside the
liquid footprint, within a compiled convex sector and clear of trench ducts and
columns. The narrow 2×2 west-entry pocket is omitted because it cannot contain a
whole five-layer recipe at these scales. The generator compiles the entire
resolved world and rejects an envelope that escapes its sector before publishing.

## Source ownership and regeneration

[generate-effects.go](generate-effects.go) selects `brackeys-fire-combined`,
`industrial-discharge` and `industrial-acid` from the lab's generated reusable library.
It follows prefab dependencies and particle texture references. All
**11 required PNG textures**
are copied byte-for-byte into `levels/hangar/textures/effects/`; `level.toml`
references those confined paths with the sprite profile.

The importer owns only clearly marked generated blocks for particle presets,
effect prefabs, effect instances and texture declarations; it clears its obsolete
smoke-animation block.
Room geometry, connections, static solids, other actors, lighting and liquid
ripples remain handwritten. The original barrel coal texture is retained.
Source fire and smoke licences/checksums are recorded in the lab's
[provenance files](../particle-lab/effects/sources/).

From the CLI repository root:

```sh
mise run generate-particle-lab
mise run generate-hangar-effects
mise run check-hangar-effects
```

The final check verifies byte reproduction and compiles the complete Hangar to
check all resolved references and analytical envelopes before publishing any
source outputs. Do not edit generated blocks or copied texture PNGs by hand.
Update the lab's handwritten recipe fragments for shared effects, this generator
for placements/phases, and the owning static-asset sources for artwork changes.

## Rendering and review

Particles derive birth, motion, rotation, size and colour on the GPU from
immutable schedules, curves and seeds. The guest sends no individual particle
movement commands. Particle resources and sprite masks upload once at mount.
Fire and smoke perform no recurring particle or ordinary sprite uploads.

The combined fire uses hybrid premultiplied blending for flame light and smoke
coverage; its embers and discharge use additive blending. Acid uses ordinary
alpha blending. Particles are unlit and test opaque depth
and respect sector apertures, but draw after ordinary transparency. Place acid
particles above the liquid so this order reads correctly. Alpha and hybrid fire
materials use the source-over pass after additive particles, with opaque depth
tests and no depth writes. Emitters retain authored submission order; there is no
per-particle sorting or interleaving with ordinary transparent meshes. Charcoal
smoke can attenuate the flame beneath it. Neither path emits light or casts a
shadow. The barrel lamp and reactor material lights provide
separate illumination.

These effects do not appear in planar acid reflections. Neither particle version has arbitrary
forces, turbulence solver, cross-sector particle trails or guest play/stop
commands. Smoke rises, grows, rotates and fades through live particle lifetime curves and
seeded velocities. It has no position-dependent wind or turbulence solver.

Use pinned tasks from the CLI root:

```sh
mise run check-hangar-schema
mise run build-hangar-native
mise run build-hangar-web
mise run capture-hangar-particles
```

Headless native captures need Xvfb and a matching host. The browser runs the
actual game: choose **Burning barrel**, **Reactor** or **Zigzag** and watch several
seconds of established motion. Passive startup stills supplement that review.
Compilation, actual WASM mounting, native graphics, browser graphics and manual
visual review are separate checks; software graphics does not establish physical
phone frame rate.

See the selected SDK's `.karty/docs/particles.md` for field bounds
and renderer limits.

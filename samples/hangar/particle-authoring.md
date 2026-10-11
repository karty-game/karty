# Hangar particle authoring

The sample's [world source](levels/hangar/world.yaml) uses version 10 and
`particles.version: 1`, with candidate SDK 0.0.11. Its three presets drive eleven
acid emitters and two barrel emitters. No gameplay update creates or moves
individual particles. The host uploads schedules, curves and sprite regions at
mount, then the GPU derives each particle from emitter time, its birth index and
the authored seed.

## Effects and tuning

| Preset          | Emission                                                     | Lifetime          | Reserved slots per emitter |
| --------------- | ------------------------------------------------------------ | ----------------- | -------------------------- |
| `acid-sparkle`  | 12 per second for each 3-second cycle                        | 0.9–1.6 seconds   | 48                         |
| `barrel-fire`   | 24 per second for each 2-second cycle                        | 0.40–0.65 seconds | 64                         |
| `barrel-embers` | 8 per second plus six at 0.35 seconds in each 2-second cycle | 0.45–0.75 seconds | 32                         |

The sample reserves 624 slots in total. Capacity prevents a new birth from
reusing a still-living particle's slot; extra capacity stays inactive. Increasing
the emission rate, burst count or maximum lifetime can require more slots.
Cycling schedules must fit their period, and overlapping lifetimes reserve
complete schedule groups. The compiler checks the actual uploaded float32
values, so use its capacity error when tuning near a period boundary.

The acid surface rests at Z = -1.10 and waves by 0.045 metres. Sparkles start at
Z = -0.93, above the crests, then drift upward. Their narrow box supports fit the
compiled sectors around the observation windows and zigzag. To make the effect
more visible, first adjust its `color_over_life` opacity or emission rate; these
do not enlarge its motion envelope. Larger sprites, wider spawn boxes, longer
lifetimes and faster movement all enlarge the envelope and need a build check.

The barrel stands at `(14.8, -1.55)` in the inspection alcove, east of arrival.
Its rim reaches Z = 1.10. Flames start at Z = 1.08 and embers at Z = 1.12;
their complete motion and billboard envelopes fit below the Z = 2.60 ceiling.
The body uses ordinary rigid solid collision. Particles have no collision and
can pass through solids inside their owning sector.

Size curves describe the square billboard edge length in world metres. Their
`age` keys are normalized from 0 at birth to 1 at death. RGBA components use
`[0,1]`, and each size/color curve has two to four strictly ordered keys including
both endpoints. Velocity and acceleration follow the emitter's local axes,
with Z up; positive drag damps velocity. Barrel flames use upright billboards
and a three-frame 12 FPS flipbook. Sparkles and embers face the camera.

Emitter `playback.mode: cycle` repeats emission. `once` finishes its schedule and
lets existing particles drain; it does not restart. A delay shifts the emitter's
start and subsequent cycle phase. Emitter and flipbook playback are separate
settings: a one-shot emitter may use a cycling sprite animation while it lives.
Changing cameras, hiding the emitter or resizing keeps the mounted-level clock.
Release/remount starts a new lifecycle. Seeds select repeatable variation.

See the CLI's [particle contract](../../docs/particles.md) and
[versioned decision](../../docs/decisions/world-particles-v1.md) for complete
authoring fields and format limits.

## Reproduce the artwork

All six new images come from the checked-in
[Go generator](generate-particle-assets.go), using the standard library without
external artwork, downloads or random input:

| Asset                                             | Source dimensions | Purpose                                           |
| ------------------------------------------------- | ----------------- | ------------------------------------------------- |
| `acid-sparkle.png`                                | 96 × 96           | White droplet core and short rays with soft alpha |
| `barrel-ember.png`                                | 64 × 64           | White elongated ember with soft alpha             |
| `barrel-flame-1.png` through `barrel-flame-3.png` | 128 × 128 each    | White flame tongues with soft alpha               |
| `barrel-coals.png`                                | 128 × 128         | Opaque dark coal fragments with orange fissures   |

From the CLI repository root:

```sh
mise exec -- go run samples/hangar/generate-particle-assets.go
```

The generator overwrites only these six files. Maintain its formulas alongside
the PNGs; repeated runs with the pinned toolchain reproduce identical bytes.
White RGB lets the GPU color curves supply green, yellow and orange independently
of the masks. Every frame is registered in [level.toml](levels/hangar/level.toml).

Particle frames must be square, and all frames in a preset need identical
dimensions. Sequences sharing frames form one connected packing group. A group
with multiple distinct frames must fit one 2048 × 2048 sprite atlas page,
including two pixels of total padding in each frame dimension. Texture metadata
is checked before level readiness; a group that cannot fit is rejected before
its first draw. The existing single-texture exception permits an oversized
singleton on a standalone page within the ordinary texture limits. Keep these
masks small. The current `max_sprite_texels_per_unit` pass measures placed actor
sprites and does not automatically downsize particle textures.

## Lighting and rendering limits

These particle sprites are unlit: their texture and color curve determine their
appearance, independently of surrounding room lights and the torch. They do not
emit light, cast shadows or appear in planar acid reflections. The barrel's warm
lamp is a separate authored light, linked to the real `barrel-coals` solid top
through its material emission pulse. The pulse lights nearby walls; the flame
flipbook and particle colors have their own timing.

The sample uses additive blending. Particles test against opaque depth and obey
sector/portal apertures. Additive sprites do not write depth and are drawn after
the ordinary translucent surfaces; they do not participate in sorted smooth
transparency or acquire attenuation from a transparent surface in front of them.
The acid sparkles are placed above the liquid to keep that relationship clear.
V1 also supports hard `cutout` sprites. Smooth smoke and order-independent alpha
blending remain outside this contract.

Each emitter's entire lifetime envelope, including maximum billboard radius,
must fit one compiled convex sector. An authored room with collinear portal
edges can become several triangular sectors, even when its outer shape looks
rectangular. The southern acid placements account for this decomposition.
Particles cannot cross sectors in v1. Builds reject invalid confinement;
retain the authored floors, portals and collision when adjusting an effect.

General changing forces, attractors, turbulence, stateful simulation, moving
emitters that leave world-space trails and gameplay play/stop/restart commands
are outside this version. CPU work still owns emitter resources, clocks,
visibility bounds and draw submission; the GPU owns per-particle animation.

## Build and review

From the CLI repository root, after installing the matching SDK/host:

```sh
mise run check-hangar-schema
mise run build-hangar-native
mise run build-hangar-web
mise run capture-hangar-particles
```

If drum geometry, solid materials or static lighting changed, run
`mise run bake-hangar` before the builds. The lamp linked to coal emission stays
live. On headless Linux, wrap captures with
`mise exec -- xvfb-run -a mise run capture-hangar-particles`.

Captures land in `dist/screenshots/particles-*.png`. The native screenshot host
loads the passive level and GPU renderer without starting the game's UI or
gameplay. It samples startup ages, so it cannot fully demonstrate an established
particle cycle. The [browser launch command](README.md#run-it) runs the actual
game; choose **Burning barrel**, **Zigzag** or **Acid control**, and watch several
seconds of birth/death, upward movement and flame frame changes. The torch can be
disabled with F to inspect the barrel lamp's local lighting.

Compilation, actual WASM mounting, native graphics, browser graphics and manual
visual review are separate checks. The maintained engine validation records
those outcomes; software graphics qualification does not establish physical
phone frame rate or hardware-driver behavior.

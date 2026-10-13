# World particles version 2: GPU alpha and premultiplied materials

Status: implemented in published SDK 0.0.12.

Live smoke needs smooth opacity that darkens the scene while its particles rise,
expand and fade. Version 1 offers additive and cutout blending. Animated actor
flipbooks cannot supply the requested GPU particle lifecycle from one static
texture per layer, so this change adds alpha blending to the particle renderer.

Source version 11 permits `particles.version: 2`, adding `sprite.blend: alpha`
and `premultiplied` beside additive/cutout. The resource remains `@world/particles` with `KWP1`
magic, the same bounded typed binary layout and the common binary header version.
Its explicit document version is 2, and required cartridge/SDK capability is
`world/particles@2` alongside `world/sectors@1`. No command, compiled-world or
particle field layout changes. This work belongs to SDK 0.0.12; released SDK
manifests remain immutable.

Compatibility remains explicit. Source v10 accepts particle version 1 only.
Source v11 supports both versions; v1 continues rejecting alpha and premultiplied materials. The public
`worldparticles.Version` and `Feature` constants retain v1 values. New
`AlphaVersion`, `FeatureV2` and `FeatureForVersion` select v2. Legacy preset
validation/compiler helpers remain strict v1; version-aware helpers receive the
enclosing document version. CLI staging requires and declares that exact
capability. A v1-only SDK/host cannot accept a v2 level.

The engine adds a dedicated premultiplied source-over particle pass after
cutout particles, ordinary transparency and additive particles. It tests opaque
depth without depth writes. Stable emitter submission order is retained; no
per-particle sorting or order-independent transparency is added. Different-colour
transparent intersections have approximate ordering. Constant-colour charcoal
smoke avoids order-dependent overlap within the plume and can attenuate the
previously drawn additive fire.

The premultiplied material preserves the independent RGB and opacity channels
used by Godot's combined flame/smoke effect. For a straight-alpha texture sample
`T` and lifetime colour `C`, the shader emits RGB `T.rgb * T.a * C.rgb` and
alpha `T.a * C.a`, using the same `ONE, ONE_MINUS_SRC_ALPHA` compositor.
Texture alpha confines the sprite; zero lifetime opacity can still emit RGB.
Ordinary `alpha` instead emits RGB `T.rgb * T.a * C.rgb * C.a`. Inputs remain
0–1, without an HDR field or a second sprite/pass per layer.

This matches the source material's hybrid blend equation and its texture import
premultiplication, while leaving Godot lighting, colour management, emission
gains, compression, postprocessing and unsupported simulation features outside
the conversion guarantee. The source [blend definition](https://docs.godotengine.org/en/stable/classes/class_basematerial3d.html#enum-basematerial3d-blendmode)
and [pinned texture import settings](https://github.com/Brackeys/vfx-in-godot/blob/23b58f272b828b0a894ed7ef72fbf9033ee9d177/vfx/brackeys_particle_bundle/particles/alpha/smoke_07_strong_a.png.import)
record why this material is required; copying its mask with ordinary alpha
would suppress the flame's RGB by its deliberately low lifetime opacity.

Each effect layer can use one static sprite. Existing GPU birth schedules,
seeded spawn/velocity/lifetime/rotation variation, acceleration, drag and
piecewise size/RGBA curves provide its live animation. The reusable collection
uses at most ten layers per effect; that authoring constraint does not change
the public resource bounds. The format retains optional flipbooks for other
consumers. No new force, noise, diffusion, stateful simulation or moving-emitter
fields are introduced.

All prior capacity, float32 representability, complete-batch validation,
texture/frame ownership and convex-sector confinement constraints remain in
force. The renderer needs no new render targets, per-particle CPU state or warm
particle-buffer uploads. Tests cover strict v1 rejection, v2/source-version
acceptance, canonical binary roundtrips and invalid-count preflight allocation,
exact SDK capability staging and renderer opacity/depth/portal behaviour.
Compilation, actual WASM execution, browser graphics and visual review remain
separate qualifications.

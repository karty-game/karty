# Cartoon smoke

`smoke-poof.png` is original smoke artwork generated with the built-in imagegen
tool. Rounded gray lobes, pale outlines and spiral curls follow the visual style
of the user's smoke reference; the reference image itself is not shipped.
The exact generation prompt is retained in [smoke-poof.prompt.txt](smoke-poof.prompt.txt).

The source retains its generated transparent alpha. The offline
`scripts/crate-room-smoke.go` preparation downsizes it to 256×256 and creates
16 cosine-eased opacity stages with identical RGB artwork. These small images
share the existing sprite atlas; no per-frame image processing is needed. Run
`mise run generate-crate-room-smoke` to regenerate or
`mise run check-crate-room-smoke` to verify exact bytes. No fire,
flame, spark or additive flash textures are used.

`src/particles.go` animates one main cloud and five small drifting puffs per blast.
They snap open, keep expanding and drifting outward, and fade over roughly
2–3 seconds on staggered timings. They never shrink away. Four
replaceable bursts cap the effect at 24 camera-facing sprites; reset destroys
all of them. These are visual particles without collision or emitted light.

The pinned SDK's mounted GPU particle system has no runtime trigger, stop or
movement API. This game-owned emitter uses `Sprite3D` so smoke can start at an
arbitrary raycast hit without remounting the physics room.

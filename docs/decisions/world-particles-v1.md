# World particles version 1

Status: accepted for unreleased SDK 0.0.11.

World source version 10 introduces a version-1 particle block and optional
room/prefab emitter arrays. Released source versions 1–9 retain their semantics
and reject particle fields even when explicitly null. Presets reference named
level textures; every expanded emitter has a bounded deterministic birth
schedule and owns one compiled convex sector.

Compiled particles use a separate typed `KWP1` payload at `@world/particles`;
the compiled world wire remains unchanged. Required cartridge/SDK capability is
`world/particles@1`, with `world/sectors@1`. Public SDK owns types, bounded binary
codecs, configuration validation, capacity calculations and conservative bounds.
CLI owns strict YAML/schema, prefab transforms, frame resolution, confinement
and data staging. Engine owns resource preparation and GPU evaluation.

The first contract provides point/box emission, rate/bursts, once/cycle playback,
lifetime and initial velocity ranges, acceleration and linear drag, rotation,
size/RGBA curves, square flipbooks and additive/cutout billboards. Slots are
reused only after the largest uploaded lifetime can expire. Derived bounds
include motion extrema, transform, billboard radius and GPU precision/error
margin. Entire effect envelopes must stay within one convex sector.

Gameplay commands, particle collisions, cross-sector movement, smooth-alpha
transparency and stateful simulation are deferred. See [authoring](../particles.md)
and the public SDK `format/worldparticles` contract for exact limits and semantics.

# Solid modeling details

Use this when implementing solid prefabs or diagnosing geometry/material seams.
Resolve exact field names and bounds from the selected project's SDK docs/schema.
The examples below describe the dungeon kit; their dimensions are not universal.

## Extrusions and curves

Karty's solid workflow can represent convex XY footprints between affine Z planes.
An opaque arch is not a flat polygon with an alpha hole. In the current kit,
XZ stone outlines are decomposed into vertical trapezoids, then extruded through
the wall depth. This preserves front, rear, soffit and end surfaces with supported
solid primitives. Keep decomposition deterministic and validate positive height
at every footprint corner; a zero-height tip is not a valid extrusion.

For a round ring with inner radius r and thickness t, use concentric radii r and
r+t. Give the ring full t-width bearings down to the base. Add wall/haunch backing
without filling the opening. A separate impost can project beyond the bearing,
but cannot replace the shaft carrying it.

For a two-centre pointed arch, construct each circular half about its own centre.
Offset each radius by t. Compute where the outer curves actually meet at the
crown; scaling or lifting the inner pointed outline produces the wrong thickness.
Use the same boundary samples for stone outlines and surrounding wall cuts to
avoid slivers, overlap and gaps. Preserve this profile for the matching vault.

A lintel can be one longer stone because it bridges a specific opening; retain
explicit bearing at both ends. A splayed window has a trapezoidal plan reveal
through the full wall, not a facade recess painted on a rectangle. Its interior
mouth and exterior throat are independently measurable.

## Backing and exposed caps

Real block joints need opaque mortar/backing behind them. A single full panel
behind the blocks would close the window/door, so partition backing around the
actual opening: bottom, head and side pieces, with matching splay/curve.

Check all three constraints separately:

1. Mortar closes joints when viewed from front, rear and grazing angles.
2. The full intended aperture stays empty through the wall.
3. Exposed stone sills, soffits and end faces remain stone, without coplanar or
   closer mortar faces covering them.

The AR06 correction recessed mortar caps 12 mm from the opening while preserving
opaque joints. That is a local tolerance example, not a mandatory dimension for
all assets. Prefer clear surface ordering over coincident faces or more tessellation.

Hardware that merely appears adjacent can still float. Independently intersect
its authored footprint/height with its supporting stone or rail. In AR06, side
stiles overlap the jamb by roughly 28 mm; bar ends overlap cross rails. Inspect
both lit and diagnostic views after any correction.

## UV and material decisions

Use [UVs and materials](uv-and-materials.md) for physical repeat scale, course
phase, surface-specific mapping support, map strength tuning and failure diagnosis.
In particular, room triplanar support does not imply solid-side triplanar support.
The current solid contract uses per-edge side fallback and separately mapped caps.
Check actual rotated returns, not only the front facade.

## Cost accounting

For a closed n-sided extrusion with no additional subdivision, authored triangles
are 2(n-2) caps + 2n sides = 4n-4. A box therefore has 12 triangles. This is a
planning formula; confirm costs using the public compiled-level format.

Do not mix these measurements:

- Source shape corners: may ignore UV splits and renderer duplication.
- Compiled renderer-prefab triangles: actual reusable asset definition.
- Triangle corner references: 3 × triangles; not unique vertices.
- Baked static world triangles: may include bake/fan subdivision and hidden-face
  cleanup, plus wall/environment geometry.
- Renderer submitted/visible counts: may count multiple passes and studio objects.

For the current SDK, the kit's mesh-cost helper uses public
`world.SolidRendererTriangles` and compiled level decoding. For a passive WASM
level, the helper uses `level.DecodePassiveModule`; treating the whole module as
a raw level envelope was an earlier failed probe. Use `world.MaterialIDs` for
world material membership rather than counting arbitrary texture entries.
Inspect the selected
SDK before adapting it. A small active prefab does not bypass the total authored
solid/definition limits; keep inactive families out of the preview build if the
accumulated kit approaches those bounds. Preserve all source kit exports.

For actual cost-reduction choices and how to verify them, read
[Mesh optimization](mesh-optimization.md). Counting geometry alone is not an
optimization strategy.

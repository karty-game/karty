# Optimize mesh cost while preserving the asset

Read when an asset is expensive, a baked scene grows unexpectedly, or many repeated
modules approach the SDK bounds. Measure first; a low triangle count does not
prove good frame performance, and a shared prefab does not make every instance free.

## Count the right thing

Inspect the compiled level using the selected public SDK format APIs. Keep the
source object definition, compiled renderer definition, baked world, and submitted
camera/pass counts separate. Report corner references as references, not unique
vertices. The solid-modeling reference gives the closed-extrusion formula.

Track at least per-prefab triangles and solid definitions, material/batch count,
and the expected number of visible instances. Review instance bounds/visibility
and material sampling when frame cost is high despite modest geometry. CPU bake
work, runtime drawing and material-map generation have different costs.

Current pinned examples include a 1,024 solid-definition and 65,536 retained-world-
triangle bound; read current SDK limits rather than treating those as permanent.
The first arch scene exceeded 1,024 solids with 1,383 authored pieces. Displaying
one specimen does not remove unused definitions from its packaged level. Include
only the necessary families in the review level as the kit grows; retain their
separate source prefab exports.

## Reduce cost in this order

1. Remove unnecessary copies and fragments. Share repeated renderer definitions
   and display one active specimen. Eliminate sliver cuts and decorative geometry
   that cannot affect the intended camera view. Instancing reduces duplicated
   definitions/uploads, but repeated visible instances still draw triangles.
2. Merge compatible pieces of the **same intended object** where the union remains
   representable by a valid convex extrusion/affine cap and preserves its visible
   shape, UVs, materials and function. Stop field blocks being split by curve cuts
   once they lie entirely above/outside the arch boundary. Do not merge away real
   block joints, openings, hardware, supports or moving-part boundaries.
3. Reconsider flat field-wall detail if repeated masonry dominates cost. A coursed
   wall material may serve a flat uninterrupted field, while framing, reveals,
   exposed ends and silhouette blocks remain modeled. This is a design tradeoff
   to compare against the approved asset, not an optimization already established
   for the current kit. Match the textured bond to adjacent modeled blocks.
4. Reduce curved-profile sampling where the nearest meaningful view allows it.
   Match samples to deliberate voussoir joints/profile boundaries rather than
   arbitrarily oversampling each stone. Review silhouette, underside normals,
   thickness and positive-height tips after changing samples.
5. Add bevels/chips only where they provide a demonstrated silhouette or edge-light
   improvement. Zero-height or tiny beveled tips caused mesh-coverage failures in
   the kit. Don't subdivide a whole panel to soften a few edges. Small surface
   wear can use maps/decals where geometry is unnecessary.

Karty solids are closed primitives; don't assume an unsupported open mesh or
manual deletion of cap triangles is an available optimization. Don't remove backs
that become visible through another bay, after rotation or with an open door.
Preserve actual full-depth openings and reliable opaque joints.

For a circular chord with radius R and angle step d, the maximum silhouette
error is R × (1 − cos(d/2)). Use the intended viewing distance/FOV to choose an
acceptable projected error, then inspect the real image. This is a starting
budget, not a requirement to hide all faceting in a retro art style. Keep the
same agreed curve for ring, backing and future vault parts.

## Keep static baking separate from asset geometry

The original round panels cost 1,332 / 1,692 / 1,836 renderer-prefab triangles but
2,712 / 3,440 / 3,752 baked static triangles. That difference includes preparation
and subdivision; it does not mean the source mesh doubled accidentally. Inspect
public compiled owners/costs before changing the asset.

A quick unbaked runtime prefab does not establish baked shadows, walkable tops or
FPS collision. On a relevant assembly milestone, validate the static/physics path
separately. Share the same authored shape/material rules between profiles so the
fast specimen remains representative; state lighting/shadow differences explicitly.

The earlier workshop also exhausted a lightmap page and a default chunk mesh
budget. Resolve those with an intentional world partition and a lighting recipe
appropriate to the actual level, within the documented limits. Do not weaken
protocol bounds, silently lower final bake quality or alter test deadlines. The
five-minute asset loop should not rebake the entire dungeon.

## Verify an optimization

Keep before/after counts and fixed-pose images. Compare silhouette, reveals,
contacts, normal response, UV continuity and rear/rotated visibility. Run only the
checks relevant to the changed geometry/material/assembly; retain meaningful
assertions. An optimization is acceptable when the measured cost improves and
the intended appearance/function survives, not simply because compilation passes.
Record remaining limits rather than claiming a whole game/device target from an
isolated asset capture.

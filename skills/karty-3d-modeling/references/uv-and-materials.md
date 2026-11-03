# UVs and consistent material quality

Read when authoring a material, connecting modules, or diagnosing stretching,
pattern mismatch, false seams or an implausible surface response. Resolve the
selected SDK's `material-mapping.md`, `static-solids.md`, `solid-renderers.md` and
`materialize.md` before copying authoring fields between surface types.

## Establish a material contract

For each shared material record its intended use, source/provenance, physical
repeat dimensions, pattern direction, palette, map-generation recipe and runtime
strengths. Keep these values in shared authoring data rather than duplicating
numbers in every family. Compare new assets beside an approved specimen under
the same neutral rig; a new torch color must not hide a quarry/palette mismatch.

Separate three scales of detail:

- Structural shape: openings, wall/reveal depth, bearings, block silhouettes,
  projecting sills and large damage. Model it when it affects silhouette,
  clearance, contact or meaningful occlusion.
- Surface response: grain, small pits and restrained roughness/normal detail.
  Keep the material scale and response consistent across related geometry.
- Context: dampness, soot, algae, wear and localized chips. Add meaningful
  variants/decals at their causes; do not bake every area's mood into its base stone.

Use the same quarry for coursed wall texture and plain cut-stone texture. A
modeled arch block needs plain cut stone, not baked rectangular brick joints.
Different textures may serve floors, timber and iron, but should share the art
reference's level of wear and detail. One family does not need a fresh texture
for each size. Directional timber grain follows the member; cut ends need an
appropriate end treatment rather than stretched longitudinal grain.

## Set scale and phase deliberately

In the current material-mapping contract, `scale` means **world units per repeat**,
not repeats per metre and not a normalized 0–1 fit to the face. A texture showing
six 0.35 m courses represents 2.1 m vertically. A 2.4 m horizontal pattern repeat
should stay 2.4 m on a half module, full module and exposed return. Read the
selected contract before assuming the same semantics in another capability.

Keep scale and phase separate. In this contract offsets are measured in texture
repeats after scale/rotation; world Z anchoring can keep masonry courses aligned
across adjacent modules. A bottom anchor follows each room's floor, which may
shift courses when floor heights change. Use that only when the intended bond
follows those floors. Check a corner, a half-length adapter and an elevation
change against the same course datum. Do not randomly offset mortar patterns;
restrained grain variation on plain individual stones can use a different phase
without moving their modeled joints.

For planar surface coordinates, U follows a chosen surface tangent and V its
vertical/up direction. A single world-X/world-Z facade projection loses usable U
variation on a return lying at constant X. That was the kit's stretched/failed
side-face mapping. Check the return, rear and rotated copy before accepting it.

### Room mapping is not solid-side mapping

The pinned SDK currently documents:

| Surface/path       | Supported approach and useful consequence                                                                                                                   |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Room walls         | Planar edge-tangent mapping; wrap can retain perimeter phase. Choose deliberately for architectural patterns.                                               |
| Solid sides        | Omitted `side_uv` uses geometric per-edge fallback. Explicit side mapping is planar with world anchor; side wrap/triplanar is unsupported in this contract. |
| Solid caps         | Set `top_uv` / `bottom_uv` explicitly when their inherited defaults are unsuitable. The kit uses supported triplanar caps for nondirectional stone grain.   |
| Triplanar surfaces | Useful for irregular nondirectional grain; blends projections, adds samples and does not smooth geometry or align a brick bond automatically.               |

Do not add `side_uv: {mode: triplanar}` just because room surfaces support it.
Likewise, changing root mapping is not proof that a solid's omitted side mapping
uses that root mapping. Read the surface-specific inheritance rules.

The static compiler projects after applying authored prefab transforms. Runtime
solid-renderer prefabs preserve local geometry and move through entity transforms;
verify their actual rotation/scale behavior rather than assuming a world-anchor
flag makes every mapping behave identically. Test the real asset path at 0°/90°,
on front/side/rear/caps and at its intended placement scale. Use several repeats
in the image so phase/stretching is detectable. Resize through authored variants
where possible rather than using nonuniform scaling to fit an unrelated opening.

## Tune response without destroying the source

Generate/cache material maps once per source and recipe. Where the SDK supports
runtime material strengths, tune those controls before regenerating textures.
Change one cause at a time and compare at the same pose/resolution/light.

- **Albedo:** establish neutral-light quarry/value consistency first. Avoid strong
  painted-in directional highlights/shadows that conflict when the rig moves.
  Inspect close, repeated and distant views for recognizable tiling and scale drift.
- **Normal:** start restrained and check both grazing directions. Stone should
  not read as crumpled foil or every noisy texel as a deep pit. Inspect decoded
  surface normals as well as the raw tangent map when the basis is suspect.
- **Height/relief:** enable only if the UVs and actual geometry support it.
  The kit's decomposed arch showed false vertical internal seams with parallax;
  zero height strength removed most of them while normals retained surface detail.
  Height offsets texture sampling; it does not fix silhouette or geometric depth.
- **AO:** keep it restrained and inspect raw material AO independently. Nearly
  white AO is plausible on clean cut stone. Do not darken every groove to simulate
  missing geometry/contact shadows. AO is not opacity; don't apply it twice.
- **Metallic/smoothness:** keep stone nonmetallic; tune fittings to the intended
  bare/oxidized/coated finish. Black albedo alone does not establish a correct
  metal response. Use reflected highlights and raw channels to judge the recipe,
  and do not assume every dark iron coating should have the same metallic value.
- **Rim/specular response:** excess edge brightness can expose decomposition or
  look polished when the reference is matte. Check neutral and grazing views
  before using warm mood lighting to approve the asset.

The approved local limestone recipe uses normal 0.28, height 0, AO 0.25 and rim 0,
with nonmetallic stone. These are that recipe's settings, not universal presets.
The raw height map still exists with runtime height strength zero; the raw map
view is not evidence that displacement/parallax is active.

## Diagnose a visible mismatch

| Symptom                                                | Isolate before changing geometry                                  | Typical correction                                                                                                            |
| ------------------------------------------------------ | ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Front good, return stretched                           | Albedo on side/cap and rotated specimen                           | Correct side projection/scale; use documented per-edge fallback where appropriate.                                            |
| Different brick sizes on adjoining parts               | Physical repeat size and modeled course datum                     | Match quarry, repeat dimensions and phase; use plain stone on modeled blocks.                                                 |
| False vertical seams in one modeled stone              | Fixed pose with height strength zero, then restrained normals     | Fix UV/relief discontinuity; do not add triangles to hide a shading artifact.                                                 |
| Brown/flat sill or soffit despite limestone assignment | Inspect exposed cap and backing depth                             | Recess mortar so the intended stone face is nearest; avoid coplanar surfaces.                                                 |
| Hairline view through wall joints                      | Rear/grazing view and actual gap volume                           | Add opaque recessed backing partitioned around the aperture.                                                                  |
| Flicker at joins/caps                                  | Near versus distant view; overlapping/coincident faces            | Establish clear surface ordering or merge compatible pieces; do not rely on draw order.                                       |
| Pits seem inverted after rotation                      | Compare raw map with decoded surface normal under opposite lights | Check the documented normal convention, mapping orientation and tangent basis. Don't blindly flip a channel.                  |
| Pattern swims/shimmers at distance                     | Same asset near/far, grazing and moving camera                    | Reduce overly fine/high-contrast source detail; verify documented filtering/mips. More polygons do not cure texture aliasing. |
| Only one lighting setup looks acceptable               | Neutral/albedo, then both grazing rigs                            | Correct base value/response or geometry; preserve mood as a separate lighting decision.                                       |

Diagnose with supported controls and actual pixels. Texture padding, mip generation
and normal-basis handling are pipeline responsibilities: if a minimal valid asset
still exposes a defect, record a public reproduction instead of hand-patching
compiled atlases, generated bindings or engine internals.

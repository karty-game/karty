---
name: karty-3d-modeling
description: Build and refine reusable Karty 3D prefabs and material families from reference art, using structurally plausible geometry, a fast inspection workbench, GPU-only visual review and compiled mesh costs. Use for asset-kit modeling and polish; full level design and engine changes have separate workflows.
---

# Efficient Karty 3D modeling

Produce one coherent asset family, compare it with the approved art, and leave a
reviewable result before moving to another family. Optimize the complete
edit → build → capture → inspect → correct cycle, not just screenshot time.

## Start from the actual project

Read contributor instructions, `karty.toml`, the asset inventory and the relevant
reference images. Preserve the selected SDK and unrelated work. Use generated
project documentation/bindings to resolve geometry, material and controller APIs;
do not guess APIs or inspect private engine internals to author a sample.
Never hand-edit `.karty/` or `dist/`. Keep generators and authored kit assets with
the game/project owner, not in engine source.

Identify the next family and its distinct use cases. A useful variation solves
an assembly/context problem: clear span, corner, exposed end, wet treatment,
service access. Avoid nearly identical variants with no placement purpose. Reuse
approved materials and geometry rules before introducing another texture or style.

## Establish and approve the concept before modeling

Create or select the concept art **before authoring the 3D model**. Existing
user-approved artwork can satisfy this gate when it clearly covers this family;
reuse that approval rather than asking again. If there is no adequate approved
target, make concrete concept art, inspect/correct it, and present it to the user
for validation before starting the model. A prompt, inventory entry or successful
image-generation command is not an approved visual target. Silence is not approval.

Validate the concept itself: use case, silhouette, dimensions/scale, plausible
supports and openings, material/quarry language, wear and intended mood. Resolve
contradictory views or impossible construction in the concept first. Record the
approved image/version, approval context and a short measurable construction brief.
Keep the original artwork unchanged and accessible throughout implementation.

Read [Concept approval and comparison](references/concept-validation.md) when
preparing new art, establishing its acceptance criteria or reviewing the model.
For an already approved family, apply that reference's comparison procedure;
do not restart concept generation for every size or fitting adjustment.

## Model the construction

Use deterministic, parameterized generators for repeated solids and assemblies.
Keep geometry helpers separate from specimen placement, materials and the review
scene. Export stable prefab IDs, dimensions, use cases and a small catalog so the
kit can become a package without copying its workshop.

- Make wall thickness, reveals, backs, sills, lintels and bearings real geometry.
  Keep apertures open through their full depth. Close masonry joints with recessed
  backing that does not cover the stone's exposed caps or reveal faces.
- Carry arches into full jambs; measure the ring normal to the curve, including
  springings and crown. A pointed extrados needs offset curves about the same
  centres, not a translated/scaled intrados. Match vault profiles to their arches.
- Seat hardware in a supporting surface. Bars enter rails; stiles enter jambs;
  hinges have a real pivot; fixture arms, chains and feet meet their supports.
  Check contact from more than one side.
- Preserve quarry, course height, bond phase and texture scale between related
  modules. Use plain cut stone on modeled blocks instead of a second brick scan.
- Keep large shape in geometry, surface grain in material maps, and contextual
  wear in decals. Decals follow causes: soot above fire, rust below iron, algae
  at waterlines, growth near moisture/light. Use anchored particle emitters for
  fire; a detailed static flame illustration is not a torch effect.

Read the relevant detail when it applies:

- [Solid modeling](references/solid-modeling.md): valid extrusion decomposition,
  true curves, backing/cap ordering, hardware contact and count interpretation.
- [UVs and materials](references/uv-and-materials.md): physical scale/phase,
  room versus solid mapping limits, rotated returns, shared material recipes,
  diagnostic comparisons and a symptom → correction table.
- [Mesh optimization](references/mesh-optimization.md): reduce fragments and
  profile samples without losing silhouette, support, contacts or UV continuity;
  distinguish source, runtime, bake and repeated-instance costs.

Before accepting a family, compare it beside an approved material specimen under
neutral and opposite grazing lights, including a return/cap and a rotated copy.
Check physical texture scale, course alignment and base-color/roughness consistency.
If a seam appears, isolate albedo, normal and height response before adding geometry.
If cost rises, measure compiled owners and remove needless cuts/copies before
sacrificing visible construction. Keep the before/after pixels and counts.

## Keep iteration cheap

Reuse one persistent `karty dev` process and warmed tool/material caches. Default
to a small unbaked inspection room, one displayed prefab, one retained camera and
one movable inspection light with constant fill. Share geometry; swap the active
renderer instead of duplicating a whole gallery as baked world geometry.
Use the supported free inspection controller for appearance review. It does not
establish FPS capsule collision. Keep the baked/collision scene as a separate
profile for relevant assembly or milestone checks.

Do not rebuild the engine, rebake a level, restart Chromium or recreate the camera
for each view/light/material change. Do not run a second build beside dev against
the same output/cache. For a production snapshot, pause only the owned dev process,
build once, pin the whole distribution, complete the review, then restore preview.
Avoid source edits during a dev capture; rebuild banners/reloads can spoil images.

Target a complete incremental family pass of about five minutes. Record elapsed
time from the first edit through visual inspection/export, with build and capture
subtimings. New concepts, materials or complex moving mechanisms may take longer;
report that honestly rather than dropping views or calling capture time total time.

## Review on the GPU

Software renderers are prohibited for this workflow. Enable hardware acceleration,
disable software fallback, verify the actual renderer and fail the graphics check
if hardware is unavailable. Never substitute SwiftShader, llvmpipe or softpipe.
The Linux AMD/EGL configuration and existing workbench commands are documented in
[Review workbench](references/review-workbench.md).

Capture the relevant angles: front, three-quarter, rear, underside, bearing,
rotated placement, rotated bearing and material/attachment close-up. Adapt the
camera to the asset: a taller crown needs a higher inspection pitch; a grille
needs a close view inside the opening, not a close-up of the adjacent stone.

For each angle, capture several useful lit setups (neutral, grazing from both
sides, warm/low light), then capture each lighting-independent diagnostic **once**:
albedo, decoded surface normal, geometric depth and available raw material maps
(normal, AO, height, metallic, smoothness). Check the selected SDK/host supports
these modes. Label raw material AO separately from scene/contact occlusion; do
not fabricate an unavailable diagnostic. Never cross-product normals/AO with lights.

Use ordinary controls or the host's public debug controls to switch views. Wait
for accepted updates and key releases; assert the accepted pose/output/material
mode rather than trusting a keypress. Retain the GPU context. Export a selector
or contact sheets so the full review is quick to scan.

Open the resulting pixels and place them beside the **original approved artwork**,
using comparable framing and a mood-light capture as well as neutral diagnostics.
Compare silhouette/proportions, support, masonry scale, material response, wear,
hidden sides and attachment details against the recorded brief. Record specific
matches and mismatches; don't substitute a previous 3D screenshot as the target. Contact sheets provide coverage;
open ambiguous joints/caps/hardware at full resolution. Correct a demonstrated
mismatch and recapture affected evidence. Passing capture code is not visual
acceptance. For contact/clearance doubts, sample actual authored geometry or use
public runtime queries; compilation alone cannot establish them.

## Finish a family

Run the generator's drift check when available, `karty schema --check`, formatting
and lint after regeneration, and the configured production compiler at completion.
Inspect actual compiled prefab costs through public format APIs. Keep source,
baked, visible/submitted triangles and vertex references distinctly labeled.
Do not run unrelated repository gates or full bake/traversal suites for every
surface adjustment; follow repository requirements when module/runtime code changes.

Leave the approved concept/version and comparison verdict, current images,
concrete mismatches, counts, GPU identity,
artifact hashes and exact checks/timings together. Preserve previous family
reviews. Keep local concepts/evidence ignored and out of build dependencies.
Do not publish a kit, advance to another family or claim user acceptance unless
that action is authorized. When pausing, record the next inventory item and how
to reopen the workbench, rather than starting the next asset.

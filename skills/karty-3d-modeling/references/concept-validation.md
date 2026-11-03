# Concept first, then validate the model against it

Use this before modeling an asset family and during its visual acceptance review.
An approved concept is the implementation target, not decoration for the report.

## Establish a concrete target

Check the existing reference pack and approval history first. A previously
approved kit sheet or asset concept can cover multiple related variants. Identify
which image and part of it governs this family; preserve its palette, material
language and construction rules. Don't generate replacement art or request the
same approval again when an adequate target is already approved.

If the visual target is missing or does not define a model-critical feature,
create the concept art first. Use the available image-generation workflow for
new bitmap concepts and follow its instructions. Include the intended use case,
scale, silhouette, supports, openings, material treatment and relevant neighboring
kit pieces. Show views that resolve the actual uncertainty: typically a readable
front and three-quarter view, with a rear/section/detail when depth or attachment
is ambiguous. An area mood painting gives context but may need an asset detail
if it hides the relevant construction.

Inspect the actual generated images. Check that views describe the same design,
arches have consistent rings and full bearings, a lintel meets its supports,
recesses have a plausible depth, and materials do not switch quarry or block
scale at arbitrary joints. Remove accidental decorative complexity that conflicts
with the intended kit and budget. Do not copy an impossible image literally into
geometry or silently correct its design after approval.

Present the finished concept with a short brief before asking the user to
validate it. State any unresolved design choice or intended simplification.
Complete this concrete review artifact before requesting approval; do not ask
for approval of an unmade concept. For new/unapproved concepts, wait for approval
before modeling. A request to generate art does not itself approve the result,
and elapsed time does not supply approval. Existing session approval remains valid.
If the user explicitly authorizes a different approval process, follow it.

## Preserve the approved baseline

Keep the original image and prompt/provenance, its version or hash, the family
it covers and the approval context. Record a compact acceptance brief:

| Aspect                 | What to establish before modeling                                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| Purpose and scale      | Where the asset belongs, clear opening/overall size and relation to the player or neighboring module.                          |
| Shape and construction | Recognizable silhouette, ring/lintel proportions, wall/recess depth, load path and attachment points.                          |
| Material               | Quarry/palette, physical course or grain scale, finish, normal/roughness character and relationship to approved kit materials. |
| Wear and mood          | Edge softness, chips, dampness/soot placement, and which appearance comes from lighting rather than the base material.         |
| Scope and cost         | Meaningful variants, details assigned to other families, and deliberate simplifications/budget expectations.                   |

A perspective painting is not a dimensioned engineering drawing. Use an explicit
brief to resolve scale/depth uncertainty; don't treat an inferred pixel ratio as
a user-approved exact dimension. If art and construction requirements conflict,
resolve that conflict in a revised concept/brief before modeling the disputed part.

Preserve v1 if a change is needed. Label v2, explain the visual/functional change,
and obtain approval when it changes the agreed design. Never regenerate or edit
the reference to make an accidental 3D result look compliant. A performance
constraint can justify proposing a simplification, not silently moving the target.

## Compare the implementation with the original artwork

Review the model during blockout before spending time on detailed materials,
then again after material/polish work. Use the same approved baseline at both
stages. A screenshot of the previous iteration is useful for regression checking
but does not replace the original concept as the acceptance target.

Keep the original concept and current capture visible together. Match projection,
framing, scale cues and viewing direction as closely as the real camera permits.
Include an approximate concept-mood lighting view for aesthetic comparison and
neutral/opposite grazing views for honest construction/material inspection.
Do not hide missing depth, wrong proportions or material mismatch behind darkness,
post-processing, cropping or a flattering single angle. Diagnostics supplement the
comparison; they cannot establish that the result has the intended visual identity.

Write a short, evidence-based comparison: what matches, the largest visible
mismatches, what will change and what remains deliberately out of scope. Examples:
"The opening and bearing widths match the brief; the stone is too glossy and
its edges are sharper than the concept" is actionable. "Looks good" or a passing
screenshot script is not a comparison verdict. Correct the implementation, then
recapture the same relevant poses and recheck against the unchanged artwork.

Finish with the approved art/version, current images, dimensions/counts, comparison
verdict and remaining deviations together. Automated mode/pose checks and GPU
captures establish reliable evidence; agent review establishes a comparison;
user acceptance is a separate decision. Do not claim a polished or approved
family merely because it compiled, met a triangle budget or survived the matrix.

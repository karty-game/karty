# Pilgrim model and animation source

The active Pilgrim uses a complete animated humanoid, rendered together with
its hood, sword and buckler. The mesh, skinning, original action and equipment
are derived from **Isometric Hero and Heroine** by **Clint Bellanger**, licensed
under [Creative Commons Attribution 3.0](https://creativecommons.org/licenses/by/3.0/).
Source: <https://opengameart.org/content/isometric-hero-and-heroine>, original
`isometric_hero_heroine_blend.zip` (2011 upload). The author's later clarification
on that page confirms the specific hero source files are CC BY.

`pilgrim.blend` retains the selected source meshes, armatures, animation and
packed resources from `clothes.blend`, `male_head3.blend`, `buckler.blend` and
`shortsword.blend`, with Vigil's material colors, orthographic camera and lights.
The original upload also credits D. Sharon Pruitt for distressed leather and
Naraphim for weapon inspiration; see the source page. The selected costume uses
cloth, rather than the leather-armor material.

Vigil modifications: weathered ochre cloth, dark leather trousers and boots,
oxblood hood with a modeled shoulder mantle, dark wood buckler with an aged
steel rim, restrained metal highlights, new lighting and 30-degree camera;
source hand/head poses and a newly authored brisk locomotion cycle using actual
thigh/shin, shoulder/elbow and ankle joints; coherent world-space skin deformation transfer
for attached equipment. The model is uniformly scaled by 1.12. No Diablo art,
models, animation data, screenshots or textures are included. Diablo II was a
visual reference for the whole-body render method and restrained gait.

## Reproduce

Use **Blender 2.79b** (the saved file contains the legacy Blender Internal
renderer; current Blender versions cannot reproduce that renderer). From the
CLI repository root, select the installed Blender 2.79b executable:

```sh
VIGIL_BLENDER=/path/to/blender-2.79b/blender mise run render-vigil-character
mise run author-vigil
```

The ordinary `author-vigil` task reads the retained atlases and does not require
Blender. The optional render task checks the Blender version, renders into
`tmp/vigil-character-frames`, and runs the retained Go atlas assembler.
On Linux systems without a system `libGLU.so.1`, set `LD_LIBRARY_PATH` to the
extracted Blender distribution's `lib` directory when invoking that same task.

`costume.py` applies the costume finish when the retained model is rendered.
The shoulder mantle is a real cloth mesh bound to the torso's `Back` bone; its
shorter front leaves the tunic and belt visible. Its complete skin deformation
is rendered with the body and equipment. Subtle object-space cloth mottling
stays attached to the costume throughout the animation. The `.blend` remains
the attributed base model; the finishing source is deliberately retained as
readable Python rather than an opaque edit to that binary.
The buckler is reduced to 85% of the retained model's size around its existing
handle center, and the left forearm carries it lower to leave the torso visible.

`locomotion.py` solves the complete skeleton before that render. The shoulders
and pelvis counterrotate, the torso has restrained forward intent, the pelvis
absorbs contact and rises over the supporting leg, and flexed elbows carry the
sword and buckler with different amplitudes. Ankle pitch provides heel contact,
toe release and a tucked recovery. Idle uses the same relaxed pelvis target as
the moving mean, with a quieter torso and lowered equipment. Final mesh-floor
normalization is applied after the joint solve; pelvis target values alone do
not establish the final visible head height.

The render uses two CPU threads and writes 192 walk frames and eight neutral
idle frames. `assemble.go` only lays those complete images into PNG atlases;
it does not crop, deform, reposition or interpolate bitmap body parts. Generated
atlases are `scripts/vigil/assets/pilgrim-walk.png` (24 columns × eight directions) and
`scripts/vigil/assets/pilgrim-idle.png` (eight columns). Direction order is south, southwest,
west, northwest, north, northeast, east, southeast.

Every frame preserves the same render root at (96,160) in its 192-pixel cell.
The runtime authoring pipeline derives grounded sprite origins from those
renders while retaining that common projection. Stride is
`0.70 / 0.44 * 1.12` world units per complete two-step cycle, approximately
1.781818. Opposite legs are half a cycle apart, with 44% support and a
0.13-unit swing clearance before uniform model scaling. At the sample's
4-world-unit/second default pace, a complete cycle takes about 0.445 seconds;
at its 5.2-unit hasten pace, about 0.343 seconds. These are a newly posed gait
and matched travel cadence, rather than a faster playback of the old walk.
Idle uses closer fore/aft boot spacing for a relaxed planted stance. Equipment transfers
must evaluate each parent before the next child: deferring updates until the
whole attachment skeleton has changed creates a stale first-frame pose.

For a quick source-only art iteration, pass `--preview` to `render.py` to render
three directions at every third phase plus idle into a separate output folder.
It does not produce a complete atlas input. Every render also writes
`pose-diagnostics.json` beside its frames with the final world-space joint
positions and the pre-normalization mesh-floor correction. Keep the ordinary
pinned full-render task for retained atlases.

The original AI-generated `../assets/pilgrim-sheet.png` is retained as historical
character exploration; it does not supply the active character frames.

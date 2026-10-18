# Art sources and animation

The playable pilgrim uses Clint Bellanger's animated humanoid, rendered together
with its costume and equipment. The [model source workflow](../source/README.md)
contains attribution, license, exact inputs and Blender instructions.
`pilgrim-walk.png`, `pilgrim-idle.png` and `pilgrim-render.json` are the selected
renderer outputs. Ordinary `mise run author-vigil` extracts their 192 walking
and eight idle frames into one set of compact runtime aliases; Blender is only
needed when changing the source render.

[`levels/crypt/walk.generated.json`](../../../samples/vigil/levels/crypt/walk.generated.json)
records the current frame order and grounded origins. This generated validation
metadata stays with the frames for project snapshots; the game does not load it.
The runtime and sight geometry use these same current frames. Per-frame sole
origins preserve the common projected root, while physical movement and sight
retain their own accepted ground position.

The original AI-generated `pilgrim-sheet.png` remains a source study. It does
not supply active character frames. The environment source sheets below and
their prompts also remain unchanged; no Diablo assets or reference screenshots
are included.

- `crypt-materials.png`: original flagstone, masonry, runner and tomb-lid study.
- `crypt-props.png`: original devotional and furniture artwork. Large furniture
  now uses depth-writing solids; small devotional accents use extracted sprites.
- `crypt-floor.png`: the selected 1774×887 flagstone source, packaged with a
  bounded texture size. See [ground material](../../../samples/vigil/design.md#ground-material-and-wear).
- `crypt-decals.png`: transparent dirt, stone chips, blood and wall seepage;
  its [prompt records](material-prompts.md) stay beside the source.

The author regenerates `samples/vigil/internal/dungeon/vision_art.generated.go` from current
character/decoration silhouettes and actual furniture geometry. It also creates
the deterministic small materials, flame cards, contact shadow and cross windows.
The DejaVu Serif display font retains its license in [`samples/vigil/assets/fonts/LICENSE`](../../../samples/vigil/assets/fonts/LICENSE).

The original source prompts follow. Requested dimensions are guidance; the
author reads actual PNG dimensions and preserves alpha.

## Character prompt

Use case: stylized-concept. Asset type: production game sprite sheet for an
original dark medieval Christian isometric dungeon game named Vigil. Create a
precise 8 columns by 8 rows regular grid on a truly transparent background,
square canvas 2048x2048 if possible. Each cell identical 256x256. Each cell shows
one complete full-body same armored Christian pilgrim, steel chainmail, faded
burgundy hood and short shoulder cape, off-white tabard with a small gold Latin
cross, brown leather boots, one sheathed short sword, small round shield.
Realistic late-1990s pre-rendered 3D sprite aesthetic like classic Diablo II;
sober adult proportions, finely textured worn cloth and steel, no cartoon
outlines. Camera elevated 30 degrees, orthographic. EXACT rows: row 1 faces
screen down (south), row 2 down-left, row 3 left, row 4 up-left, row 5 up/back,
row 6 up-right, row 7 right, row 8 down-right. Eight columns show successive
evenly spaced poses of a complete walking cycle in each direction, alternate
legs and arms, restrained cape movement. Each figure same scale, about 190
pixels tall and 95 wide, centered horizontally in its cell, soles aligned to
230 pixels below its cell top, generous padding, no overlap between cells.
Character chest and head readable against dark stone. Soft neutral lighting
from upper left, readable metal highlights; shadows only on figure, no ground
plane or ground shadows. Every cell strictly one character, no captions, no
borders, no lettering, no scenery, no extra props. All 64 cells must be occupied.

## Material prompt

Use case: stylized-concept. Asset type: game environment source texture atlas for
a realistic dark medieval Christian abbey crypt. Square 1024x1024, EXACT four
equal 512x512 square quadrants with no gutters or labels. Upper left: worn gray
limestone flagstone floor, rectangular staggered large slabs, cracked chipped
edges, subtle dirt deep mortar, neutral flat albedo. Upper right: medieval
charcoal limestone wall, roughly dressed rectangular masonry blocks, staggered
courses, weathered chisel marks. Lower left: faded burgundy woven textile runner
with narrow antique gold embroidered borders at left and right edges, a worn
small gold Latin cross centered, medieval cloth rough weave. Lower right: pale
aged carved limestone tomb lid seen straight overhead, inset rectangular bevel,
centered shallow-relief Latin cross, lightly cracked stone. Every quadrant fills
its square edge to edge, no perspective, no cast shadows, no dramatic lighting,
no words or lettering. Naturalistic pre-rendered dark fantasy textures, rich fine
surface detail, restrained muted colors, original artwork suitable for 1990s
style isometric action RPG.

## Prop prompt

Use case: stylized-concept. Asset type: original production prop sprites for a
dark Christian medieval abbey dungeon, classic Diablo II pre-rendered visual
quality. A precise 2 columns by 3 rows regular grid on a genuinely transparent
background, 1536x1536 square canvas. Six distinct completely isolated objects,
one centered per square cell with generous clear padding, no overlap and no
labels. Render every prop with the SAME orthographic DIMETRIC game camera, 45
degrees azimuth and 30 degrees elevation, square world ground axes project at
+/-26.565 degrees to horizontal (2:1 diamonds). Sharply defined fine contours,
restrained photographic materials and subtle warm upper-left key lighting,
dark crevices and tiny highlights, no bloom, no blur, no cartoon outlines, no
ground plane, no external cast shadow. TOP LEFT: a tall narrow Gothic limestone
clustered pier, carved octagonal plinth, five slender attached colonnettes,
fluted clustered shafts, intricate acanthus capital, chipped mouldings and
horizontal coursing, smoky grey aged stone. It is slender, approximately 3
metres tall, a structural church column NOT a square block. TOP RIGHT: a long
ornate dark oak church kneeler/pew with carved end panels, slender legs, worn
seat, fine grain, small carved Latin cross on each end, long axis runs from
image bottom-left toward image top-right; rear wood panel no taller than a
person's waist. MIDDLE LEFT: a raised closed medieval limestone sarcophagus
with a highly legible Latin cross carved in the long lid, bevelled tiered lid
and base, recessed side relief panels, aged ash-grey stone, long axis
bottom-left toward top-right. MIDDLE RIGHT: a devotional statue of a robed
Christian saint carrying a small cross and closed book, standing on a small
octagonal plinth, worn ivory limestone, dignified face, distinct finely carved
folds. BOTTOM LEFT: a cluster of seven uneven half-melted ivory votive candles
in an ornate low tarnished brass dish, trails of wax, tiny warm flames. BOTTOM
RIGHT: small set dressing cluster of three old leather-bound books, one open
illuminated manuscript without readable lettering, a brass censer and a rosary
with a tiny Latin cross. Original assets, natural adult proportions and actual
carved detail, muted realistic palette, clear readable silhouette, authentic
late-1990s pre-rendered dark action RPG game art, not low-poly or stylized toy
shapes.

The small gold/wood/wax materials, flame flipbook, contact-shadow mask and
cross windows are deterministic assets authored by `cmd/author`. The supplied
DejaVu Serif display font retains its license in [`samples/vigil/assets/fonts/LICENSE`](../../../samples/vigil/assets/fonts/LICENSE).

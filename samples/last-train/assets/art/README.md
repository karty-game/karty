# Last Train artwork

The backgrounds and original portraits are the approved anime interpretation of
a 1930s railway mystery. Expression and alternate-pose variants were generated
on 2026-10-17 with the built-in imagegen tool, using each character's original
portrait as its edit reference. No external artwork or image-generation API
credentials are required to build or run the sample.

| Expression asset    | Edit reference                      | Intended reaction                                  |
| ------------------- | ----------------------------------- | -------------------------------------------------- |
| `mara-worried.png`  | `mara-conductor.png`                | Concerned eyes, raised inner brows and parted lips |
| `mara-relieved.png` | `mara-conductor.png`                | Relaxed brows and a gentle closed-eye smile        |
| `elias-wistful.png` | `elias-signalman.png`               | Lowered gaze and quiet sadness                     |
| `elias-hopeful.png` | `elias-signalman.png`               | Lifted gaze and a warm smile                       |
| `iris-guarded.png`  | `iris-traveller-blonde-glasses.png` | Watchful narrowed eyes and tense lips              |
| `iris-relieved.png` | `iris-traveller-blonde-glasses.png` | Softened brows and a closed-eye smile              |

Each portrait is 1024 × 1536 pixels. The variants preserve the original framing,
lower crop, clothing and props so the sample can change expressions without
changing the layout. Like the original portraits, all six expression variants contain
transparent alpha outside the character and can be composited over the separate
train backgrounds. Some image viewers display the hidden dark/gold RGB pixels
instead of compositing the alpha; the PNG alpha channel is authoritative.

## Generation prompts

The shared prompt was an identity-preserving production visual novel portrait
edit: match the reference's exact 1024 × 1536 canvas, framing, pose, silhouette,
head size, lower crop, original anime linework and warm lighting; change only the
facial expression; preserve transparent alpha outside the character; add no text,
objects, people, scene, panel or expression sheet.

Character constraints and expression requests:

- **Mara, worried:** preserve dark pinned wavy hair, navy conductor uniform,
  burgundy scarf, gold trim and watch chain, raised ticket punch and pocketed
  hand. Raise her inner brows, give her concerned eyes and part her lips as she
  admits uncertainty.
- **Mara, relieved:** preserve those same features and positions; relax her
  brows and give her a gentle closed-eye smile.
- **Elias, wistful:** preserve swept grey hair, moustache, round brass spectacles,
  mature face, green overcoat, brown cable-knit waistcoat, tie, watch chain,
  folded railway map and pocketed hand. Lower his gaze slightly, soften his
  eyebrows with sadness and keep his mouth closed as he remembers a lost friend.
- **Elias, hopeful:** preserve those same features and positions; lift his gaze
  toward the viewer, relax his eyebrows and add a small warm smile beneath his
  moustache, with subtly raised cheeks.
- **Iris, guarded:** preserve her blonde bob, brown eyes, large round brass
  glasses, youthful face, red belted coat, white blouse/scarf, brown gloves,
  shoulder bag and yellow sealed envelope at her chest. Add a small frown,
  slightly narrowed watchful eyes and closed tense lips.
- **Iris, relieved:** preserve those same features and positions; give her a
  gentle closed-eye smile, softened brows and raised cheeks as tension releases.

The selected files were generated with the built-in tool's transparent
background option. A separate background-extraction retry for worried Mara was
discarded; its original expression edit already contained transparent alpha.
Early opaque versions of hopeful Elias and relieved Iris were also discarded
and regenerated with transparent alpha. All six selected files were copied into
this directory without resizing, cropping or manual pixel editing.

Every selected output was visually reviewed against its reference for face,
hair, costume, prop, framing and expression consistency. Go's standard PNG
decoder verified 1024 × 1536 dimensions, zero-alpha exterior pixels and nonzero
character pixels in every selected variant; an RGBA file alone does not imply
transparency.

## Alternate poses

| Pose asset          | Edit reference                      | Body language                                                               |
| ------------------- | ----------------------------------- | --------------------------------------------------------------------------- |
| `mara-resolute.png` | `mara-conductor.png`                | Ticket punch lowered to waist; other hand touches her burgundy collar       |
| `elias-reading.png` | `elias-signalman.png`               | Reads an unfolded cream letter held in both hands; map tucked under his arm |
| `iris-offering.png` | `iris-traveller-blonde-glasses.png` | Offers the yellow sealed envelope; other hand rests at her coat             |

Each pose was generated in a separate built-in imagegen call with
`transparent_background: true`, referencing the original portrait. The common
prompt preserved exact character identity, clothing, period details, props,
anime linework and warm rim lighting, requested a 1024 × 1536 transparent canvas,
and kept head scale, top-of-head height, torso scale and lower crop consistent
with the reference. Both arm positions were explicitly changed. No scenery,
text, collage, additional people or extra limbs were requested.

The individual pose requests were:

- **Mara, resolute:** lower the brass ticket punch toward her waist, lift the
  other hand from her pocket to touch her burgundy collar, relax her shoulders
  and give her a calm, gently determined expression facing the viewer.
- **Elias, reading:** hold an unfolded blank cream letter at chest height with
  both hands, tuck the folded railway map under his arm, and look down through
  his spectacles with softened brows and a tender smile.
- **Iris, offering:** extend one gloved hand gently to the side and toward the
  viewer with the yellow envelope and red wax seal, rest her other hand at the
  front of her coat, and add an apprehensive small smile and slight forward lean.

All three selected outputs were visually reviewed and copied without resizing,
cropping or manual pixel editing. Go's standard PNG decoder verified
1024 × 1536 dimensions and actual alpha transparency in every pose: 746,555
fully transparent pixels for Mara, 544,715 for Elias and 616,907 for Iris.

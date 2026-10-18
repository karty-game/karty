# Short coastal seagrass

Original asset generated with the built-in `image_gen` tool on 2026-10-16.
Master: `seaweed-short-master-v1.png`.
Prepared runtime inputs: `samples/terrain/levels/ground/seaweed-short-v1.png` and
`samples/terrain/levels/ground/seaweed-short-v1-normal.png`. The retained `scripts/terrain/author-groundcover.go`
trims transparent margins, removes extraction fringes below alpha 64 and
estimates tangent-space normals from diffuse luminance. CLI sprite preparation
can limit these to 256 pixels; the host supplies coverage-preserving mipmaps.

Master 1254×1254; alpha-128 bounds `(36,50)–(1218,1224)`, prepared 1182×1174. Prepared coverage is 25.51%; bottom-quarter coverage is 38.81%, retaining open spaces between the three root groups.

Exact generation prompt:

```text
Use case: photorealistic-natural
Asset type: original front-view diffuse RGBA cutout of SMALL COASTAL SEAGRASS for a real-time tropical island game, used on two intersecting polygon cards.
Primary request: one small natural seaweed patch made of three loose overlapping bunches of olive-green eelgrass, with roughly twenty-five to thirty smooth slender flat ribbon leaves, softly bent by a gentle underwater current. A few leaves lean left and others right, the tallest off-center. Short low side leaves give a rounded irregular mound shape. Each ribbon tapers to a softly pointed tip and has a subtle central rib. A few muted warm tan tips and faint mottled brown-green marks are natural.
Composition/framing: almost square frontal orthographic view, whole plant inside frame with transparent margins. Root groups spread across the middle half of the width at a common ground level, each basal cluster very narrow and softly ragged, with transparent gaps extending down to the ground BETWEEN clusters. No continuous solid bottom strip or thick opaque base. Outermost sides taper into separate scattered blades rather than upright boundaries. About sixty percent of the rectangle remains genuinely transparent with clearly readable gaps between blades; overlap enough for a lush small tuft without becoming a wall. Leaf widths should survive a 256-pixel runtime texture.
Color/lighting: medium muted olive, sage and soft amber green, restrained natural variation; no neon lime. Neutral evenly lit diffuse albedo only, no aquatic blue fog baked in, no black shadows, no sun glints.
Scene/backdrop: actual transparent alpha background, no seabed, sand, mud, roots drawn as a soil disc, bubbles, underwater scenery, glass, aquarium or cast shadows.
Style: natural realistic game texture with crisp clean silhouette, visible broad flat ribbon surfaces, no microscopic fuzz, no flowers or terrestrial grass seedheads.
Constraints: plant only, no text, logo, watermark or border. Do not clip any leaf. No rectangular opaque patch, no straight horizontal cut line at base, no fern or branching bush.
```

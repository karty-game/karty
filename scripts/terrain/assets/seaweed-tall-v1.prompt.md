# Tall ribbon kelp

Original asset generated with the built-in `image_gen` tool on 2026-10-16.
Master: `seaweed-tall-master-v1.png`.
Prepared runtime inputs: `samples/terrain/levels/ground/seaweed-tall-v1.png` and
`samples/terrain/levels/ground/seaweed-tall-v1-normal.png`. The retained `scripts/terrain/author-groundcover.go`
trims transparent margins, removes extraction fringes below alpha 64 and
estimates tangent-space normals from diffuse luminance. CLI sprite preparation
can limit these to 256 pixels; the host supplies coverage-preserving mipmaps.

Master 887×1774; alpha-128 bounds `(35,16)–(854,1752)`, prepared 819×1736. Prepared coverage is 25.15%; bottom-quarter coverage is 23.94%, retaining open spaces between holdfasts.

Exact generation prompt:

```text
Use case: photorealistic-natural
Asset type: original front-view diffuse RGBA cutout of TALL RIBBON KELP for a real-time tropical island game, used on two intersecting polygon cards.
Primary request: one open natural underwater kelp plant with about ten to fourteen LONG flat gently twisting ribbon fronds, several sprouting from each of three small loosely grouped holdfasts. Fronds rise in asymmetric flowing S-shaped bends at varied heights, with clear negative space between them. Some broad wavy ribbons twist slightly to show changing widths; a couple droop sideways. Make it a tall graceful marine plant, not straight lawn grass, not a fern or palm leaf. Smooth tapered pointed tips, delicate wavy edges and subtle lengthwise ribs. Include only a few short side shoots. No spherical air bladders.
Composition/framing: portrait composition approximately twice as tall as wide, whole plant fully contained, frontal orthographic view. Tallest ribbons at slightly different off-center positions. Three narrow basal holdfasts spread across central forty percent of width at a common ground plane; naturally uneven ends and transparent ground-level openings BETWEEN them. No opaque floor strip, no painted mound and no flat cut edge. Outer fronds form an irregular airy outline. Around sixty-five percent of the rectangle remains transparent, with substantial irregular holes throughout the middle.
Color/lighting: natural subdued olive green and warm amber-brown, a few sage-green young fronds; medium values so lighting and underwater fog can be applied later. Neutral soft diffuse lighting, no black baked shadows, no strong glossy reflections, no blue tint baked in, no fluorescent lime.
Scene/backdrop: actual transparent alpha background, plant only, no ocean, sand, rocks, underwater scenery, bubbles, caustics, aquarium or cast shadows.
Style: realistic game diffuse texture with clearly readable ribbon surfaces, restrained fine variation, clean antialiased edges at 256-pixel runtime size, original artwork.
Constraints: no letters, text, logo, watermark, frame, flowers or seed heads. Do not clip leaf tips. No rectangular hedge, opaque bottom bar, root-soil disc, thick trunk, microscopic fuzz, continuous filled foliage panel or geometric straight leaf pattern.
```

# Cloud panorama prompts

Built-in imagegen, 2026-10-12. Both outputs requested a transparent background.
The initial outputs below were superseded by the high-resolution edits at the
end of this file. Current PNGs are native 2172×724; no source pixels were
subsequently edited or resized. Runtime sampling places cloud banks above the
horizon and uses their luminance with the selected environment palette.

## Dense source generation

Use case: stylized-concept. Asset type: game skydome equirectangular cloud texture,
RGBA with genuine transparent background. Create a high-quality 2:1 wide
panoramic texture, ideally 2048x1024, for the inside of a classic game skydome.
Image represents longitude 360 degrees horizontally and elevation +90 degrees
at TOP to -90 degrees at BOTTOM, horizon exactly at vertical midpoint. ONLY
clouds, no blue background, no sky gradient, no sun, no stars, no ground, no text.
Natural soft cumulus cloud banks with intricate wispy feathered boundaries,
bright softly lit white tops, subtle blue-grey shaded bases, irregular overlapping
shapes and broad empty transparent gaps. Art direction: convincing semi-realistic
hand-painted game sky as in polished classic Doom/Duke-inspired outdoor
environments, calm and understated, no cartoon lumps or rounded blob icons.
Place clouds in the upper half, mostly between elevations 10 and 55 degrees,
sparse small clouds near the horizon, leave clear space near the zenith; entire
lower half transparent. Make cloud sizes vary and arrange several broad banks
rather than isolated identical puffs. Full horizontal wrap: left and right 10%
must be transparent to guarantee no seam, top and bottom transparent. Preserve
smooth fractional alpha at edges and opaque cloud interiors. Keep neutral
daylight shading so runtime tinting can produce morning, evening and fantasy
palettes.

## Sparse source edit

Edit the cloud skydome texture just generated. Keep its convincing neutral
semi-realistic painted cloud detail, shaded bases, truly transparent background
and 2:1 equirectangular layout. Change ONLY distribution and unwanted color
contamination: replace the continuous horizon-spanning band with THREE separate
irregular cloud banks, separated by very broad clear transparent gaps. All cloud
RGB must be neutral white to grey; remove all cyan/blue rims or background pixels.
The leftmost and rightmost 15% of the entire image MUST be entirely transparent,
to give a perfect repeat seam. The entire bottom HALF MUST be transparent. TOP
10% must be transparent. All cloud banks should be in the upper middle between
15% and 45% of the image height; their bases do not form a straight continuous
line. One broad bank, one smaller distant bank, one wispy uneven medium bank.
Generous clear gaps totaling at least half of the upper hemisphere. No cartoon
icon blobs, no blue background, no sun, no scenery, no text. Preserve soft
fractional alpha at edges.

## Selected high-resolution sparse edit

Edit target: the sparse cloud panorama. Remake as a genuinely native high
resolution 3:1 panorama, ideally 3072×1024 or 3840×1280, at least 2048 pixels
wide. Full-sphere normalized UV: top +90°, horizon 50%, bottom −90°;
intentionally non-square angular texels. Sparse, soft rounded cumulus banks
with roughly 30% upper-hemisphere coverage and large clear gaps. Place banks
at horizontal 16–33%, 52–65% and 76–86%; leave the middle 34–50% clear for
the default courtyard view. Top 15%, bottom half, and outermost left/right
10% transparent. Detailed neutral white and grey semi-realistic painted
clouds, no blob icons, cyan fringes, blue sky, sun, ground or text. Preserve
real fractional alpha.

## Selected high-resolution dense edit

Use case: stylized-concept. Edit target: cloud panorama. Remake as a native
high resolution extremely wide 3:1 panorama (3072x1024 preferred, at least
2048px wide), mapped to full sphere normalized UV top zenith, horizon exact
middle, bottom nadir. Retain realistic intricate cloud detail but refine
distribution into broad broken banks with overlapping soft cumulus and some
thin wispy sheets, varied heights and broad clear gaps totaling 25 percent
of upper hemisphere. All cloud artwork only between 15% and 49% image
height; entire bottom half and top 12 percent completely transparent.
Leftmost/rightmost 8 percent entirely transparent for horizontal tiling seam.
Genuine RGBA transparent background. Cloud RGB neutral white/gray only, no
cyan/blue color rims, avoid coarse grain and icon-like puffs. Soft atmospheric
naturally feathered boundaries, subtly shaded bases, lighter rounded tops.
No ground, sky color, sun, text, watermarks. Change resolution and
distribution only; preserve polished classic game painted sky aesthetic.

The selected outputs spread lower on the canvas than requested. The shader
uses `clamp(v * 1.6, 0, 1)` to put the banks above the horizon, and samples
linear luminance to avoid colored edge contamination during palette tinting.

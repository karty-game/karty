# Faint cirrus v1

Original artwork generated on 2026-10-15 with the built-in `image_gen` tool.
Generation and both targeted edits used transparent-background mode. Reference
screenshots informed the distant high-cloud direction; no game artwork was copied.

Final asset: `clouds-cirrus-v1.png`, 1774 × 887, RGBA, a 2:1 equirectangular layer.
SHA-256: `1c3949221c3cc6200a64d1396c7d5f2f1671e88ff9a635b8cef0563f13009831`.
The generated alpha is preserved without resizing or other postprocessing.

PNG inspection found 1,505,642 fully transparent pixels and 67,896 pixels with
partial alpha. The maximum alpha is 244/255; mean alpha among nonzero pixels is
24.2%. The authored 20% cloud amount further attenuates the cloud layer. The
generated image did not obey the prompt's numerical alpha ceiling exactly.

## Exact generation prompt

Use case: photorealistic-natural. Asset type: original transparent RGBA cirrus cloud layer for an equirectangular tropical island game skydome. Canvas must be a wide 2:1 panorama mapping 360 degrees longitude and 180 degrees latitude. The game supplies its own blue sky separately: background must be genuinely transparent, with no blue, grey, black, checkerboard, or sky-colored fill. Show a few extremely faint distant high-altitude cirrus veils: delicate elongated white ice-crystal filaments, soft featherlike streaks, gently curved wisps and finely dispersed translucent trails, irregular natural spacing with almost all the sky remaining clear. These are thin atmospheric traces seen far away, not solid cloud objects. Most visible filaments should be low opacity, about 15–35 percent, fading seamlessly into fully transparent edges, no hard cutout silhouettes. Quiet tropical noon mood, subtle white only, no grey shadow or bulky shading. Layout for spherical skydome: place the faint traces only between 10 and 45 percent canvas height (upper hemisphere), keep the lower half fully transparent, keep the topmost 8 percent and left/right outer 4 percent fully transparent. A few slender gently sloping feathery patches across the panorama; broad untouched transparent gaps between them. No central hero cloud, no fluffy blobs, no cotton puffs, no cumulus, no round billows, no thunderheads, no continuous cloud wall or horizon bank, no opaque sheet, no fog, no sun, no horizon line, no landscape, sea, objects, text, borders or watermark. Original understated cirrus artwork with the natural barely-visible high cloud character of a clear tropical lagoon sky, not any copied game artwork.

## Exact first edit prompt

Change this cloud layer into very sparse, faint high-altitude CIRRUS. It currently has a huge thick continuous cloud bank and is far too solid. Remove roughly 85 percent of the existing cloud material. Break all large masses into a few detached THIN hairlike filaments and tiny feather strokes, only one tenth of the current cloud height. There should be mostly empty fully transparent canvas. Do not retain any solid cloud mass or broad paintbrush ribbon. Think subtle threads of white silk seen high and far away in a clear tropical sky, not smoke and not cotton. Make even the brightest filament very translucent, maximum alpha about 50 out of 255, with most alpha about 15–30 out of 255. No opaque or nearly opaque regions. Keep white RGB rather than grey shading; use actual alpha for faintness. Keep the 2:1 panoramic canvas, all faint threads only in the upper half, lower half completely transparent. Disconnected sparse wisps with large empty gaps, no skyline, no sky background, no horizon line, no land or water, no text. Preserve genuine RGBA transparency.

## Exact final edit prompt

Remove the entire large connected feathery cloud formation from the LEFT HALF of this transparent panorama, including its tall peak and its long trailing arm. Replace that whole left-half area with genuine full transparency. Preserve only the small detached thin cirrus wisps in the upper right half, preserving their current very low alpha and delicate threadlike feather details. Do not add any new cloud, fill, sky color, opacity or texture. Preserve the 2:1 image dimensions and RGBA transparent background. The final should be almost entirely empty with just a few faint distant cirrus threads, no hero shape, no thick cloudy mass.

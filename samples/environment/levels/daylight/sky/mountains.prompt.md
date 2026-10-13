# Mountains imagegen provenance

Tool: built-in `image_gen.imagegen`, transparent background requested, 2026-10-12.
Final source: `mountains.png`, native 2171 × 724 pixels, greater than 2048 pixels wide.
No upscaling, resampling, pixel processing or alpha replacement was applied.
The earlier 1774 × 887 source is retained in `/tmp/karty-skydome-polish-baseline`.

## High-resolution generation prompt

Use case: stylized-concept.
Asset type: HIGH RESOLUTION production first-person game skydome horizon texture. Native wide 3:1 panoramic canvas, 3072x1024 or larger, at least 2048 pixels wide. This is new high resolution authored artwork, not an upscale.
Primary request: a believable distant mountain range for a classic Doom/Duke inspired exterior courtyard, suitable also for fantasy. Continuous 360-degree horizon artwork, unwrapped horizontally.
Composition: horizon is exactly halfway down the panorama. Several coherent, irregular overlapping ranges, atmospheric depth: a softer more distant low ridge, a textured mid-distance ridge and a subdued darker rocky foothill base. Mountain peaks only 10–20 degrees above the horizon, not towering over the camera. On a 1024px-high canvas, keep the highest peaks between y=400 and y=450, most ridge tops between y=440 and y=510. Above these cutout silhouettes is fully transparent empty sky. Every pixel below the horizon y=512 is a fully opaque dark neutral gray rocky base, extending to the bottom without holes or margins.
Artwork: semi-realistic matte painted game environment art, naturally eroded rock ridges with readable but restrained fine stone texture, varied diagonal slopes, ledges and asymmetric peaks. Soft diffuse neutral daylight, controlled contrast, cool neutral slate grays, distant rock faces slightly lighter and less detailed, foreground foothills moderately darker. Natural credible terrain, no stylized polygon mountains or inflated cartoon forms.
Seam: horizontally seamless panorama. Outer 8 percent at both ends should be the same quiet low foothill band at the horizon; matching left and right silhouette height and gray base color. No distinct peak or unique landmark near the seam. Keep both outermost columns matching, transparent above the same low ridge and dark neutral slate below. Avoid a pronounced central hero peak or symmetry; spread interest unevenly across the panorama.
Transparency: genuine alpha-zero background above ridges, soft antialiased silhouette edges, alpha=255 solid stone and lower base. No sky fill, checkerboard, clouds, sun or moon.
Avoid: trees, snow-white caps, buildings, water, neon, text, watermark, large near-camera boulders, black outlines, photographic foreground perspective, vanishing-point distortion, vertical seam stripe.

## Targeted mountain edge edit prompt

Use case: precise-object-edit.
Edit target: supplied HIGH RESOLUTION mountain panorama, native 2172x724, 3:1. Preserve the high resolution canvas, all central mountain artwork, realistic muted slate texture and transparent sky. Do not downscale or upscale it.
Correct only the horizontal seam: both outermost 8 percent of the image must be the same quiet low foothill strip, with one completely flat silhouette at precisely the image's midline (y=362 on this canvas), not at different heights. Left and right edge columns must match: alpha zero above row 362, fully opaque cool dark slate gray #3B3E43 below row 362. No stray semitransparent pixels, colored specks or small peaks above this line anywhere in those outer edge bands. Keep this seam band simple and neutral, then blend its stone color and height into the existing interior mountains over another small transition region. Preserve all mountain ridges away from the edges.
Keep the continuous stone base below the horizon fully opaque to the bottom. Upper empty sky remains genuine transparent alpha, not painted black or checkerboard. No other objects, sky colors, clouds, labels or borders.

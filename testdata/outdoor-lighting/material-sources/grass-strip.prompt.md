# Grass strip source

Generated with the built-in imagegen tool on 2026-10-14. Selected edited output:
`exec-0c92ba1a-49f3-4652-a32a-e606e0f2e21a.png`.
The previous clump artwork remains tracked for comparison.

Generation prompt:

> Create a production game foliage ALBEDO cutout texture: a loose strip of fine temperate/subtropical forest-edge grass, on a genuinely transparent background. Square composition, straight orthographic frontal view, flat neutral diffuse daylight without cast shadows. 55 to 80 slender individual naturally curved grass blades with varied heights and angles, some thin dry ochre stems, mostly muted warm olive and fresh green blades. The roots of the blades are distributed evenly across the ENTIRE bottom width as a continuous meadow strip, NOT converging to a central tuft, NOT a fountain or a potted plant. Short blades mixed with taller slender blades, generous transparent gaps between the blade tips, tallest blade tips in upper third. Blades arch and lean gently sideways irregularly. Ground line at the bottom image edge, all roots on that edge. No soil, no root ball, no flower, no container, no backdrop, no text, no border, no shadow. This is a realistic 2000s AAA jungle ground-cover grass card texture; subtle broad colour variation, readable soft colour masses, not noisy microdetail. Keep all tips and left/rightmost blades in frame, tips no closer than 4% to edges. The lower 12% has dense overlapping short green blades all the way across the width; the upper half has delicate sparse silhouettes. No large broad leaves and no symmetrical fan.

Selected edit prompt:

> Clean up this grass texture for game use. Keep the same natural grass strip rooted across the whole bottom edge, same blade shapes and varied heights. Remove every red, fluorescent green, yellow chroma-key artefact and all pale fuzzy halos between the blades: these areas should become completely transparent. Only actual naturally olive-green grass blades and muted straw stems should be visible. Make blades in the lower half easier to read as broad clean colour shapes, slightly softer surface detail, diffuse neutral albedo. Actual transparency all around and between blades. No soil, no background, no colours other than natural olive grass and muted straw. Preserve crisp clean grass blade silhouettes.

The retained generator removes extraction fringes below alpha 64 before
resizing. Bright red pixels in the master have alpha 1–4 and do not survive
this step. Roots span the lower edge instead of converging into one tuft.

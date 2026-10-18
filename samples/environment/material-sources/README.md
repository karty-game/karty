# Environment material sources

The tracked generator in [`scripts/environment-assets.go`](../../../scripts/environment-assets.go)
prepares the sample's retained textures. Ordinary game builds consume those
textures and do not run image generation or import 3D vegetation models.

- [`shrub-leaf-spray.png`](shrub-leaf-spray.png): original transparent leaf artwork
  made with the built-in imagegen tool; [exact prompt](shrub-leaf-spray.prompt.md).
- [`grass-strip.png`](grass-strip.png): current continuous meadow-strip artwork,
  with roots distributed across the card; [prompts](grass-strip.prompt.md).
- [`grass-clump.png`](grass-clump.png): original transparent meadow grass artwork
  made with the built-in imagegen tool; [generation and selected edit prompts](grass-clump.prompt.md).
- [`forest-floor.png`](forest-floor.png): current moss/short-grass/leaf-litter
  terrain artwork, reduced with area filtering; [prompt](forest-floor.prompt.md).
- [`grass-soil-2048.png`](grass-soil-2048.png): the existing procedural ground
  material source retained for comparison.

Asset preparation preserves cutout alpha and resizes the leaf spray to a
512×512 upper region in the 512×1024 shrub atlas. Its lower region is opaque
bark for the generated branches. Grass becomes a 512×512 cutout with its root
edge aligned to the bottom. Low-alpha extraction fringes are removed before
resampling to avoid color bleed into visible edges.

The generator also creates matching tangent-space normal maps using estimated
edge curvature and subtle texture luminance detail. These are approximate
relief maps, not measured plant geometry. Background directions and bark stay
neutral; normal alpha is opaque. Authors can supply their own matching normal
map through `normal_texture` without handling a 3D model.

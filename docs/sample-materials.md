# Sample material classifications

The crate-room, environment and world-camera samples select Materialize recipes
by the material depicted in the source artwork. `profile = "environment"` controls
image processing; `[textures.materialize]` independently controls generated
normal, height, AO, metallic and smoothness maps. A floor can therefore use
`wood`, `concrete` or `marble`; its position does not determine its preset.

This inventory uses the SDK-pinned Materialize CLI 2.0.0 presets, checked against
the source pinned by [karty-tools](https://github.com/karty-game/karty-tools/blob/main/materialize/sources.json)
(upstream commit `1a3fe7d052a464d3217ef66d2bce9889d3946792`). `foliage` covers
leaves and grass; `floor` supplies strong ground relief/AO; `stone` is rough rock;
`marble` is polished stone; `plaster` supplies soft, shallow relief.

Every nonmetal surface below explicitly sets `metallic_scale = 0`. Several named
presets inherit a nonzero metallic detector from the general defaults, so choosing
a named preset alone does not guarantee a nonmetallic result. The CLI also clears
the tool's residual specular-derived metallic channel for explicit zero. This
retains dielectric highlights and the preset's smoothness. Metal scales multiply
the tool's detection result; they are not a literal percentage of exposed metal.
Painted steel uses `metal` for its surface recipe with metallic generation forced
to zero for the paint. Emission is authored separately from material generation.

## Crate-room

[Manifest](../samples/crate-room/levels/room/level.toml): seven world surface
textures, all with explicit recipes.

| Assets           | Depicted material / use                        | Preset     | Metallic scale |
| ---------------- | ---------------------------------------------- | ---------- | -------------- |
| `floor`          | Jointed concrete floor panels                  | `concrete` | 0              |
| `ceiling`        | Concrete ceiling slab                          | `concrete` | 0              |
| `wall`           | Horizontal concrete wall panels                | `concrete` | 0              |
| `wood`, `frame`  | Crate planks and wooden frame battens          | `wood`     | 0              |
| `ramp`, `bumper` | Hazard-painted steel banks and painted bumpers | `metal`    | 0              |

The physics demonstration retains its authored runtime `normal`, `height`, `ao`
and `rim` strengths of zero. These settings do not select the generation preset
or disable reflectance maps. Changing them is a separate visual tuning choice.
The handwritten manifest owns the recipes; the tracked
[generator](../samples/crate-room/generate-assets.go) owns only the seven albedo
PNGs. There are no sprite textures in this level.

## Environment

[Manifest](../samples/environment/levels/daylight/level.toml): eleven world
surface textures, all with explicit recipes.

| Assets            | Depicted material / use                                       | Preset                               | Metallic scale |
| ----------------- | ------------------------------------------------------------- | ------------------------------------ | -------------- |
| `stone`           | Rough stone paving, recesses, foundation and boundary walls   | `stone`                              | 0              |
| `tile`            | Concrete tiles in the observation room and passage            | `concrete`                           | 0              |
| `wall`            | Brick courses                                                 | `brick`                              | 0              |
| `ceiling`         | Rough concrete ceiling                                        | `concrete`                           | 0              |
| `ground`          | Muted grass mixed with soil on the reserved field             | `foliage`                            | 0              |
| `wood`            | Wooden shadow-test crate, table tops, chair seats and benches | `wood`                               | 0              |
| `wood-edge`       | Lighter wooden end grain on narrow furniture edge bevels      | `wood`                               | 0              |
| `metal`           | Weathered monolith, canopy and window frames                  | `metal`                              | 0.35           |
| `furniture-steel` | Bare brushed furniture legs and bench supports                | `metal`                              | 0.8            |
| `slate`           | Rough overlapping slate tiles on the pitched building roof    | `stone`                              | 0              |
| `glass`           | Slightly tinted recessed building window panes                | `water`, neutral relief/AO overrides | 0              |

The grass/soil master under `material-sources/` is an authoring input. Its compact
`ground.png` derivative ships in the level. The tracked
[generator](../scripts/environment-assets.go) owns both images, the manifest
recipes and the world layout; regenerate with `mise run generate-environment`.
The `table`, `chair` and `bench` source prefabs are reusable static
geometry. The low perimeter walls and raised facade footing reuse the opaque,
nonmetallic `stone` material. The upper storey and pitched roof are decorative closed solids. Glass
uses the water recipe's smooth dielectric reflectance, with generation and runtime
normal/height/AO strengths set to zero. Its static liquid binding has opacity
0.16 and zero flow, speed or deformation; this reuses the existing transparency
format while keeping the panes still. All facade panes share one plane, allowing
the bounded reflection capture to serve them together. The roof remains a rough
stone material rather than a shiny metal surface.
Sky, clouds, mountain panorama and the separate sun layer belong to the host
renderer and do not pass through these level Materialize recipes.

Furniture separates restrained wood relief from quieter bare-metal supports.
Table/bench tops and chair seats use narrow 45-degree edge bands partitioned
inside their existing footprints. Chair geometry remains one shared runtime
prefab; its physical box is unchanged. Broad indoor fills have shorter radii
and lower intensity, while lighter ceiling/oak surfaces and sixteen samples
improve the prepared one-bounce lighting. Density, page and residency limits
remain unchanged.

## Hangar hero surfaces

The suspended ceiling fixtures use `ceiling-strip`, a steady emissive opal
diffuser with no normal/height/AO relief, and the existing metal trims. The
material generator owns `strip-light.png`; `scripts/hangar-ceiling-lights.go`
owns the housings, end caps, ceiling mounts and hangers. Each existing point
source sits just below its strip, so opaque fixture geometry cannot hide the
source inside its own shadow caster. The light colors/radii/intensities are
retained for working fixtures, and emission supplies the visible diffuser rather than extra room
illumination. The fixtures do not add dynamic lights or player colliders.

The broken arrival strip uses nonemissive `ceiling-strip-broken` and generated
`strip-light-broken.png`: soot-darkened cracked glass, with a missing central
section in its geometry. Its `ventilation-fill` source is zeroed so the baked
room lighting matches its unpowered appearance. The window bay's working strip
uses its existing `window-key` source and ordinary `ceiling-strip` diffuser.

The reactor's original ribbed artwork and the barrel's quiet ribbed paint use
explicit zero metallic generation. Their highlights remain dielectric. Bare
brushed `hero-steel` uses metallic scale 0.85 with low relief, separately from
the general steel room trim. Smoothness remains generated by the pinned tool;
no packed material map is edited by hand.

`scripts/hangar-material-assets.go` owns the brushed albedo. The handwritten
`scripts/hangar-hero-assets.go` owns the nonblocking barrel hoop's sloped edge
bands and the collidable reactor cap's vertical corner chamfers. The cap keeps
flat horizontal surfaces required by the sample's step-clearance data. Generate
and check these with `mise run generate-hangar-heroes` and
`mise run check-hangar-heroes`. Builds regenerate walking data after hero trim,
and lighting bakes depend on the same source generation.

## World-camera

[Manifest](../samples/world-camera/levels/showcase/level.toml): 25 texture entries.
Sixteen world surface textures have explicit recipes, one secondary layer uses
color only, and eight sprite/flipbook textures do not use Materialize.

| Assets                              | Depicted material / use                                  | Preset                       | Metallic scale |
| ----------------------------------- | -------------------------------------------------------- | ---------------------------- | -------------- |
| `wall`, `wall-top`, `wall-bottom`   | Gallery bricks and masked brick trim                     | `brick`                      | 0              |
| `floor`                             | Jointed concrete gallery tiles                           | `concrete`                   | 0              |
| `ceiling`                           | Painted plaster gallery panels                           | `plaster`                    | 0              |
| `z-court-marble`                    | Polished veined marble columns and trim                  | `marble`                     | 0              |
| `z-court-mosaic`                    | Polished stone mosaic and basin lining                   | `marble`                     | 0              |
| `z-court-paving`                    | Rough courtyard paving slabs                             | `stone`                      | 0              |
| `z-court-plaster`                   | Bevelled limestone blocks; the name is historical        | `stone`                      | 0              |
| `z-court-coffer`                    | Carved stone ceiling coffers                             | `stone`                      | 0              |
| `z-court-water`, `z-water-boundary` | Visible basin water and invisible slab side/bottom faces | `water`                      | 0              |
| `z-hanging-bronze`                  | Bronze brazier bowls / sconce collars                    | `metal`                      | 0.75           |
| `z-hanging-iron`                    | Iron hanging chains / sconce supports                    | `metal`                      | 0.65           |
| `z-hanging-coals`                   | Rough charcoal with a separate emission setting          | `stone`                      | 0              |
| `z-court-sky`                       | Flat sky illustration on a world ceiling                 | `default`, neutral overrides | 0              |

The sky illustration remains a primary world ceiling texture, so the current
atlas pipeline generates its data. Its normal strength, height contrast and AO
depth overrides are zero, as are its existing runtime relief/AO strengths.
Generated normals and AO are neutral; the tool's small residual height detail
is unused because runtime height strength is zero.
It represents painted sky color rather than a physical stone or plaster surface.

| Assets                                                     | Classification                                        | Materialize treatment                            |
| ---------------------------------------------------------- | ----------------------------------------------------- | ------------------------------------------------ |
| `z-detail-grain`                                           | Secondary mineral color variation on primary surfaces | Color-only; no generated material data or preset |
| `character`, `ghost`, `tree`, `sign`, `decal`              | World billboard/decal sprite artwork                  | Sprite processing; no preset                     |
| `z-hanging-fire-1`, `z-hanging-fire-2`, `z-hanging-fire-3` | Animated fire sprite frames                           | Flipbook sprite processing; no preset            |

Masked wall bands remain brick world surfaces. Their alpha silhouettes are
preserved while the existing coverage extension supplies opaque input to the
material generator. They are not sprite-only textures. The hidden water faces
use the same recipe and source pixels as the visible water, allowing material
cache reuse while retaining their independent animation/opacity settings.

The handwritten manifest owns these recipes. The tracked
[albedo generator](../cmd/world-camera-materials/main.go) and
[fixture generator](../cmd/world-camera-fixtures/main.go) own their existing
artwork/layout outputs. Neither generator rewrites the manifest. Builds perform
Materialize generation into ignored `.karty/` caches; do not edit generated
material maps by hand. Runtime `material_strengths`, liquid animation and baked
lighting remain separate controls.

The pinned detector infers metallic pixels from albedo. Its response is weak for
these dark bronze/iron tiles, even with the correct `metal` preset. This inventory
qualifies preset selection and zero-metal enforcement; it does not establish
physically measured reflectance for the procedural artwork.

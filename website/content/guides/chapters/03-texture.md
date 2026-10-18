## Give surfaces a material

Use a small tileable image first. The texture's `name` is the material name
used by floors, ceilings, and boundary edges. Texture paths are relative to
`level.toml`.

Add this at the top level of `world.yaml` to repeat the image every two metres:

```yaml
uv:
  mode: triplanar
  anchor: world
  scale: {x: 2, y: 2}
```

Scale is metres per repeat: a larger value makes the pattern larger. Set a
room's `wall_uv`, `floor_uv`, or `ceiling_uv` to change only those surfaces.
World anchoring keeps repeats aligned when you place prefabs.

Materialize generates height, normal, and ambient occlusion maps for world
surfaces. SDKs with material reflectance also generate metallic and smoothness
maps. Add a preset immediately after the texture entry in `level.toml`:

```toml
[textures.materialize]
preset = "concrete"
```

Run `karty build --target web` to generate and cache the maps. The CLI makes
world textures seamless automatically. These controls apply to world surfaces;
sprite textures keep their original image processing.

### Choose a preset

Use these exact lowercase names:

- `default` balances surface detail. This is the default when no preset is set.
- `auto` chooses a preset by analysing the source image.
- `skin` produces smooth, non-metallic skin.
- `floor` emphasises height and ambient occlusion for ground surfaces.
- `metal` strengthens metallic detail and sharp edges.
- `fabric` produces soft, matte cloth.
- `wood` preserves moderate grain detail.
- `stone` produces rough rock with deep ambient occlusion.
- `concrete` produces rough concrete.
- `leather` produces pebbled leather with some smoothness.
- `marble` produces polished, veined stone.
- `sand` produces fine, rough grain.
- `foliage` suits leaves and grass with little metallic detail.
- `plaster` keeps surfaces flat with soft normals.
- `asphalt` produces rough surfaces with dense edge detail.
- `brick` emphasises sharp edges and rough surfaces.
- `ice` produces smooth surfaces with subtle detail.
- `snow` produces soft, diffuse surfaces.
- `lava` produces partly metallic, molten surface detail.
- `water` produces smooth water surface detail.

Presets change generated maps. They do not add liquid motion or light emission.
Start with their defaults. Use `normal_strength`, `height_contrast`,
`metallic_scale`, or `ao_depth_scale` only when the result needs adjustment.
The separate `textures.material_strengths` settings control how strongly the
game displays those maps. Your generated `.karty/docs/assets.md` lists the
overrides and limits for your SDK.

Check the result at player height. A material that looks good close up may
repeat too obviously across a large wall. [The Hangar](../hangar/) shows wall
bands, secondary weathering, and small sprite decals.

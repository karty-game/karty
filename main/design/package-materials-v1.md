# Package material generation, version 1

Package format 1 texture exports can carry optional Materialize generation
settings. A package's metal barrier or concrete wall then keeps its intended
material preset when imported into another game. This is an additive CLI
authoring feature; it keeps the existing world and cartridge formats.

```toml
[[textures]]
name = "bumper-red-metal"
source = "textures/bumper-red-metal.png"
profile = "environment"

[textures.materialize]
preset = "metal"
normal_strength = 0.75
```

The optional table accepts the same generation controls as a level texture:

| Field             | Meaning                                                         |
| ----------------- | --------------------------------------------------------------- |
| `preset`          | Materialize preset; omission or an empty string uses `default`. |
| `normal_strength` | Normal generation strength.                                     |
| `height_contrast` | Height generation contrast.                                     |
| `metallic_scale`  | Metallic detection scale.                                       |
| `ao_depth_scale`  | Ambient occlusion depth scale.                                  |

Numeric overrides must be finite within `[0,16]`. Omission keeps the preset's
value; an explicit zero remains zero throughout archive restoration, texture
selection and level staging. Presets are `default`, `auto`, `skin`, `floor`,
`metal`, `fabric`, `wood`, `stone`, `concrete`, `leather`, `marble`, `sand`,
`foliage`, `plaster`, `asphalt`, `brick`, `ice`, `snow`, `lava` and `water`.
Unknown presets, unknown fields and incompatible types fail package validation.
The table is supported only for the `environment` texture profile, including
when the table is empty. Transparent sprite and decal exports cannot carry it.

Import and export validate every declared texture and generation setting,
including unused exports. Archive checksums cover the manifest and PNG sources;
generated maps remain ordinary consuming-project cache outputs. Existing file,
manifest and aggregate package bounds and path confinement continue to apply.

Only textures selected by the world enter the level's generated manifest.
Their fully qualified names, profile, PNG source and generation settings travel
together through `.karty/packages/` staging. Unused exports remain pruned.
The consuming level must use a selected material on a world surface that
generates material data; secondary-only surface restrictions still apply.

Package settings feed the existing per-material generation recipe and cache
identity. They do not set runtime material strengths, emission, physics or
collision behavior. Projects can use an instance material replacement to select
an authored local texture with different generation settings.

Existing packages without the table behave as before. Older CLIs strictly
reject the new table, so packages using it require a CLI implementing this
decision. Format version 1 remains current because the archive structure,
namespace and world reference semantics are unchanged; this does not claim that
an older strict reader can consume every newer optional field.

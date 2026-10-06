# Bake storage v1

Status: implemented CLI storage decision. The CLI owns filesystem storage; the
SDK owns bake algorithms and cartridge/level formats.

## Authored inputs and generated output

Every level declares its world source and lightmap recipe in
`levels/<directory>/level.toml`. That world source owns `lighting`, including
ambient settings and light definitions. Light IDs are scoped to that world.
The CLI does not read a project-global lighting sidecar or lighting settings
from `karty.toml`.

`karty bake` writes only to the project-root
`.karty/bakes/<directory>/`. The directory component comes from the source
directory under `levels/`, never the logical level name. It writes a
content-addressed QOI image, then atomically publishes `lightmap-prebake.json`.
It leaves authored source files and manifests untouched. Cache directory
components must be ordinary directories; symlinks are rejected when writing.

Builds read each level's cache with project-root path confinement and validate
the complete manifest/image pair against that level's freshly compiled world,
layout, original albedo and configured quality. Missing, stale, corrupt or
escaping cache data falls back to that level's runtime recipe.

## Compatibility

Rerun `karty bake` after upgrading from the previous nested
`levels/<directory>/.karty/bakes/` location. Those old generated caches are no
longer read and can be removed. Caches are reproducible; the CLI does not move
or delete old cache directories automatically.

Explicit `prebake` and `prebake_image` imports remain level-relative, confined
inputs with strict errors for invalid pairs. They do not provide output paths
for `karty bake`. Use `offline = true` in the level recipe to package the
generated project cache automatically.

KLD continues to embed authored lighting in `@world/main`, layouts and prebake
pairs in their existing fixed entries, with no filesystem paths or external
lighting dependencies. SDK versions, wire schemas and released manifests are
unchanged. Regression coverage executes KLD WASM and checks independent levels,
cache placement, deterministic output, stale/corrupt fallback and confinement.

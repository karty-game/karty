# Terrain authoring

These offline tools maintain the [Terrain sample](../../samples/terrain/README.md).
Building or playing the sample uses its retained assets and does not run them.
Run the pinned mise tasks below from the repository root.

| Task                            | Inputs and output                                                                                                                                         |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `author-terrain-structures`     | Uses `cove-layout.mjs` and the retained height/edge data in `data/` to update the marked bunker, jetty, lighthouse and rock sections of `world.yaml`.     |
| `author-terrain-trees`          | Uses retained ground-cover/seabed cohorts to update the outdoor grid and vegetation in `world.yaml`.                                                      |
| `author-terrain-groundcover`    | Prepares the five artwork masters in `assets/`, chooses grass cohorts against the retained terrain, then updates vegetation. Seed searches can take time. |
| `repair-terrain-groundcover`    | Replaces only invalid grass seeds after a terrain edit, preserving valid seeds and authored counts, then updates vegetation.                              |
| `author-terrain-seabed`         | Chooses submerged plant cohorts against the retained terrain, then updates vegetation.                                                                    |
| `author-terrain-paints`         | Regenerates the small deterministic lighthouse paint textures.                                                                                            |
| `author-terrain-tree-materials` | Estimates normal maps from the retained tree artwork.                                                                                                     |

For example:

```sh
mise run author-terrain-structures
mise run author-terrain-trees
```

The tools write into `samples/terrain/levels/ground/`; review those changes before
building. `data/` retains compact seed cohorts and placement measurements used by
the tools and terrain regression tests. Keep them consistent with the authored
`terrain.bin`. That binary is a retained input; these tools read its public SDK
format and do not regenerate it.

`assets/` keeps the original vegetation masters, artwork prompt records and
material provenance. Runtime textures stay beside the level that references
them. Prompts document the original artwork; regenerating an image is optional
and is separate from deterministic texture preparation.

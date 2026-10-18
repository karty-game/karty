# Willowbrook artwork sources

These source sheets and prompts are inputs to Willowbrook's contributor
artwork generator. The game loads the prepared PNGs in
[`samples/willowbrook/levels/village`](../../../samples/willowbrook/levels/village).
Ordinary builds need only files inside the sample.

From the CLI repository root:

```sh
mise run author-willowbrook
mise run check-willowbrook
```

The handwritten generator lives in
[`samples/willowbrook/cmd/author`](../../../samples/willowbrook/cmd/author).
It crops and samples source images, writes reusable level textures and records
texture IDs, aliases and source hashes in
[`internal/village/map.generated.json`](../../../samples/willowbrook/internal/village/map.generated.json).
It preserves the editable Tiled map and tileset, which own object placement,
collisions, animation and foreground masks.

## Selected sources

| Source                                                                                                                         | Purpose                                                   |
| ------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------- |
| [reference-village.png](reference-village.png)                                                                                 | Visual reference; not loaded by the game                  |
| [village-extracted.png](village-extracted.png)                                                                                 | Cottage, trees, rocks, fences, lantern, chest and scenery |
| [hero.png](hero.png)                                                                                                           | Twelve character poses                                    |
| [dual-grid-grass-path.png](dual-grid-grass-path.png)                                                                           | Sixteen grass/path corner configurations                  |
| [staircase-intact-v2.png](staircase-intact-v2.png), [bridge-intact-v2.png](bridge-intact-v2.png)                               | Complete structures and matching foreground pixels        |
| [riverbed-v2.png](riverbed-v2.png), [riverbanks-v2.png](riverbanks-v2.png), [waterfall-v3-outlet.png](waterfall-v3-outlet.png) | Stream bed, banks and outlet                              |
| [grass-tuft-v1.png](grass-tuft-v1.png), [garden-plot-v1.png](garden-plot-v1.png), [notice-board-v1.png](notice-board-v1.png)   | Environmental details and interaction artwork             |

Exact prompts are retained beside these images in `*-prompt.txt`,
[prompts.txt](prompts.txt) and
[artwork-review-prompts.txt](artwork-review-prompts.txt).
The dual-grid atlas uses mask order 00–15 with NW=1, NE=2, SW=4 and SE=8.
[`dual-grid-guide.png`](dual-grid-guide.png) records that layout;
`mise run author-willowbrook-guide` regenerates the guide only.

See the [sample README](../../../samples/willowbrook/README.md) for controls,
Tiled editing, builds and browser checks. Source artwork is not a full-map
background: the map places independently reusable tiles and sprites.

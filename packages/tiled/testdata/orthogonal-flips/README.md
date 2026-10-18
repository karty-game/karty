# Official Tiled transform oracle

`tiled-1.11.2.png` was rendered by the official Tiled 1.11.2 Linux rasterizer,
independently of this package. The small, self-authored atlas is copied from the
`testdata/tiled-room` consumer. Its yellow top-left corner, dark bottom-right
corner and unequal cross arms expose incorrect flips and stretching.

The 32 isolated cases cover all eight GID flip combinations in four rows:

1. A tile layer scaling 16×24 source pixels into 24×32 map cells.
2. The same cells with a tileset offset of (2, −3), scaled with the image.
3. Tile objects rotated 90° around their bottom-left alignment point.
4. Tile objects rotated 270° around bottom alignment, including the scaled offset.

Generate the reference from the CLI repository root with an official Tiled
1.11.2 installation, then review the resulting image before replacing the golden:

```sh
tmxrasterizer --no-smoothing \
  packages/tiled/testdata/orthogonal-flips/map.tmj \
  packages/tiled/testdata/orthogonal-flips/tiled-1.11.2.png
mise run test-tiled-package
```

On a headless Linux system, wrap the rasterizer command in `xvfb-run -a`.
The reference implementation is
[Tiled 1.11.2 CellRenderer](https://github.com/mapeditor/tiled/blob/v1.11.2/src/libtiled/maprenderer.cpp#L394-L499).
The package's test inverse-maps its exported sprite geometry into this reference.
It checks coverage at every pixel and color everywhere except exact source
texel-boundary nearest-neighbor ties; Qt's fragment and fallback paths can choose
different adjacent texels at those ties. It does not require Qt or Tiled to run.

The generator executable used for the checked-in reference had SHA-256
`c45db33b8235ae43afd286b0f50a85ba2fc092e8088065c6bd6c34d6e7d62a8d`.

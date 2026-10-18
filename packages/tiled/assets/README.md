# Tiled PNG assets

`assets` imports the PNG rectangles described by a parsed Tiled map. It has no
Karty, generated SDK, renderer or image-generation dependency.

```go
func Import(
    fsys fs.FS,
    level *tiled.Map,
    emit func(Input) (string, error),
) (map[string]string, error)

type Input struct {
    Key, Source             string
    Rectangle, SourceBounds image.Rectangle
    Pixels                  *image.NRGBA
}
```

`Key` is `tiled.ImageKey`: the complete source path and source rectangle.
`Rectangle` and `SourceBounds` use decoded-image coordinates. `Pixels` is a
separate zero-origin canvas containing that rectangle. The callback owns this
canvas and may retain or modify it; it cannot change the cached source image.
Tile flips remain transform metadata and do not create duplicated image crops.

The callback chooses a nonempty output name and returns it. The result maps
image keys to those names. A caller can reuse an existing whole-image name when
`Rectangle == SourceBounds`, write PNGs, or retain pixels for an asset pipeline.
Each complete source path is opened and decoded once. Each image key is emitted
once across static sprites and every animation frame, including unplaced frames.
Same-basename files in different directories remain distinct.

Actual PNG headers are checked with `png.DecodeConfig` before pixel decode.
Source rectangles are checked against the decoded bounds without overflowing
coordinate addition. Fixed public limits are:

| Limit                   | Value                                            |
| ----------------------- | ------------------------------------------------ |
| `MaxEncodedImageBytes`  | 32 MiB per source                                |
| `MaxEncodedBytes`       | 128 MiB across unique sources                    |
| `MaxImageDimension`     | 8192 pixels per edge                             |
| `MaxImagePixels`        | 16,777,216 pixels per source                     |
| `MaxDecodedSourceBytes` | 256 MiB, charged conservatively at 8 bytes/pixel |
| `MaxOutputBytes`        | 256 MiB across distinct crops, at 4 bytes/pixel  |

Encoded budgets include trailing bytes after PNG IEND. A one-byte lookahead
rejects oversized streams while permitting files exactly at the limit. Repeated
sprites, flips and frames sharing a source/key do not spend those budgets again.
Decoder working buffers are additional to the pixel budgets. Limit errors wrap
`ErrLimit` and can be recognized with `errors.Is`.

Paths must be valid relative `fs` paths without backslashes, colons or NUL. This is
lexical validation. The caller-supplied filesystem defines physical and symlink
confinement; `os.DirFS` alone does not prevent symlink escapes.

Missing/corrupt PNGs, invalid rectangles, nil arguments, callback errors and empty
output names stop import. On error the returned alias map is nil. Earlier
callback side effects are not rolled back; callers needing transactional output
should stage their files and publish them only after Import succeeds.

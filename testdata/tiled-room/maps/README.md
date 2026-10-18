# Official Tiled rendering reference

`room.tiled.png` was rendered from `room.tmj` by official Tiled 1.11.2:

```sh
xvfb-run -a "$KARTY_TILED_RASTERIZER" --no-smoothing \
  testdata/tiled-room/maps/room.tmj \
  testdata/tiled-room/maps/room.tiled.png
```

Set the distribution's Qt plugin/library paths when its packaging requires it.
Keep this reference separate from the shared package's transforms. The browser
check compares actual Ebiten framebuffer pixels against interior yellow and
blue corner marks from this reference, covering all eight GID flip combinations.
Both authored animation frames share those marks, so the comparison stays valid
while animation runs. Colors inside those regions tolerate small GPU sampling
differences; animation colors and tile-edge pixels are checked separately or
excluded from this reference comparison.

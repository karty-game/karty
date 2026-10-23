# Capture and inspect world content

Use `karty render screenshot` to review authored world geometry, materials,
lighting and authored sprites. It rebuilds the level, runs a native host, writes
a PNG and exits. It does not require compiling or starting the game cartridge.

## Single view

From the game root, select a level directory or logical level name:

```sh
karty schema --check
karty render screenshot --position 2,2,1.7 --yaw 45 --pitch -8 --fov 65 --width 960 --height 540 --output dist/screenshots/room-entry-v1.png room
```

Replace `room` and the camera with values from the actual world. This example
fits the skill's eight-metre room. No separate `karty build` is needed first.
On success stdout prints the absolute PNG path; diagnostics go to stderr.
Check the command's exit status before opening that path: a failed capture can
leave an older image at the requested output. Use revision-labelled names for
before/after views or let the default timestamped output choose a fresh path.

## Place the camera deliberately

| Control               | Meaning                                                                                                                                     |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `--position X,Y,Z`    | Eye in world metres, Z up. Choose a free point inside the room, above its floor and below its ceiling. Default `0,0,1.7` can lie on a wall. |
| `--yaw`               | Degrees: 0 faces +Y, 90 faces +X, 180 faces -Y, -90 faces -X.                                                                               |
| `--pitch`             | Degrees: positive looks up, negative looks down; strictly between -90 and 90.                                                               |
| `--fov`               | Vertical field of view, 5–150 degrees; defaults to project camera.                                                                          |
| `--width`, `--height` | Default project resolution, with maximum 1280×720. Override explicitly if the game's resolution exceeds those limits.                       |

To aim eye `E` at target `T`, set `d = T - E`,
`yaw = atan2(d.x, d.y) * 180/pi`,
`pitch = atan2(d.z, hypot(d.x, d.y)) * 180/pi`.
Near/far clipping comes from project camera settings. The command does not move
or collide a player into a valid position and exposes no orthographic/isometric
projection switch. Use the running game when its actual camera projection matters.

## Open the PNG and judge the result

1. Open the newly produced absolute path with the agent's image-viewing tool
   (`view_image` when available). Read the actual pixels; file existence, dimensions,
   successful exit and logs do not establish that the scene looks correct.
2. Compare with the user's reference or concrete requirements: required landmarks,
   placement/scale, passages and silhouettes, material assignment/repeat scale,
   lighting direction/contrast, unwanted gaps, missing surfaces and occlusion.
   State observable matches and mismatches; do not claim unseen content is correct.
3. Capture another viewpoint for hidden sides or ambiguous depth. For a blank or
   obstructed image, check eye position, facing, floor/ceiling and clipping first.
4. Correct authored world/material/light inputs, then recapture from the same
   camera, resolution and FOV. Open the new image and compare again. Keep relevant
   before/after images under `dist/screenshots/` and report paths plus observations.

If no image-viewing tool or native graphics environment is available, report that
visual confirmation is unperformed. Do not substitute a build pass for that review.

## Multiple views in one native session

Retain meaningful inspection poses in a handwritten `tooling/screenshots.toml`:

```toml
[[views]]
position = [2.0, 2.0, 1.7]
yaw = 45.0
pitch = -8.0
output = "dist/screenshots/room-entry-v1.png"

[[views]]
position = [6.0, 6.0, 1.7]
yaw = -135.0
pitch = -8.0
output = "dist/screenshots/room-reverse-v1.png"
```

```sh
karty render screenshot --batch tooling/screenshots.toml --fov 65 --width 960 --height 540 room
```

Batch accepts 1–64 views, shares one resolution/host session and prints one
absolute PNG path per view. Each view can override position, yaw, pitch, FOV and
output. Relative outputs resolve from the game root. Use per-view outputs instead
of `--output` with `--batch`. Open every image relevant to the change.

## Lighting, host and scope

- For final lighting review, [prepare matching bakes](tools-and-validation.md#bake-lighting-when-its-inputs-change).
  Eligible directional recipes with no valid bake may trigger a quick four-sample,
  one-bounce bake during capture; do not mistake that for configured final quality.
- Native host selection is `--host`, `KARTY_HOST_NATIVE`, staged
  `dist/native/<platform>/karty-host` or `dist/native/karty-host`, then the installed
  SDK host. Check existing overrides/staged hosts when provenance or compatibility
  is uncertain; use the game's matching SDK host. Single capture needs screenshot-v1,
  batch needs screenshot-v2. No private host build is required.
- Native graphics/display support is required. On headless Linux with Xvfb
  installed, prefix the same command with `xvfb-run -a`. Display denial or missing
  graphics libraries are environment failures, not evidence of bad world content.
  `--timeout` bounds native capture after preparation; it is not a total build/bake
  deadline.
- This path does not execute `OnStart`/`OnUpdate`, runtime-spawned props/lights,
  player movement, UI, audio or physics. For those, run `karty dev`, reach the
  relevant game state and capture it using available browser/runtime tooling,
  then open that image. A passive world capture cannot validate client behavior.
- Captures support visual review, including under software rendering. They do not
  establish browser parity, physical-device FPS or audio readiness.

# Native level screenshots

Run `karty screenshot <level>` from a project containing `karty.toml`. Select
the level by its logical name or directory under `levels/`. The CLI builds that
world and its assets, launches the native host, saves a PNG and exits. It does
not need to compile the game client or launch a browser.

```sh
karty screenshot my-level --position 4.1,16.5,1.7 --yaw 24 --pitch 3.4 --output view.png
```

On success, stdout contains the absolute image path. Build and renderer
diagnostics go to stderr, so the path can be captured directly:

```sh
image_path=$(karty screenshot my-level --position 4.1,16.5,1.7 --yaw 24)
```

| Option                | Meaning                                                               |
| --------------------- | --------------------------------------------------------------------- |
| `--position X,Y,Z`    | Camera eye in world units, Z up; default `0,0,1.7`.                   |
| `--yaw degrees`       | Zero looks along +Y; positive turns toward +X. Default zero.          |
| `--pitch degrees`     | Positive looks up; must be strictly between -90 and 90. Default zero. |
| `--fov degrees`       | Vertical field of view from 5 to 150; defaults to project camera.     |
| `--width`, `--height` | Image dimensions; default project resolution, maximum 1280 × 720.     |
| `--output`, `-o`      | PNG path; default `dist/screenshots/<level>-<UTC timestamp>.png`.     |
| `--host path`         | Local native host executable, overriding host discovery.              |
| `--batch path`        | JSON array of 1–64 views captured in one native session.              |
| `--timeout duration`  | Native capture timeout, default `1m`; asset building happens first.   |

Near/far clipping distances come from the project camera. Choose an eye point
inside the level; this command does not move or collide a player into position.
The host waits for camera rendering and directional lighting before reading the
image. Captures contain the world, its materials, lights and authored sprites;
the game client, its UI, audio and gameplay are not started.

The CLI validates the fresh PNG and its dimensions before publishing it at the
requested path. A failed capture returns an error and preserves an existing
image at that path.

## Batch capture

```sh
karty screenshot my-level --batch views.json
```

For example, `views.json`:

```json
[
  {"position": [6, 2.2, 1.7], "yaw": 0, "output": "arrival.png"},
  {"position": [4.1, 16.5, 1.7], "yaw": 24, "pitch": 3.4, "output": "view.png"}
]
```

The level, textures and lighting load once. The same host process and graphics
loop render every view, with exact camera placement between captures. All views
share the command's width and height; each may override `position`, `yaw`,
`pitch` and `fov`. Omitted camera fields inherit command defaults. Missing
outputs receive unique timestamped paths. Relative paths resolve from the
project directory. `--output` is for single captures; batches use per-view paths.

Stdout contains one absolute PNG path per line, in request order. The CLI checks
all fresh images before publishing them. Empty batches, duplicate output paths,
unknown fields and request files larger than 64 KiB are rejected. The timeout
applies to the whole native session.

## Automatic quick baking

Before rendering an enabled directional lightmap recipe, the CLI validates any
existing bake against the current geometry, authored lights, surface reflectance
and denoiser. It reuses a valid bake, including one with higher sampling quality.
Missing, stale or corrupt automatic bakes are regenerated using **four samples
and one bounce**, then packaged into the captured level. A batch bakes once.

The generated pair lives in `.karty/bakes/<level-directory>/`. Screenshot
transport settings are temporary; the CLI does not edit the level manifest.
Explicit prebake imports keep their strict validation. Worlds without an enabled
directional recipe or selected bake lights have nothing to bake. Automatic
baking requires an SDK advertising `world/lightmaps-prebaked@1`.

## Host selection

Selection order is `--host`, `KARTY_HOST_NATIVE`, a staged project
`dist/native/<platform>/karty-host` or `dist/native/karty-host`, then the
SDK-pinned installed native host. Windows uses `karty-host.exe`.
The selected host must support **screenshot-v1**; older hosts report an error.
The CLI uses a versioned process interface and public SDK formats, and does
not import private engine source.

For local engine development, build from the engine repository root with
`mise run build-host`, then set `KARTY_HOST_NATIVE` to the resulting executable.
For Linux amd64, for example:

```sh
export KARTY_HOST_NATIVE=/path/to/karty-engine/dist/host/linux-amd64/karty-host
```

Native graphics need a display and OpenGL. On headless Linux:

```sh
xvfb-run -a karty screenshot my-level --position 4.1,16.5,1.7 --yaw 24
```

Material generation still uses the SDK-pinned asset tools and caches. Use
`karty bake` to generate a higher-quality offline bake before a capture.

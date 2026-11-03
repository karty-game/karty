# Software captures

Use an explicit software backend for CLI screenshots and reflection baking on
CPU-only CI runners or agent tooling:

```sh
karty render screenshot levels.room --host /path/to/native-bundle --backend software --output dist/room.png
karty render screenshot levels.room --host /path/to/native-bundle --backend software --batch views.toml
karty render reflections levels.room --host /path/to/native-bundle --backend software
```

Software capture requires a validated WebGPU native host bundle and a compatible
CPU adapter available to that runtime. A missing adapter produces an error; the
command does not switch backends automatically. Capture commands load the level
without running the game client.

`--backend hardware` is the default. Older hosts without software capture support
reject the selection before building level inputs. Software captures establish
functional output, not device performance.

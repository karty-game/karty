# Terrain: coastal cove

Explore a tropical cove with a beach, wooden jetty, cliff bunker and lighthouse.
Walk the connected trails, swim beneath the ocean and inspect terrain-attached
palms, grass and seaweed.

[All samples](../README.md)

![Palms beside the cove, bunker and wooden jetty.](preview.jpg)

## Run it

Complete the [sample setup](../README.md#run-a-sample), then run from the repository root:

```sh
mise run build-terrain-web
mise exec -- ./dist/karty serve samples/terrain/dist/web --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080/`. The sample pins SDK 0.0.14 and uses its matching
public host. For native packaging, run `mise run build-terrain-native`.
Run native and web builds sequentially because they share generated UI staging.

## Controls

| Control                         | Action                                                            |
| ------------------------------- | ----------------------------------------------------------------- |
| WASD / Shift                    | Walk / run                                                        |
| Click, then mouse or arrow keys | Capture the pointer and look                                      |
| Escape                          | Release the pointer                                               |
| Space / C                       | Rise / dive while swimming                                        |
| V / R                           | Cycle eleven landmarks / return to the beach                      |
| Touch                           | Left stick moves; right stick looks, including swimming direction |

## Explore the project

- `levels/ground/terrain.bin` contains the retained heightfield and ground paint.
  The public CLI imports it without private engine tools.
- `levels/ground/world.yaml` connects the terrain to ordinary sectors, static
  structures, vegetation and the ocean.
- `src/main.go` selects viewpoints; `src/fps_controller.go` and `internal/fps/`
  combine accepted physical movement, camera control and swimming.
- `levels/ground/` contains the textures used by the level. The lighthouse is
  an exterior landmark; its door does not open.

For structure, planting and material changes, use the maintained
[terrain authoring tools](../../scripts/terrain/README.md). Source artwork and
provenance live with those tools; game builds consume the authored level inputs.

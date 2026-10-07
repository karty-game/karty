# 🏭 The Hangar

**Follow the light into the reactor.**

Look through the service windows, climb to the control deck and cross a zigzag
above a recessed trench. Warm lamps, cool reactor light and deep shadows shape
an original Doom-inspired industrial world. Every passage is open for exploration.

[More demos](../README.md)

## Take a look around

**WASD** walks; click the scene to look with the **mouse**, and press **Esc** to
release it. Hold **Shift** to run; arrow keys also look. On a smartphone, use the
**left joystick** to walk and the **right joystick** to look. They appear on first
touch. Use view buttons or **V** to jump between landmarks, and **H** to hide the
interface.

## Run it

[Set up once](../README.md#make-it-yours) and install **SDK 0.0.10**, then from
this sample's folder:

```sh
../../dist/karty build --target native
cd dist/native
./karty-host --cartridge game.kart
```

## Capture the views

From the sample's folder, capture the included viewpoints in one native session:

```sh
../../dist/karty screenshot hangar --batch screenshots.json
```

Each capture prints its image path. See [camera options](../../docs/screenshot.md)
to choose your own angle.

## Make it yours

Change the rooms and lights in [world.yaml](levels/hangar/world.yaml), materials
in [level.toml](levels/hangar/level.toml), or the nearby texture PNGs. Rebuild to
see your changes. The green trench basin is ready for a future emissive experiment.

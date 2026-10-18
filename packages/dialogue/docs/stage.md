# Stage direction

`dialogue.Stage` is an optional, renderer-independent visual novel stage. It
runs in user land and does not require a new engine API. Register the cast,
select who is on stage, and feed one `Step` per fixed 60 Hz update:

```go
motion := dialogue.NewStage("conductor", "passenger")
motion.SetScene("platform")
_ = motion.SetPortrait("conductor")
_ = motion.SetFraming("close")
_ = motion.Gesture("conductor", "nod")
motion.Step()
frame := motion.Snapshot()
// Project frame.Background and frame.Actors through your sprite renderer.
```

Transforms use viewport-relative centre positions and heights, with rotations
in radians. Up to eight actors and bounded ambient particles fit inside a
snapshot without per-frame allocation. `SetPortrait("")` makes the cast leave;
`SetEnsemble(true)` arranges everyone across the stage. `wide`, `medium` and
`close` smoothly adjust framing. `nod`, `recoil` and `reach` are brief authored
reactions rather than an automatic movement on every line.

The platform scene uses rain; other scenes use subtle sway and intermittent,
low-opacity window lights. Renderers may ignore or replace those sample ambient
particles for another game's setting. Expression textures belong to the game's
asset renderer, independently of actor identity or dialogue captions.

`SetReducedMotion(true)` immediately settles entrances/framing and disables
continuous motion, rain, passing lights and reaction movement. Expressions and
story choices still work. Store that preference with the game's other profile
data. `Settle()` finishes transient motion after loading an authored composition;
phases are deliberately not a save format. Pause stage updates in menus/history
when appropriate for the game.

Last Train's `internal/stage` adapter uses existing sprite transforms, alpha and
vectors for desktop motion. Its responsive `TrainStage` UI keeps portrait
expressions, chapter composition and sliding entrances on narrow screens.
The sample keeps a steady single-character shot, with one persistent portrait
container so changing a line or expression never replays the entrance. Continuous environmental motion is currently desktop-only: the pinned
SDK does not expose viewport dimensions to cartridge code, while responsive UI
styles do not support animated transform bindings. Reduced motion also removes
the narrow-screen entrance transitions.

# Input

For a default first-person player, attach the
[`controllers-3d` package component](cameras.html#first-person-player). It owns movement
input, pointer capture and touch feedback; keep game hooks for your own actions.
Its generated adapter connects those controls to the engine's camera, body
and component lifecycle APIs.

## Keyboard

Keyboard hooks deliver presses and releases. Keep held state for `OnUpdate`
and clear it through `OnInputReset`.

```go
func keyboard(held map[engine.Key]bool) engine.Hooks {
    return engine.Hooks{
        OnKeyDown: func(key engine.Key) { held[key] = true },
        OnKeyUp: func(key engine.Key) { delete(held, key) },
        OnInputReset: func() { clear(held) },
    }
}
```

## Pointer input

| `PointerInput` field | Meaning                                      |
| -------------------- | -------------------------------------------- |
| `ID`, `Kind`         | Pointer identity and mouse/touch source      |
| `Phase`              | Down, move, up or cancel                     |
| `X`, `Y`             | Logical canvas coordinates                   |
| `DeltaX`, `DeltaY`   | Relative movement for dragging or mouse look |
| `Captured`           | Whether capture is active                    |

`SetCursorCaptured(true)` requests mouse capture. Check `Captured` before using
capture-dependent controls, and release capture when leaving them.

Track each touch by ID. `PointerCancel` ends one gesture; `OnInputReset` clears
all held state. Modal UI can consume gameplay input before your hooks receive it.
Use [Camera 2D](cameras.html#camera-2d) to convert logical pointer positions into world positions.

[UI input routing](../guide/ui.html) · [Crate room touch controls](https://github.com/karty-game/karty/blob/main/samples/crate-room/src/input.go)

The full event bounds and routing rules are in `.karty/docs/pointer-input.md`.

# Turn movement into an interaction

Use typed hooks for startup, frames, input, and level results. Match each level
result to the request that created it. Handle `OnLevelFailed` with a message
the player can read.

These snippets extend a controller with a loaded `camera`. Add these fields
to your `Game`:

```go
status string
nearGoal, won bool
```

After creating the camera, check its starting position and subscribe to later
accepted poses. Registration alone does not call the callback.

```go
updateGoal := func(position engine.Vec3D) {
    dx, dy := position.X-6, position.Y-6
    game.nearGoal = dx*dx+dy*dy < 1
}
updateGoal(game.camera.Settings.Position)
_, err := game.WatchTransform(game.camera, func(pose engine.Transform3D) {
    // The host has applied collision and portal movement.
    updateGoal(pose.Position)
})
if err != nil {
    game.status = err.Error()
}
```

Propose movement from `camera.Settings`, which the hook keeps synchronized
with the accepted pose. Do not accumulate a separate guessed player position.

The position check defines a one-metre trigger around `(6, 6)`. Call this method
from your existing `OnKeyDown` handler to finish when the player presses E there.
Keep the movement handler in that hook too.

```go
func (game *Game) onKeyDown(key engine.Key) {
    if key == engine.KeyE && game.nearGoal && !game.won {
        game.won = true
        game.status = "You made it!"
    }
}
```

Draw `status` in your HUD; invalidate it when the value changes if using
KartUI. Reset `won` when starting again. Reset held controls in `OnInputReset`
so losing focus does not leave the player moving.

Tags identify actors; they do not automatically create proximity events. Query
actors after the level is ready, and keep trigger checks in your game. For
longer scripted interactions, the court sample uses typed actions and
`StartAuthoredSequence`; start with a direct Go handler first.

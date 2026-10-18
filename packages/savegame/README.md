# Profile and run storage

A portable organization layer over Karty's keyed save API. Provide a non-nil
`Backend` (the generated `Game` implements it), then use separate lifetimes:

```go
saves := savegame.Store{Backend: &game}
_ = saves.WriteProfile([]byte("any game-defined settings or unlock data"))
_ = saves.WriteRun("1", []byte{0, 255, 42})
profile, err := saves.ReadProfile()
run, err := saves.ReadRun("1")
_ = saves.DeleteRun("1") // Leaves profile and other runs intact.
_, _, _ = profile, run, err
```

Payloads are arbitrary bytes, including empty or non-UTF-8 data. This package
adds no schema, envelope, version checks, serialization or migration policy.
Each game chooses what to store and how to interpret it. Loading a run reads
only that slot; the game applies it without replacing its current profile.

The stable keys are `profile` and `run_<slot>`. Slot names contain 1–60 ASCII
letters, digits, underscores or hyphens. They do not include schema versions.
Backend errors pass through unchanged. With Karty, each value can occupy the
full 48 KiB, and all keys share the game's 64-key / 2 MiB quota. Writes are
independent transactions; profile and run updates are not one transaction.
There is no slot enumeration or cloud synchronization.

Last Train demonstrates three run slots, immediate audio preference writes and
lifetime ending unlocks. Its payload validation belongs to the sample, not this
package or the engine. Run `mise run test-dialogue-packages` from the CLI root.

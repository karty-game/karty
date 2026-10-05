# UI and inventory sample

Build from this directory with `../../dist/karty build --target web` or run
`../../dist/karty dev`. From the menu, start a level, pause, and open Inventory.

The inventory replaces the old `karty-ui-prototype` demonstration. It uses the
normal compiled KartUI components and SDK UI protocol: host action events call
the client handlers, and invalidated components send batched presentation updates.
There is no experimental inventory transport or host-owned gameplay logic.

The client owns potion quantity. Keyed `ItemRow` components own their local use
counters; Add, Remove and Reverse demonstrate keyed reconciliation. Closing and
reopening the inventory resets component state while preserving game data.
Idle frames send no presentation updates. `mise run check-ui` executes those
behaviors with a real TinyGo cartridge and browser host.

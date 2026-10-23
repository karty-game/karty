# Orbital flight deck

Customize a ship while it cruises above a live flight console. This sample
shows how menus, tabs, settings, tooltips and inventory connect to a running game.

![The orbital flight deck interface.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/ui-demo/) · [All samples](../README.md)

## Try it

Choose **Play flight demo**. Complete a delivery under **Mission**, change
ships under **Loadout**, or adjust your call sign and thruster power in
**Settings**. Press **Escape** to pause and open the inventory. Resize the
window to see the interface adapt.

## Built with

- [KartUI](https://github.com/karty-game/karty-ui): menus, settings, tooltips and inventory layouts.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
cd samples/ui-demo
../../dist/karty dev
```

Explore [the game](src/main.go), [cockpit](src/cockpit.go) and [UI layouts](ui/).

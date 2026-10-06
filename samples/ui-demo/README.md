# Orbital flight deck

A small game screen built with current KartUI `.kui` single-file components,
Go setup blocks and indented styles. It requires **SDK 0.0.9 / API 0.0.7**, with
widget schema 10 and explicit sizing schema 11. Follow the
[candidate SDK/host setup](../README.md), then run `../../dist/karty dev` here.

Choose **Play flight demo** from the Orbital menu. A ship cruises above an
interactive, nonmodal console. Its tabs demonstrate real game controls:

| Tab      | Controls and behavior                                                                                                                   |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| Mission  | Record deliveries, update the mission counter, open the pause menu                                                                      |
| Loadout  | Choose Explorer or Interceptor with a combo; ship color and speed change immediately; a locked freighter demonstrates a disabled choice |
| Settings | Edit the pilot call sign, toggle autopilot, adjust thruster power with a slider, reset the flight settings                              |

Tooltips explain each control. Tab selection and flight preferences survive
pause/resume and switching destinations because the game owns the model. Input
editing uses the host's text-focus capture. Escape or Pause opens a modal screen;
Resume returns to the saved console, Inventory opens supplies, and Return to menu
releases the level. UI clicks do not accidentally pause the game.

The menu, modal screens and layouts share a theme. The console uses percentage
sizing and a narrower stacked settings layout on small viewports. Components
update through callbacks/invalidation; idle frames do not re-project UI text.
The frame hook advances only the moving ship.

The keyed inventory retains its existing Add, Remove, Reverse and Use behavior.
Each `ItemRow` owns a local use counter; keyed reordering preserves it. Closing
and reopening Inventory resets row state while game-owned potion quantity survives.
Layouts are build-time structure with named slots and scoped styles.

Start with [game-screen.kui](ui/views/game-screen.kui),
[menu.kui](ui/views/menu.kui), [inventory.kui](ui/views/inventory.kui),
[window-layout.kui](ui/layouts/window-layout.kui) and [main.go](src/main.go).
The two levels also include static `.kui` HUD assets as examples of level-owned
presentation; the interactive flight deck uses the generated client component.

From the CLI root, `mise run test` compiles the authored UI sources.
`KARTY_HOME=/path/to/candidate-sdk-cache mise run check-sample-clients` additionally
compiles all clients and runs a small CPU fixture through the actual generated
widget callbacks. It checks movement, preferences, pause/remount and idle commands
without loading sample assets, a baker, graphics or a browser. This is native Go
compilation/execution evidence. Manual graphics, keyboard navigation and touch
review remain separate. The optional `check-ui` exercises the older dedicated
compatibility fixture, rather than this expanded game screen.

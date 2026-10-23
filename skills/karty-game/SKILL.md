---
name: karty-game
description: Author and debug Karty games with the pinned SDK, built-in lights/physics/UI/audio, asset and level pipeline, Go cartridge APIs, and CLI validation. Engine and toolchain implementation changes follow their repository instructions.
---

# Karty games

Work from the directory containing `karty.toml`. Assume access to the game,
public CLI and installed SDK only; no engine source or pre-existing sibling checkout.

## Resolve the contract

- Read `karty.toml`, `go.mod`, the project README and relevant source. The exact
  `[sdk].version` selects bindings, capabilities, tools and hosts; the CLI has its
  own version. Preserve the pin unless the task includes an upgrade.
- Read `.karty/docs/README.md` and the relevant feature reference. Inspect
  `.karty/engine/`, `.karty/assets/` and `.karty/ui/` for exact signatures and
  generated names. Use the game's `go.mod` module path in imports.
- `karty dev` or a finite development build regenerates missing/stale docs and bindings.
  `karty <command> --help` gives installed CLI flags. Newer samples or a candidate
  version number do not establish feature availability in this SDK.
- For questions this skill or the bundled docs do not answer, use
  [Public sources](references/public-sources.md): topic-specific website guides,
  API pages, package references and working samples. Check the linked example's
  SDK/CLI requirements before adapting it; do not guess a missing API.

## Edit and preview

Start from the closest supported starter and get its unchanged preview running
before adding features. Read [tool setup](references/tools-and-validation.md#setup)
once, then keep one terminal/session running:

```sh
karty dev --port 0
```

Open the printed URL; wait for `Preview updated (build N)`. Edit source and save;
the same process rebuilds and updates/restarts the browser game. Build errors keep
the previous successful game available, so a visible game is not proof the latest
edit built. Fix the reported authored file and wait for the next successful update.
Stop your preview with Ctrl+C when finished; do not start another per edit.

In a second terminal, run `karty fmt` to apply Go formatting and `karty lint` after
a successful rebuild. Use `karty schema --check` for world/level edits and focused
portable tests for changed rules. Dev supplies generated bindings; do not also
run a separate development build after every save. Baking and final production
checks are in [Tools and validation](references/tools-and-validation.md).

## Use built-in systems

Map each feature to [Code and capabilities](references/code-and-capabilities.md)
before implementing it. Default to authored/runtime lights, host `World`/`Body`
physics and queries, supported navigation, KartUI widgets/layout, host audio,
animations and asset processors. Game code owns rules, state, interactions and
adapters. Avoid duplicate lighting renderers, physics solvers, pathfinders,
widget systems or audio mixers when Karty supplies the required behavior.

For an unsupported requirement, identify the documented limit and choose a
supported design or narrow game-side extension. Honor an explicitly requested
custom implementation. Diagnose with generated contracts, CLI/asset reports and
host/browser logs; report SDK/host defects with a minimal public reproduction.
Do not patch generated bindings or depend on private implementation access.

## Do not

- Do not draw HUD/interface text with `Text2D` or recreate menus, buttons and
  panels with game-layer shapes. Use KartUI components under `ui/`: they render
  at presentation resolution. A higher numbered layer cannot move game text into
  the UI pass. Use `Text2D` for game-space text. Game graphics such as aiming
  reticles can stay in the game pass.
- Do not replace a supplied light, physics, navigation, UI or audio system
  without first identifying a documented gap or an explicit custom requirement.
- Do not edit generated `.karty/` files or restored package copies; change
  authored inputs and regenerate.
- Do not integrate a host body's position yourself or drive its pose through
  ordinary transform setters. Use body controls and accepted poses.
- Do not treat an old preview, a passing build or an unopened screenshot as
  runtime/visual validation. Passive world captures do not execute game UI.

## Source layout

- `src/main.go`: cartridge registration and coordination; split player, camera,
  input, HUD, audio and scene owners into related files as needed.
- `internal/`: optional portable rules/packages; keep generated-SDK adapters in
  the consuming game.
- `assets/`, `ui/`, `levels/`, `packages/`: authored content and required package
  inputs; exact paths/formats are in [Assets and levels](references/assets-and-levels.md).
- `.karty/`: generated docs, bindings, schemas, restored packages and caches;
  read, do not hand-edit. `dist/`: regenerated output/reports. Keep both ignored.
- `.agents/skills/karty-game/`: user-owned skill copied by `karty new`; builds and
  SDK upgrades do not refresh it. Review updates deliberately.

For a new project: `karty new --yes --template 2d my-game`; alternatives are
`fps`, `isometric`, `ui`, `blank`. `--sdk VERSION` selects an exact SDK.
`--no-template` selects blank, which renders nothing until code creates a scene.
For a blank 3D game, follow the selected SDK's `client-hooks.md` and `cameras.md`:
request the level, match `OnLevelReady`, then create its camera; handle failures
and release the mount when leaving.

## Validate the changed behavior

Use [Tools and validation](references/tools-and-validation.md) for setup and
commands. World edits start with `karty schema --check`. Regenerate before linting
changed assets/UI/actions. A fast Go development build does not establish
production TinyGo compatibility. Execute the changed path for runtime behavior.
For authored world appearance, [capture and inspect screenshots](references/visual-validation.md):
open the generated PNGs, compare the scene with the requested result, correct
visible mismatches and recapture. A successful capture command alone is not a
visual review. Client-created content and UI require a running-game capture.

Report exact commands/outcomes and SDK/compiler/runtime. Distinguish compilation,
WASM execution, native/browser graphics, audio and physical-device performance.

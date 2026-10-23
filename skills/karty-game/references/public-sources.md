# Public sources

Use the narrowest reference that answers the question. Exact APIs and limits come
from the game's `.karty/docs/` and generated bindings. The links below describe
current public sources; `main` and the website can be newer than the game's SDK.

## Guides and API behavior

| Question                                         | Public reference                                                                                                                                                                                                                                                                         |
| ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Install, create and preview a game               | [Getting started](https://karty-game.github.io/karty/main/guide/); [2D](https://karty-game.github.io/karty/main/guide/2d.html), [first-person](https://karty-game.github.io/karty/main/guide/fps.html), [isometric](https://karty-game.github.io/karty/main/guide/isometric.html) tracks |
| Level manifests, rooms and reusable geometry     | [Levels](https://karty-game.github.io/karty/main/guide/levels.html), [prefabs](https://karty-game.github.io/karty/main/guide/prefabs.html)                                                                                                                                               |
| Textures, materials, processing and retention    | [Textures](https://karty-game.github.io/karty/main/guide/textures.html), [asset pipeline](https://karty-game.github.io/karty/main/design/asset-pipeline.html)                                                                                                                            |
| Authored lights and static baking                | [Lighting guide](https://karty-game.github.io/karty/main/guide/lighting.html); `.karty/docs/lighting.md`, `dynamic-lights.md`, `lightmaps.md` for the pinned contracts                                                                                                                   |
| Actions and game-owned interactions              | [Interaction hooks](https://karty-game.github.io/karty/main/guide/interactions.html)                                                                                                                                                                                                     |
| Menus, layouts and typed callbacks               | [UI guide](https://karty-game.github.io/karty/main/guide/ui.html); KartUI language links below                                                                                                                                                                                           |
| Images, sound, music and video                   | [Media guide](https://karty-game.github.io/karty/main/guide/media.html), [audio API](https://karty-game.github.io/karty/main/technical/audio.html)                                                                                                                                       |
| Entity/level lifetime and resource ownership     | [Runtime API](https://karty-game.github.io/karty/main/technical/runtime.html)                                                                                                                                                                                                            |
| Physics bodies, collision queries and navigation | `.karty/docs/api.md`, `simulation.md`, `physics.md`, `navigation.md`; Crate Room and Vigil examples below                                                                                                                                                                                |
| Keyboard/pointers and coordinate conversion      | [Input API](https://karty-game.github.io/karty/main/technical/input.html), [transforms](https://karty-game.github.io/karty/main/technical/transforms.html), [cameras](https://karty-game.github.io/karty/main/technical/cameras.html)                                                    |
| Sprite and particle rendering                    | [Graphics API](https://karty-game.github.io/karty/main/technical/graphics.html); `.karty/docs/particles.md` for preset contracts                                                                                                                                                         |
| SDK/compiler/host version selection              | [SDK and toolchains](https://karty-game.github.io/karty/main/design/sdk-and-toolchain.html)                                                                                                                                                                                              |
| Reusable code and content dependencies           | [Package workflow](https://karty-game.github.io/karty/main/design/packages.html)                                                                                                                                                                                                         |
| Web/native packaging, CI and distribution        | [Build and share](https://karty-game.github.io/karty/main/guide/share.html), [web delivery](https://karty-game.github.io/karty/main/design/web-delivery-v1.html)                                                                                                                         |

The [API index](https://karty-game.github.io/karty/main/technical/) and
[design index](https://karty-game.github.io/karty/main/design/) route other topics.
For plain text, the site publishes Markdown companions, for example
[lighting.md](https://karty-game.github.io/karty/main/guide/lighting.md).
If a page is unavailable, read the tracked GitHub source under
[guides](https://github.com/karty-game/karty/tree/main/website/content/guides),
[API](https://github.com/karty-game/karty/tree/main/website/content/api) or
[design](https://github.com/karty-game/karty/tree/main/website/content/design).
CLI root `docs/` is not the public website source.

## Package references

Read the [Plugins catalog](https://karty-game.github.io/karty-packages/),
then the selected package's README and `karty-package.toml` at a compatible public
revision. These packages are separate inputs, not automatically installed by the CLI.
The package manifest's optional `sdk` field is a minimum compatible SDK version;
the consuming game's exact SDK pin must meet or exceed that minimum.

| Package     | Use and reference                                                                                                                                                                                                                                                                                                                                                                             |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Controllers | Separate [3D](https://github.com/karty-game/karty-packages/tree/main/packages/controllers-3d), [top-down](https://github.com/karty-game/karty-packages/tree/main/packages/controllers-topdown) and [isometric](https://github.com/karty-game/karty-packages/tree/main/packages/controllers-isometric) packages. The 3D package owns the first-person component and generates its SDK adapter. |
| FPS Kit     | [Prototype materials, crates, bumpers and decals](https://github.com/karty-game/karty-packages/tree/main/packages/fps-kit).                                                                                                                                                                                                                                                                   |
| Dialogue    | [Yarn compilation, story runtime and visual novel UI](https://github.com/karty-game/karty-packages/blob/main/packages/dialogue/README.md). Check its compiler and processor setup.                                                                                                                                                                                                            |
| Savegame    | [Profile preferences/unlocks and independent run slots](https://github.com/karty-game/karty-packages/blob/main/packages/savegame/README.md). The game owns payload encoding and migration.                                                                                                                                                                                                    |
| Tiled       | [Supported Tiled maps, image imports and 2D view adapter](https://github.com/karty-game/karty-packages/blob/main/packages/tiled/README.md). Read the supported map subset and texture-generation steps.                                                                                                                                                                                       |

Acquisition, linked imports and fixed `.kpkg` archives are covered in
[Acquire reusable packages](code-and-capabilities.md#acquire-reusable-packages).

## Working samples

For direct visual checks, use [Capture and inspect world content](visual-validation.md)
and the public [screenshot command source](https://github.com/karty-game/karty/tree/main/internal/commands/screenshot).

[Sample index](https://github.com/karty-game/karty/blob/main/samples/README.md) ·
[Playable demos](https://karty-game.github.io/karty/main/)

| Feature to study                                                   | Source project                                                                                                                                       |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| First-person world, portals, torch, baked lights and spatial audio | [Hangar](https://github.com/karty-game/karty/tree/main/samples/hangar)                                                                               |
| Pushable crates, shots, impulses and physics/UI coordination       | [Crate Room](https://github.com/karty-game/karty/tree/main/samples/crate-room)                                                                       |
| Isometric navigation, sprites and occluder fading                  | [Vigil](https://github.com/karty-game/karty/tree/main/samples/vigil)                                                                                 |
| 2D Tiled maps, following camera and touch input                    | [Willowbrook](https://github.com/karty-game/karty/tree/main/samples/willowbrook)                                                                     |
| Menus, settings, widgets, reusable layouts and modal state         | [UI Demo / Orbital](https://github.com/karty-game/karty/tree/main/samples/ui-demo)                                                                   |
| Yarn dialogue, story state, saves and responsive visual novel UI   | [Last Train](https://github.com/karty-game/karty/tree/main/samples/last-train)                                                                       |
| Terrain, vegetation, sky/environment and world materials           | [Terrain](https://github.com/karty-game/karty/tree/main/samples/terrain)                                                                             |
| Sound effects, streaming music and video                           | [Media Lab](https://github.com/karty-game/karty/tree/main/samples/media-lab)                                                                         |
| Authored particle recipes, schedules and source-generation steps   | [Particle Lab](https://github.com/karty-game/karty/tree/main/samples/particle-lab)                                                                   |
| Material/lighting comparisons and host presentation effects        | [Effect Lab](https://github.com/karty-game/karty/tree/main/samples/effect-lab). A debug-panel effect is not proof of a guest API for controlling it. |

To adapt a sample:

1. Read its README, `karty.toml` and `go.mod`. Check SDK, host, package and authoring
   prerequisites before copying or running anything. A sample may target an
   unpublished candidate; find a compatible public revision instead of changing
   the game's pin implicitly.
2. Start at `src/main.go`; follow the relevant owners into `src/`, portable rules
   into `internal/`, presentation into `ui/`, and authored content into `levels/`.
   Inspect only the relevant feature, not the whole repository.
3. Adapt handwritten logic and retained source inputs. Do not transplant `.karty/`,
   generated numeric IDs/tables, or `dist/`; regenerate for the consuming game.
   Generated files can also exist outside `.karty/`. Retain required asset notices.

CLI repository-root mise tasks apply to that public checkout. Use the game's ordinary
`karty` commands unless you intentionally adopt a sample's authoring pipeline.

## Language, formats, releases and CLI behavior

- [KartUI repository](https://github.com/karty-game/karty-ui),
  [language](https://github.com/karty-game/karty-ui/blob/main/docs/0.0.1/kartui.md),
  [styles](https://github.com/karty-game/karty-ui/blob/main/docs/0.0.1/styles.md),
  [indented sizing](https://github.com/karty-game/karty-ui/blob/main/docs/0.0.1/indented-styles.md).
  Use these for grammar/compiler questions; use the game's pinned UI docs for
  supported host behavior. The [SFC demo](https://github.com/karty-game/karty-ui/tree/main/samples/sfc-demo)
  illustrates compilation; UI Demo above illustrates game integration.
- [Public SDK](https://github.com/karty-game/karty-sdk) and
  [world authoring contract](https://github.com/karty-game/karty-sdk/tree/main/format/worldsource)
  cover formats, codecs and shared validation. This repository is not the game's
  generated Go API: gameplay imports still come from `.karty/engine`.
- [CLI releases](https://github.com/karty-game/karty/releases),
  [SDK/host releases](https://github.com/karty-game/karty-sdk/releases),
  [Materialize tool releases](https://github.com/karty-game/karty-tools/releases).
  [Package releases](https://github.com/karty-game/karty-packages/releases) are
  published independently with package-specific version tags.
  SDK bundles/hosts use `sdk-vVERSION` tags; public Go format modules use separate
  `vVERSION` tags. CLI and package revisions are independent of both.
- If CLI help leaves behavior unclear, inspect the public
  [commands](https://github.com/karty-game/karty/tree/main/internal/commands) or
  [build pipeline](https://github.com/karty-game/karty/tree/main/internal/build).
  Do not import CLI implementation packages into the cartridge.

Once a source example is selected, use its public tag/commit in the GitHub URL
(`blob/REVISION/...` or `tree/REVISION/...`) and record that revision when adapting
it. Follow the project's pin when source examples and generated SDK contracts differ.

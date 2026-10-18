# Packages

A package lets several games share a controller, a set of crates, a material
palette or a HUD. It carries source code and original assets so each game can
build them with its own selected SDK.

## One package can supply code and content

`karty-package.toml` gives the package its name, version and exports.
A package can contain Go behavior, static solid prefabs, PNG world materials,
or KartUI components, layouts, images, fonts and theme defaults. It can combine
these exports or provide just the parts a game needs.

A Go module supplies code imports and Go dependencies. A Karty package adds
content exports and their build integration. Code packages still use their
ordinary Go module paths; resource-only and UI-only packages need no Go module.
An SDK bundle serves a different purpose: it supplies the game's engine API,
templates, tools metadata and reference.

## Link while developing and archive to share

Link a working folder when you want package edits to reach a game's next build
or development preview. The game keeps a path to that folder, so another
machine needs the folder too.

From the game project, link an existing package folder:

```sh
karty package import ../warehouse-kit
karty dev
```

The folder must contain `karty-package.toml`. Its manifest name is the package
identity; the folder name does not choose the import name.

Use a `.kpkg` archive for a fixed dependency. The project records its checksum
and keeps a copy under `packages/`. Commit that archive with the project so
another checkout can restore the same package. Updating it is an explicit
replacement; a build does not silently select a newer version.

Export the package from its source folder, then import the archive into a game:

```sh
# Export an existing package; its manifest supplies the version.
karty package export --output ../warehouse-kit-0.1.0.kpkg ../warehouse-kit

# Run from the game project, using the archive's actual location.
karty package import ../warehouse-kit-0.1.0.kpkg
```

When that archive lives outside the project, import copies it under `packages/`
and records its checksum in `karty.toml`. Commit the archive, `karty.toml`, and
any Go module changes. Builds and `karty dev` restore declared packages
automatically; you can also restore them explicitly:

```sh
karty package restore
```

To switch a linked package to an archive, or select a newer archive with the
same manifest name, make the replacement explicit:

```sh
karty package import --replace ../warehouse-kit-0.2.0.kpkg
```

## Player controller presets

The reusable `controllers` package bundles first-person, top-down 2D and
isometric input and camera behavior in one install. Choose a preset, customize
its options, route your game hooks to it and apply its results through your
SDK bindings. There is no separate camera or input package to assemble.

Hangar, Environment, Terrain and Crate Room share first-person movement and
look calculations. Willowbrook uses top-down movement and camera following;
Vigil uses the isometric movement basis, following camera, ground picking and
zoom. Each game retains its collision rules, navigation, artwork and gameplay.

The module path is `github.com/karty-game/controllers`. Version 0.1.0 is local
source, supplied through a linked package or exported archive; a public Go
module tag is not assumed. Its package README contains setup and adapter
examples. This adds no engine API or SDK requirement beyond the consuming
game's existing APIs.

SDK 0.0.13's `2d`, `fps` and `isometric` starters include the
same package source under `packages/controllers/`. Their `karty.toml` links that
folder, so a generated project builds without a published controller module.
Keep the folder in Git. Blank and UI starters add no player controls.
See the [2D](../guide/2d.html), [first-person](../guide/fps.html) and
[isometric](../guide/isometric.html) guides for complete SDK adapters.

## The game owns composition

Suppose `warehouse-kit` exports a `crate-stack` prefab and a `wood` material.
The game places `warehouse-kit.crate-stack` in its world and can override that
instance's material. Other instances keep the package's default.

The same boundary applies to UI: a shared HUD component receives the game's
state and callbacks, while the game chooses where it appears and how long it
lives. Shared Go behavior should accept a small interface from its consumer,
so it can work across games with different generated bindings.

## Packages use the ordinary build pipeline

The CLI resolves exports before compiling the game. Imported images go through
asset processing, prefabs go through world compilation, and KartUI sources
compile with the game's SDK. Generated files stay in `.karty/`; package authors
distribute sources rather than a game's generated bindings.

The current package workflow uses local folders and archives. A remote registry,
automatic updates and transitive resource dependencies are outside its current
scope. Particle, audio and video exports are also not supported yet.

Follow [Build and share](../guide/share.html#share-reusable-code-prefabs-and-ui)
to import or export one. Read [Asset pipeline](asset-pipeline.html) to see where
its content goes during a build.

## Dialogue package v1

The candidate `dialogue` 0.1.0 package combines the Yarn compiler, portable Go
story runtime, presentation model and visual novel UI in one install and one
archive. Last Train demonstrates this integration. Its optional `savegame`
package supplies generic profile/run storage independently of dialogue.

Game code imports `github.com/karty-game/dialogue`. `Load` reads official
compiled Yarn programs and their string table; `NewStory(program)` creates a
story runner, while `New()` creates presentation state. The package supplies
`NovelFrame`, `NovelDialogue`, choices and history to the game's generated UI.
Games own artwork, fonts, scenes and custom commands, and may use their own UI.

The game explicitly selects `dialogue.compile` and its `.yarn` input paths in
`[[build.processors]]`. Builds require .NET SDK 9.0.203 and Yarn Spinner 3.2.1's
locked compiler dependencies; the shipped game uses a pure Go runtime. Initial
Yarn feature limits are documented in the package's compatibility reference.

This replaces the unreleased separate `yarn` and `novel-ui` packages. Migration
removes those package entries and the Yarn Go dependency, uses `NewStory`, and
changes processor selection/resource reads from `yarn.compile` to
`dialogue.compile`. UI identities move from `novel-ui.*` to `dialogue.*`;
generated component type names remain unchanged. No engine, SDK or wire change
is required. The [binary runtime v2 decision](https://github.com/karty-game/karty/blob/main/packages/dialogue/docs/binary-runtime-v2.md)
replaces the candidate's CSV tables and run snapshots. Rebuild the game and
start a new run; lifetime preferences and ending unlocks retain their format.

The same candidate includes an optional `dialogue.Stage` controller for authored
entrances, framing, short reactions and ambient motion. It exposes bounded,
viewport-relative snapshots to a game's renderer and needs no engine change.
Games save authored composition alongside the story; animation phases remain
transient, and reduced motion belongs with lifetime preferences. Last Train
demonstrates sprite-based desktop staging and responsive portrait UI. Continuous
environmental motion and gesture movement currently use the desktop renderer;
the narrow-screen UI supports expressions, group composition and entrances.

## Tiled package v1

The local `tiled` 0.1.0 candidate shares a build-time Tiled JSON parser, PNG
importer, bounded binary map codec and 2D sprite view. It is not a published package release. Read the
[package reference](https://github.com/karty-game/karty/blob/main/packages/tiled/README.md)
and its two independent consumers:
[Willowbrook](https://github.com/karty-game/karty/blob/main/samples/willowbrook/README.md)
uses TinyGo; [Tiled room](https://github.com/karty-game/karty/blob/main/testdata/tiled-room/README.md)
uses Go.

From either sample's project directory, link the working package:

```sh
karty package import ../../packages/tiled
karty package restore
```

For a fixed dependency, export from the repository root, then replace the link
from the consuming game:

```sh
# Repository root; choose a new output file.
karty package export --output /tmp/tiled-0.1.0.kpkg packages/tiled

# Game project; import copies and checksums the archive.
karty package import --replace /tmp/tiled-0.1.0.kpkg
```

Both consumers call the shared image importer from their own author generator,
which writes ordinary level texture inputs. From the repository root, run
`mise run author-willowbrook` followed by `mise run build-willowbrook-web`, or
`mise run author-tiled-room` followed by `mise run build-tiled-room-web`.
Use the SDK and matching public host selected by those samples.

Game code imports `github.com/karty-game/tiled` and its `karty2d` subpackage.
The game supplies a small create/update/destroy Sprite2D backend using its own
generated bindings and mounted textures. The shared view handles bounded
residency, authored animation and optional wind; the game owns its camera,
input, collision rules, actors and HUD. Static sprites stay in world coordinates
while the existing SDK Camera2D follows the player. No engine API or wire change
is needed to use this package.

The supported subset is finite orthogonal `right-down` JSON maps with image
collections or atlases, group offsets/opacity, tile flips/rotation and timed tile
animations. Infinite maps, encoded/compressed tile data, image layers and
external object templates are rejected. The package reference lists the exact
geometry limits and input/allocation bounds. Importing this code package does
not automatically register PNGs as level textures; that remains the consuming
game's explicit authoring step.

### Runtime format boundary

Editable Tiled JSON and Yarn source remain build inputs. Games explicitly
select `tiled.compile` or `dialogue.compile`; their generated resources are
binary. Dialogue string tables and run saves are binary too. The CLI checks
the actual staged cartridge dependency closure before compilation, rejecting
JSON, CSV, XML, gob and known YAML/TOML decoders.

See the [compiled Tiled map contract](https://github.com/karty-game/karty/blob/main/packages/tiled/docs/binary-map-v1.md)
and [dialogue/save compatibility](https://github.com/karty-game/karty/blob/main/packages/dialogue/docs/binary-runtime-v2.md).

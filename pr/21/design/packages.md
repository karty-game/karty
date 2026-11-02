# Packages

A package lets several games share a controller, a set of crates, a material
palette or a HUD. It carries source code and original assets so each game can
build them with the rest of its source.

To create a repository, publish a release and request a catalog listing, follow
[Write and publish a package](../guide/packages.html).

## One package can supply code and content

`karty-package.toml` gives the package its name, version and exports.
A package can contain Go behavior, static solid prefabs, PNG world materials,
transparent PNG sprites and decals, or KartUI components, layouts, images,
fonts and theme defaults. It can combine these exports or provide just the
parts a game needs. Use a sprite texture export in `actor.sprite.texture`;
`facing: fixed` places a decal on a surface, with actor rotation controlling its
orientation. The `fps-kit` package includes a complete placeholder decal set.

A Go module supplies code imports and Go dependencies. A Karty package adds
content exports and their build integration. Code packages still use their
ordinary Go module paths; resource-only and UI-only packages need no Go module.

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

Use a `.kpkg` archive for a fixed dependency. Importing a local archive records
its checksum and keeps a copy under `packages/`. Commit that archive with the
project so another checkout can restore the same package. Updating it is an explicit
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

## Pin a release archive over HTTPS

A package declaration can point to a versioned HTTPS `.kpkg` release URL with
its publisher-provided SHA-256 checksum. The first restore downloads a bounded
archive, verifies its checksum and validates its contents. Later restores check
and reuse the cached archive. Keep the URL and checksum in `karty.toml`; updates
require changing that pin explicitly.

Import a published package by its catalog name, or select an exact version:

```sh
karty package import fps-kit
karty package import fps-kit@0.0.1
```

The first form chooses the latest stable version when imported. The second
requires that exact published version. These examples show command syntax;
the chosen package and version must be available in the catalog. Both forms
record the resolved archive URL and SHA-256 in `karty.toml`. Restore and builds
reuse that fixed pin without consulting the catalog; a verified cached archive
also supports restoration offline.

Use `karty package upgrade fps-kit` to update an installed package to its latest
stable catalog release. Upgrade requires an existing archive dependency and
always resolves the catalog name, even if a local folder has the same name.
It preserves linked development folders; explicitly replace a link with
`karty package import --replace fps-kit@0.0.1` when you want a published archive.
Use that same exact-version import to select a specific release.

For imports, an existing local folder named `fps-kit` takes priority over the
catalog; use `./fps-kit` to explicitly select a local path. Local folders and
archives remain supported. You can also configure URL dependencies directly in
`karty.toml`.

The CLI uses the official
[package catalog](https://karty-game.github.io/karty-packages/master.json),
which can list packages from different repositories. Import checks the archive
checksum, package name and version, complete contents and project compatibility
before changing authored project files. See the
[named package imports v1 decision](named-package-imports-v1.html) for resolution
and compatibility rules.

The FPS starter declares the published
[FPS Kit](https://karty-game.github.io/karty-packages/packages/fps-kit.html) through this
mechanism. Its template carries the dependency pin, and restored package docs
appear under `.karty/packages/fps-kit/`.
See the [remote package archives v1 decision](remote-package-archives-v1.html)
for declaration syntax, HTTPS rules and cache validation.

## Player controller presets

Choose the published controller package matching your game:

| Package                 | Go module                                     | Usage                     |
| ----------------------- | --------------------------------------------- | ------------------------- |
| `controllers-3d`        | `github.com/karty-game/controllers-3d`        | FPS movement and controls |
| `controllers-topdown`   | `github.com/karty-game/controllers-topdown`   | Willowbrook, 2D starter   |
| `controllers-isometric` | `github.com/karty-game/controllers-isometric` | Vigil, isometric starter  |

For example, run `karty package import controllers-topdown@0.0.1` from an
existing game. Customize the preset options, route game hooks to it and apply
its results through your game's engine bindings. The Go package name remains
`controllers` within each separate module.

Hangar, Terrain, Crate Room and Effect Lab use the `controllers-3d` package's
first-person component and generated adapter. Willowbrook uses top-down movement and camera following;
Vigil uses the isometric movement basis, following camera, ground picking and
zoom. Each game retains its collision rules, navigation, artwork and gameplay.

Package source and release tooling live in
[`karty-packages`](https://github.com/karty-game/karty-packages), independently
of CLI releases. Imports pin checked archives and stage their Go modules
under `.karty/packages/`; a separate public Go module tag is not needed.
Package READMEs contain setup and adapter examples.

The `2d` and `isometric` starters pin their matching controller
release in `karty.toml`. The FPS starter pins `controllers-3d` for its player
and FPS Kit for its level content. Keep the declarations in Git.
Blank and UI starters add no player controls.
See the [2D](../guide/2d.html), [first-person](../guide/fps.html) and
[isometric](../guide/isometric.html) guides for complete game adapters.

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
compile with the game's engine bindings. Generated files stay in `.karty/`; package authors
distribute sources rather than a game's generated bindings.

An explicitly selected build processor can also return Go adapters for the
game's engine bindings. These compile under `.karty/processed/code/<package.processor>/`;
the package owns their source and templates. This lets a controller provide
typed camera/body integration without adding gameplay policy to the engine.
See [package-generated Go adapters v1](package-generated-code-v1.html).

Only selected package textures enter a world's asset build. Transparent decals
use the `sprite` profile and retain their alpha through the ordinary bounded,
checksum-verified package and image pipeline. They use existing fixed actor
sprites and add no package format, world schema or wire format changes.

The package workflow supports local folders, local archives, named catalog
imports and checksum-pinned HTTPS archives. Automatic updates and transitive
resource dependencies remain outside its current scope. Particle, audio and
video exports are also not supported yet.

Environment texture exports may include a `[textures.materialize]` table to
retain a material's generation preset and overrides in every consuming level.
For example, a metal bumper can export `preset = "metal"` together with its PNG
and prefab. Settings are validated for all exports and staged only for selected
textures; sprites and decals cannot declare them. See the
[package material generation v1 decision](package-materials-v1.html) for the
fields, compatibility and build rules.

Follow [Build and share](../guide/share.html#share-reusable-code-prefabs-and-ui)
to import or export one. Read [Asset pipeline](asset-pipeline.html) to see where
its content goes during a build.

## Dialogue package v1

The published `dialogue` 0.0.1 package combines the Yarn compiler, portable Go
story runtime, presentation model and visual novel UI in one install and one
archive. Last Train demonstrates this integration. Its optional `savegame`
package supplies generic profile/run storage independently of dialogue.

Game code imports `github.com/karty-game/dialogue`. `Load` reads official
compiled Yarn programs and their string table; `NewStory(program)` creates a
story runner, while `New()` creates presentation state. The package supplies
`NovelFrame`, `NovelDialogue`, choices and history to the game's generated UI.
Games own artwork, fonts, scenes and custom commands, and may use their own UI.

The game explicitly selects `dialogue.compile` and its `.yarn` input paths in
`[[build.processors]]`. Authoring requires the .NET tools and Yarn Spinner compiler
described in the package's setup instructions; the shipped game uses a pure Go runtime. Initial
Yarn feature limits are documented in the package's compatibility reference.

The [binary runtime contract](https://github.com/karty-game/karty-packages/blob/main/packages/dialogue/docs/binary-runtime-v2.md)
describes compiled dialogue resources, string tables and saved runs. Follow the
package's migration instructions when updating an existing story project.

The package includes an optional `dialogue.Stage` controller for authored
entrances, framing, short reactions and ambient motion. It exposes bounded,
viewport-relative snapshots to a game's renderer and needs no engine change.
Games save authored composition alongside the story; animation phases remain
transient, and reduced motion belongs with lifetime preferences. Last Train
demonstrates sprite-based desktop staging and responsive portrait UI. Continuous
environmental motion and gesture movement currently use the desktop renderer;
the narrow-screen UI supports expressions, group composition and entrances.

## Tiled package v1

The published `tiled` 0.0.1 package shares a build-time Tiled JSON parser, PNG
importer, bounded binary map codec and 2D sprite view. Read the
[package reference](https://github.com/karty-game/karty-packages/blob/main/packages/tiled/README.md)
and its two independent consumers:
[Willowbrook](https://github.com/karty-game/karty/blob/main/samples/willowbrook/README.md)
uses TinyGo; [Tiled room](https://github.com/karty-game/karty/blob/main/testdata/tiled-room/README.md)
uses Go.

The samples already pin this release. From another game's directory, import it:

```sh
karty package import tiled@0.0.1
karty package restore
```

To edit the package itself, clone the packages repository and explicitly switch
the consuming game to its working folder:

```sh
karty package import --replace /path/to/karty-packages/packages/tiled
```

Both consumers call the shared image importer from their own author generator,
which writes ordinary level texture inputs. From the repository root, run
`mise run author-willowbrook` followed by `mise run build-willowbrook-web`, or
`mise run author-tiled-room` followed by `mise run build-tiled-room-web`.
Follow each sample's setup instructions before building.

Game code imports `github.com/karty-game/tiled` and its `karty2d` subpackage.
The game supplies a small create/update/destroy SpriteRenderer2D backend using its own
generated bindings and mounted textures. The shared view handles bounded
residency, authored animation and optional wind; the game owns its camera,
input, collision rules, actors and HUD. Static sprites stay in world coordinates
while the game's `Camera2D` follows the player.

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

See the [compiled Tiled map contract](https://github.com/karty-game/karty-packages/blob/main/packages/tiled/docs/binary-map-v1.md)
and [dialogue/save compatibility](https://github.com/karty-game/karty-packages/blob/main/packages/dialogue/docs/binary-runtime-v2.md).

Processors may declare [generated level textures](processor-textures-v1.md).
The CLI stages these under `.karty/` and packages them without writing generated
artwork into project source directories.

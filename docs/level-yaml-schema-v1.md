# World YAML editor schemas, version 1

Run `karty schema` from a project root to refresh YAML editor completions without
installing an SDK, reading images, baking lighting, or compiling game code.
`karty schema --check` also validates the authored world files. Builds refresh
and validate the same schemas before preparing the SDK or processing assets.

The CLI generates these ignored files:

- `.karty/schemas/worldsource.base.schema.json`: the common world grammar for
  source versions 1–6, derived from the public SDK field vocabulary.
- `.karty/schemas/levels/<directory>.schema.json`: a level schema referencing
  that base, with material and sprite texture names from its `level.toml` and
  prefab names from its world declarations.

Add this first line to `levels/showcase/world.yaml`:

```yaml
# $schema: ../../.karty/schemas/levels/showcase.schema.json
version: 6
```

Use the level directory name, which can differ from `[level].name`. For a world
file in a deeper directory, adjust the relative path from that YAML file to the
project's `.karty/schemas` directory. IntelliJ and YAML language-server editors,
including the YAML extension in VS Code, use this header for field diagnostics
and value completion. See [IntelliJ's local schema documentation](https://www.jetbrains.com/help/idea/yaml.html#json_schemas).
See the [language server's modeline documentation](https://github.com/redhat-developer/yaml-language-server#using-a-modeline)
for editor integration. The generated schemas use JSON Schema 2020-12.
All schema references are local, so validation works offline after generation.
Refresh after adding or renaming textures or prefab declarations; `karty dev`
refreshes them through its normal rebuilds. Generated `.karty` files are excluded
from the dev watcher to prevent rebuild loops.

World materials and sprite textures resolve through the level's texture catalog;
unrelated project textures are not valid world materials. Prefab placeholders
are allowed when declared as a material override's `from`. Override targets at
the root must be level textures; nested targets can be declared placeholders.
The compiler still checks final material resolution after nested expansion.

Schema checks reject unknown fields, wrong scalar types (including quoted
numbers), unsupported versions, invalid enum values, declared per-list bounds,
and unknown texture/material or prefab references. Defaults and nullable legacy
lists remain compatible with authored source versions 1–6. Version-6 `solids`
and root/prefab `contents` must be sequences when present. The parser rejects
duplicate or non-string keys, anchors, aliases, multiple documents, nonfinite
numbers, custom tags, files over 8 MiB, and nesting beyond 64 levels.

Schema validation complements the existing compiler checks for geometry,
identifier byte limits, duplicate identities, room/edge/port scope, cycles,
aggregate and expanded bounds, and SDK capabilities. Build validation uses the
CLI's generated schema in memory, so editing a generated JSON file cannot bypass
these checks. Schema generation does not modify authored YAML.

Without a `levels` directory, `karty schema` emits just the base file. This is
useful for standalone authoring fixtures whose headers reference the base.

## Validation evidence

Checked on 2026-10-06 with the pinned repository tasks:

- CLI: `mise run test`, `mise run build`, `mise run lint`, `mise run smoke`, and
  `mise run check-dev` passed. Go tests use a 9-second package timeout. Crafted
  schema tests took 0.26 seconds; level-build tests took 0.95 seconds; the real
  Air watcher check took 4.64 seconds, including ignored generated schema writes.
  The isolated camera component fixture also passed against installed SDK 0.0.9
  in 0.76 seconds after updating its copied HUD filename to `hud.go`.
- SDK: `mise run test`, `mise run build`, and `mise run check-fmt` passed after
  adding the canonical fixture's YAML schema directive. The fixture remains
  JSON-compatible beneath the directive and adds no YAML parser dependency.

Schema/build unit fixtures contain just the authored fields they need, with no
sample assets or baker. The fail-fast fixture selects an unavailable SDK and
missing image sources, proving that YAML checks precede those dependencies.
The migrated project YAML files also passed `karty schema --check` without image
processing. All four existing world-YAML directives resolve to local schema files.

CLI checks ran with `GOWORK=off` against the already prepared public SDK 0.0.9
and KartUI 0.0.5 module candidates. Their public publication remains a separate
release step. The smoke workflow compiles TinyGo WASM, validates the native
cartridge and stages its web build; it does not execute gameplay or browser
graphics. Smoke needed execution outside the sandbox for Xvfb startup, with
`XDG_CACHE_HOME=/tmp/karty-tinygo-xdg-cache` and Go caches under `/tmp`.
Editor schema support was checked against IntelliJ and the YAML language server's
documented modeline and JSON Schema 2020-12 support; interactive IDE feedback was not manually
reviewed. No browser graphics or additional runtime execution was required for
this CLI authoring change.

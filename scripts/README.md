# Contributor scripts

Run scripts through their pinned mise tasks from the repository root. Use
`mise tasks` to browse the full task list. Browser checks use Node and Playwright;
the CLI itself is written in Go.

Task definitions are grouped in [`tasks/`](../tasks/); sample tasks live in
[`tasks/samples/`](../tasks/samples/). Keep everyday build, test and formatting
commands in the root `mise.toml`.

## Integration checks

| Task                          | Purpose                                                                            |
| ----------------------------- | ---------------------------------------------------------------------------------- |
| `smoke`                       | Build and execute a temporary project with installed public SDK and host artifacts |
| `check-integration`           | Run the customer integration checks                                                |
| `check-materialize-software`  | Check cold and warm native atlas generation with software graphics                 |
| `check-video`                 | Exercise packaged media playback in Chromium                                       |
| `check-local-packages`        | Check linked package edits and portable archive builds                             |
| `check-package-ui`            | Check shared UI packages in independent Go and TinyGo games                        |
| `check-tiled-package-web`     | Exercise the Tiled package in an independent browser game                          |
| `check-tiled-package-archive` | Build a Tiled package consumer from an exported archive                            |
| `check-controller-starters`   | Compile the 2D, first-person and isometric starters with Go and TinyGo             |

`check-package-controllers.mjs` is called by `check-local-packages.mjs`.
Sample-specific tasks and prerequisites are documented in the
[sample READMEs](../samples/README.md). Checks write reports and captures under
`dist/validation/`. Compilation, WASM execution and browser rendering are
separate checks; record which ones passed.

## Asset and template generation

Keep generators with the assets or templates they own. The Roman court regression
fixture uses `cmd/roman-court-materials` and `cmd/roman-court-fixtures`; the
outdoor lighting fixture uses `scripts/outdoor-lighting-fixture.go`. Last Train's Foley and score
have separate generation tasks; score rendering requires Python, FluidSynth
and a soundfont. See its [audio sources](../samples/last-train/assets/audio/README.md).
Terrain's retained authoring tools and artwork sources live in
[`terrain/`](terrain/README.md). Vigil's selected source artwork and model workflow
live in [`vigil/`](vigil/assets/README.md); Willowbrook's retained artwork and
prompts live in [`willowbrook/assets/`](willowbrook/assets/README.md). These asset-generation tools are not needed to
compile the CLI.

`export-controllers-starter.mjs` copies the public controller package into a
starter directory, including its license and contract:

```sh
mise run export-controllers-starter -- /path/to/template/packages/controllers
```

Add `--check` to verify an existing copy without rewriting it. SDK template
revision 0.0.4 includes a copy in each of its three genre starters. The
`check-controller-starters` task requires the released SDK 0.0.13,
compiler and web host.

Website generation, browser checks and sample capture tools live in
[`website/`](../website/README.md).

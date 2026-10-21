# Dialogue for Karty

One package for authored Yarn dialogue, portable story execution and a ready-made
visual novel interface. Install `dialogue` once; it includes the compiler, runtime,
presentation model, choices, history and responsive layout. No separate `yarn`
or `novel-ui` package is required.

## Add it to your game

From your game directory:

```sh
karty package import /path/to/karty-cli/packages/dialogue
```

An exported `dialogue.kpkg` contains the same complete package and can be
imported instead. Select the compiler and your authored scripts in `karty.toml`:

```toml
[[build.processors]]
use = "dialogue.compile"
inputs = ["dialogue/*.yarn"]
```

Write normal Yarn Spinner scripts in `dialogue/`. The build machine needs Go
and .NET SDK **9.0.203**; the bundled compiler pins Yarn Spinner **3.2.1** with
locked NuGet dependencies. The first restore needs NuGet access. The game itself
uses a pure Go runtime, compatible with Go WASI and TinyGo, with no .NET runtime.
Processors run only when explicitly selected by the game.

## Run and display dialogue

Game code uses one Go import, `github.com/karty-game/dialogue`:

```go
// processed is your game's generated .karty/processed package.
compiled, _ := processed.Read("dialogue.compile", "story.yarnc")
strings, _ := processed.Read("dialogue.compile", "strings.bin")
program, err := dialogue.Load(compiled, strings)
if err != nil { /* report the script error */ }
story := dialogue.NewStory(program)
view := dialogue.New()
err = story.Start("Start")
```

`story.Continue()` yields lines, choices, commands or completion; your game
controls its scenes and handles those events. `view` holds the displayed line,
progressive reveal, choices and history. The package automatically supplies
`NovelFrame`, `NovelDialogue`/`NovelDialogueProps`, and
`NovelHistory`/`NovelHistoryProps` to the game's generated KartUI bindings.
No separate UI package or engine-specific bindings are distributed. The packaged
UI requires SDK 0.0.17 and its matching host for full-layout character reveal
(`UI.Reveal`), plus a CLI compiler using KartUI v0.0.8 or newer. The portable
runtime and model remain independent of engine bindings.

Supply your game's artwork, fonts and command handlers. Use the full interface
or just the runtime/model with your own UI. Profile/run storage is independent;
the optional `savegame` package remains useful to games of any genre.

[Last Train](../../samples/last-train/README.md) is a complete example using
`dialogue` plus optional save storage. Detailed references:

- [Yarn compatibility, runtime events and compiler requirements](docs/yarn-compatibility.md)
- [Presentation, reveal and checkpoints](docs/presentation.md)
- [UI components, slots and font roles](docs/ui.md)
- [Stage motion, entrances, framing and reduced motion](docs/stage.md)

This is an independent integration of a documented subset of real Yarn Spinner,
not the official runtime. Unsupported saliency/node groups, smart variables and
other initial limits are stated in the compatibility reference.

From the CLI repository root, run `mise run generate-yarn-fixtures` to regenerate
upstream-compiled fixtures and `mise run test-dialogue-packages` for portable
runtime/presentation/save tests. `mise run check-last-train-packages` validates
clean archive consumers with both Go and TinyGo.

See the [binary runtime v2 decision](docs/binary-runtime-v2.md) for resource layout and candidate save compatibility.

# Yarn dialogue for Karty

Portable Go execution of a documented subset of **real Yarn Spinner 3.2.1**
compiled programs. The package contains no generated game-engine imports and
works with both Go WASI and TinyGo. It is an independent integration, not an
official Yarn Spinner runtime.

From your game's directory, import this folder with
`karty package import /path/to/karty-cli/packages/dialogue` (or import an exported
`.kpkg`). In the consumer's `karty.toml`, enable its compiler:

```toml
[[build.processors]]
use = "dialogue.compile"
inputs = ["dialogue/*.yarn"]
```

The build machine needs Go and **.NET SDK 9.0.203** on PATH. From this repository,
`mise exec dotnet@9.0.203 -- ...` selects that exact SDK. The compiler pins
`YarnSpinner.Compiler` **3.2.1**, with a locked NuGet dependency graph. The first
restore needs access to NuGet; later builds can reuse its cache. Builds execute
the selected processor with ordinary developer tool access.

The processor emits `story.yarnc` and `strings.bin` into the game's owned
`.karty/processed` package. Compilation errors fail the build before replacing
the previous resources. The compiler runs in private staging, and sends all
build diagnostics to stderr so stdout remains the KPR1 binary envelope.

```go
compiled, _ := processed.Read("dialogue.compile", "story.yarnc")
strings, _ := processed.Read("dialogue.compile", "strings.bin")
program, err := dialogue.Load(compiled, strings)
if err != nil { /* show an error */ }
story := dialogue.NewStory(program)
err = story.Start("Start")
event, err := story.Continue()
// event.Kind: "line", "choices", "command", "complete"
// Present lines, dispatch your own commands, and call story.Choose(choice.ID).
```

`Continue` yields one event. Choices include an `Enabled` flag: preserve their
IDs and never select a disabled choice. Call `Choose(-1)` only when every option
is unavailable. `Save` and `Restore` serialize VM position, variables, pending
choices, stack, detours and visit counters. Restoring validates the compiled
program fingerprint and replaces state only on success. Persist that snapshot
alongside the game's presentation state. Register application functions with
`Register`; custom commands are returned to the game unchanged.

Initial compatibility limits are deliberate:

- Supported: ordinary nodes, lines and `#line:` IDs, shortcuts/conditional
  choices, bool/finite-float32/string variables, substitutions, arithmetic/comparison,
  logical operators, jumps, detours/returns, commands and visit tracking.
- Built-ins: `visited`, `visited_count`, conversions `string`/`number`/`bool`,
  `format_invariant`, `round`, `floor`, `ceil`, `int`, `decimal`, `inc`, `dec`,
  `min`, `max`, and the corresponding scalar operators in `functions.go`.
- Yarn language version 3, compiler 3.2.1 is the qualified combination. Yarn 2
  and saliency/node-group instructions fail during loading. Smart variables
  fail when evaluated; implement additional functions explicitly with `Register`.
- The bundled compiler exposes upstream standard-library signatures. Custom
  function-signature configuration is not provided yet; `Register` can execute
  custom functions in programs compiled elsewhere with matching signatures.
- No markup interpretation, localization selection, random-function library,
  voiceover sequencing, Unity integration or automatic save migration. Text is
  returned literally; a game owns those presentation features. Compiler string
  metadata is exposed as `Line.Tags`.
- Limits: 4 MiB each for program/string table, 1,024 nodes/variables/stack
  entries, 65,536 instructions, 128 options, 64 detours and 2,048 instructions
  per `Continue`. Presented strings are at most 4,096 UTF-8 bytes and snapshots
  at most 48 KiB. The shared dialogue view presents at most 32 choices.

The checked-in compatibility fixture is compiled by the actual pinned
compiler, not a hand-assembled imitation. Run repository-root tasks
`mise run generate-yarn-fixtures` and `mise run test-dialogue-packages`.

The upstream compiler and format are maintained by
[Yarn Spinner](https://github.com/YarnSpinnerTool/YarnSpinner). NuGet dependency
licenses remain with their respective authors; no upstream binaries are
vendored in the package archive.

See the [binary runtime v2 decision](binary-runtime-v2.md) for resource layout and candidate save compatibility.

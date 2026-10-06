# Client hooks and authored actions

The SDK 0.0.9 candidate provides typed game hooks, accepted movement updates,
authored actor references and bounded action sequences. Use its matching bundle
and host. The CLI default retains its released SDK pin; all four samples now explicitly
use this candidate and typed hooks.
The exact runtime API lives in the selected `.karty/docs/client-hooks.md`.

Declare `//karty:action stable.name` or `//karty:condition stable.name` above a
top-level game function whose first argument is `engine.ActionContext`. Actions
return error; conditions return (bool, error). At most 16 additional named
arguments use bool, string, int32, uint32, float32 or engine.WorldActorRef.
Identifiers survive function renames. Methods, generics, platform/build
constraints and unsupported signatures fail the build.

Author `levels/<directory>/actions.json` using version 1, named sequences and
steps with action/condition identifiers, typed args, then/else/onFailure branches
and positive waitFrames. Actor literals name exact compiled contents within
that level, including prefab provenance; stale runtime IDs are never authored.
Build validation uses the compiled actor inventory, catches argument/target
errors and packages the source as level data `karty/actions@1`.

The CLI emits `.karty/actions/catalog.json`, `.karty/actions/schema.json` and
`.karty/actions/levels/<directory>.schema.json`. Set the authored JSON's `$schema`
to `../../.karty/actions/levels/<directory>.schema.json` for standard JSON editor
completion of action names, argument types and that level's actors. The schema
comes from the selected public SDK bundle. Generated invocation code is staged
outside src; its diagnostics refer to authored sequence paths.

After building, call generated `StartAuthoredSequence(handle, levelName, name)`
from a level/input/UI handler. Repetition is ignore, restart or parallel; the
returned ID supports CancelSequence and SequenceRunning. Level release and
shutdown cancel owned runs. A missing runtime target invokes failure handling
before any game action receives that target.

The [main world-camera sample](../samples/world-camera/src/main.go) now uses
SDK 0.0.9 hooks with camera, marker, HUD and spinner behavior components.
Its marker and HUD react to accepted transform callbacks, while the frame hook
only advances continuous movement and animation.
From this repository root, `mise run check-client-hooks` requires installed SDK
0.0.9 and matching KARTY_HOST_NATIVE/CHECK inputs. Optional
KARTY_GO/TINYGO/WASM_TOOLS overrides select already available tools. It builds
the [dedicated fixture](../cmd/karty-check/testdata/client-hooks/README.md) twice
for native, compares distributions and executes actual WASM. It checks authored
condition/wait/action, fresh actor IDs after remount, typed input and accepted
transform callbacks. It loads no sample, UI, material tool or lightmap bake.
The camera is hidden; graphics pixel acceptance belongs to the engine's small
native GPU fixtures. `check-client-hooks-web` explicitly adds Chromium and
requires KARTY_HOST_WEB. Its browser execution has a nine-second watchdog and
kills its owned browser on cleanup. Build/install time is separate.

Candidate bundles use project generator version 2. Older CLIs report that an
upgrade is needed; existing version 1 bundles remain supported. Set
`KARTY_CHECK_KEEP_PROJECT=1` when running the fixture check to retain its isolated
project for inspection. Nothing is committed or published by that check.

`mise run check-client-hooks-native` is an explicit alias for the default
check; it requires only KARTY_HOST_NATIVE and
KARTY_HOST_CHECK. Browser graphics acceptance remains a separate check.

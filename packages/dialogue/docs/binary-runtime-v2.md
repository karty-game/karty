# Binary dialogue resources and saves v2

Decision: 2026-10-18, for the unreleased `dialogue` 0.1.0 candidate. Yarn source
and the pinned official compiler remain unchanged. The package emits binary
string tables and writes binary checkpoints, so cartridges need no CSV, JSON
or general protobuf runtime. The existing handwritten reader continues to
consume official `.yarnc` bytecode.

The `dialogue.compile` processor now emits `story.yarnc` and `strings.bin`.
`Load(compiled, table)` retains its Go signature but requires the binary table.
The processor and game must be rebuilt together. Its KPR1 transport, compiler
version, engine bindings and SDK selection do not change.

## Records

Each record begins with a four-byte magic/version and a little-endian u32
payload length. Counts and UTF-8 byte lengths use u32; booleans use exactly 0
or 1. Integers have explicit bounds, and variable numbers use finite float32.
Unknown tags, invalid UTF-8, excessive lengths/counts, incomplete input and
trailing bytes are rejected. Readers scan the complete structure before
allocating decoded collections and copied strings.

- **KST1:** sorted unique string IDs, presented text and counted metadata tags.
  IDs/tags are at most 256 bytes, text 4096, tags 128 per line and rows 65536.
  The whole record remains limited to 4 MiB. The pinned C# compiler and Go
  encoder emit identical canonical table bytes.
- **DLS2 / DLR2:** full presentation or run-only presentation. Seven strings,
  completion/audio flags, choices and history retain the model's existing
  bounds. Run records omit lifetime preferences and transient status. Run
  restore also accepts binary full records while preserving current audio
  preferences. The entire record is at most 48 KiB.
- **YSS2:** compiled-program fingerprint and instruction position; completion
  flags; sorted variable values; stack values; detour frames; choices and tags;
  sorted visit counters. Values use explicit string/number/bool tags. Existing
  VM limits, the 2399 data-record aggregate bound and 48 KiB record limit remain.
  Semantic restore validates the complete state against the selected program
  before replacing the live story.

## Compatibility

These candidate records replace the earlier CSV string tables and CSV story/
presentation checkpoints. Old candidate run saves are rejected explicitly;
there is no legacy CSV or JSON decoder in the cartridge. Start a new run after
upgrading this candidate. Rejected loads leave the current state and stored
bytes intact. A save migration would belong in a separate offline tool.

Last Train's opaque LTN2 checkpoint envelope remains unchanged. Its direction
payload now uses **LTC1**, a fixed nine-byte record: magic, framing enum,
ensemble flag and one expression enum for each of its three characters.
Unknown enums, flags and lengths are rejected. An absent direction payload
retains the established default composition behavior.

Lifetime profiles retain their existing six-byte LTP1/LTP2 contracts, so audio
preferences, reduced motion and ending unlocks survive the run-format change.
The savegame package and engine continue to store opaque bytes.

Coverage includes official compiler fixtures, deterministic tables and saves,
round trips, exact bounds, all truncations, invalid tags/versions/counts,
rejected legacy data and transactional restore. Go/TinyGo builds and actual
WASM/browser behavior are qualified separately.

# Hosted sample runtime v1

The sample website keeps its complete output below 200 MiB, including all ten
approved samples and the documentation. Website publication shares one verified
WebGPU runtime for each SDK version instead of copying it into every game.
Standalone game builds retain their complete `host-bundle@1` distributions.

## Build contract

The website builder verifies every complete source bundle, including both
byte-identical `karty-host.wasm` and `runtime.bin` files, before staging shared
files. It copies unchanged runtime modules, Go glue, texture codec, WebAssembly
and required licenses to `runtime/SDK/INVENTORY_SHA256/`.

The separate `karty.hosted-runtime@1` representation contains a bounded,
checksum-named `hosted-runtime.json` inventory. Its `bundleSha256` records the
verified source descriptor. Its `files` table preserves each selected file's
size and SHA-256. The hosted representation omits only `runtime.bin`, the
optional byte-identical network fallback alias. `karty-host.wasm` remains the
primary runtime URL. No source bundle or immutable SDK release is modified.

Publication rejects incomplete inventories, changed file bytes, unexpected
files, links, invalid paths and multiple different runtimes for one SDK version.
The existing aggregate 200 MiB and 10,000-entry limits remain enforced.

## Browser delivery

The website shell loads the shared `wasm_exec.js` and `bootstrap.mjs` URLs.
Existing module-relative imports and fetches resolve the rest of the runtime
from that same directory. Each game's cartridge and level content stay beneath
its own directory. No asset transformation, decoding or additional rendering
work is introduced.

The builder regenerates each game's existing release manifest and pointer.
Shared runtime files remain startup entries with their original sizes and
hashes; the modified shell receives its new size and hash. Existing service
worker rules permit shared startup URLs while retaining confinement for
background game content. Only the selected release and its referenced content
are staged; original build distributions retain their earlier generations.

Documentation generation shares static assets at the site root. Paths are
rewritten during generation; browser image and document loading is unchanged.
Published samples require the complete verified WebGPU bundle.

`karty-samples compact` creates a new immutable website snapshot from existing
complete sample builds, generates its documentation and checks the same limits.
It installs no SDK, runs no compiler or baker, and preserves its inputs. This
local candidate command does not establish public SDK artifact availability.
Supply `compact --notices DIRECTORY` to include explicit candidate
`RUNTIME_LICENSE.md` and `SDK_LICENSE.md` inputs beside every game. Both must be
bounded regular files. These notices count toward the same total website limit;
their candidate provenance must be recorded separately from public release
availability. Normal publication downloads the notices from the public SDK
release.

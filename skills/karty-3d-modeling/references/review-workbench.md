# Review workbench and local resume point

Use the general approach in any Karty project. The commands/paths in the second
section are this workspace's existing implementation, not SDK-wide requirements.

## Hardware and review invariants

Use installed Playwright/Chromium and existing project helpers when available.
On this Linux AMD workspace the proven launch configuration is:

```text
--enable-gpu --ignore-gpu-blocklist --disable-software-rasterizer
--use-gl=angle --use-angle=gl-egl
EGL_PLATFORM=surfaceless
LIBGL_ALWAYS_SOFTWARE=0
```

The existing launcher also uses `--no-sandbox` for this container; that is a local
container configuration, not a general recommendation for other environments.
Use a normal sandbox where supported. Display/socket/device denials need the
appropriate execution access; they are not a reason to switch to a software GPU.

Read CDP `SystemInfo.getInfo`: require accelerated WebGL and a nonempty renderer;
reject renderer identities containing SwiftShader, llvmpipe, softpipe or software.
Record the actual GPU, viewport, world scale and quality settings. The observed
renderer here is AMD RENOIR through EGL/ANGLE. Desktop headless capture and synthetic
input are not evidence of mobile-device FPS, audio or human playability.

One browser/context should cover the whole family. Keep one active prefab/camera,
select a variant, then loop angles. For each angle, loop lighting presets in lit
output, restore neutral state, then loop diagnostic modes once. Four lights plus
eight diagnostics mean 12 captures per angle, not 4 × 9. Skip a channel only when
unsupported or irrelevant, and document the coverage accurately.

For keyboard automation, let both press and release reach accepted cartridge
frames. Reusing a camera does not prove teleport succeeded: assert the accepted
position, output enum and raw material mode. Use host Debug → Settings → Material
view for raw normal/AO/height/metallic/smoothness where the selected host exposes
those channels. Don't invent cartridge camera-output enum values for them.

Use an immutable in-memory/directory copy of the **whole** production distribution
for final runs, including content modules and runtime glue. Fetch host/cartridge
from that same server, record SHA-256 and verify snapshot ETags when supported.
Hash before dev resumes: dev can overwrite `dist/web` with a different compiler's
cartridge. A post-run hash of regenerated files is not capture provenance.

Runtime switching needs no rebake. CPU geometry processing and documented CPU
lightmap baking are separate from the prohibited use of a software renderer.
Bake only when the baked assembly/lighting is what needs acceptance.

## Other demonstrated workflow failures

| Failure                                                               | Correction that preserves the intended review                                                                                                                                                        |
| --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Named view selects but FPS camera stops at an intervening wall        | Inspect the accepted controller/body pose. Use free inspection for appearance; keep the tested native controller lifecycle for collision checks. Do not assume setting a transform bypasses physics. |
| Consecutive G presses produce mislabeled normal/depth images          | Wait for both press and release to reach accepted frames and assert the actual output mode.                                                                                                          |
| Reducing the review to three images makes it fast but misses failures | Keep the full relevant matrix and share GPU context/scene instead. Deduplicate light-independent diagnostics, not hidden-side coverage.                                                              |
| Dev changes files or displays rebuild banners during capture          | Freeze source or use an immutable whole-distribution snapshot, then restore dev. Record the tested artifact before regeneration.                                                                     |

## Existing dungeon-kit implementation

Project: `/home/kef/projects/karty-cli/samples/dungeon-kit`.
Maintained helpers under its `tooling/`:

- `generate.py`: quick/full scene profiles, stable prefab/catalog exports, metrics.
- `masonry.py`, `pointed.py`, `windows.py`: shared geometric construction.
- `review-browser.mjs`: hardware-only Chromium and GPU verification.
- `review-web.mjs`: family-filtered angle/light/diagnostic capture and HTML gallery.
- `mesh-cost.go`: public compiled-level cost inspection.
- `check-web.mjs`: full baked AR04 collision/traversal review; quick dispatches
  to the appearance matrix. Quick appearance review is not capsule acceptance.

Use the project's existing environment, if still available:

```sh
export KARTY_HOME=/tmp/karty-saved-probes-final-cli-home
export GOCACHE=/tmp/karty-engine-go-cache
export GOLANGCI_LINT_CACHE=/tmp/karty-engine-lint-cache
export GOMAXPROCS=2
```

SDK 0.0.20 currently selects Go 1.27.3 / TinyGo 0.43.0. Re-read the installed
manifest before a later session; do not turn these observations into upgrade pins.
From the project root:

```sh
python3 tooling/generate.py --profile quick --family ar08
../../dist/karty dev --port 33593
node tooling/review-web.mjs http://127.0.0.1:33593 --family ar08
python3 tooling/generate.py --family ar08 --check
../../dist/karty schema --check
../../dist/karty fmt --check
../../dist/karty lint
```

Use pinned mise tools when needed (`mise exec -- ../../dist/karty ...`). Keep an
existing dev process instead of starting another. Before a production build,
pause only the owned dev process. `../../dist/karty build --target web` uses the
configured compiler; the current local candidate has additionally used the matching
host override `/home/kef/projects/karty-engine/dist/host/web/karty-host.wasm`.
This prebuilt override is local; do not rebuild or require private engine source.
Then `node tooling/review-web.mjs --family ar08` reviews a production snapshot.
Restore dev only after the snapshot/provenance has been pinned (simplest: after
review completes). From the CLI root, inspect mesh costs with:

```sh
mise exec -- go run samples/dungeon-kit/tooling/mesh-cost.go
```

Gallery output is `dist/review/<family>/` (AR04 uses `dist/review/quick/`).
Shared local review copies and original concepts live in
the CLI's ignored `tmp/dungeon-kit/`. Do not overwrite old evidence or embed those
local concept files in builds. Validation history is CLI `docs/validation.md`.

## Resume context recorded 2026-10-25

The user resumed asset work and explicitly authorized parallel agents. The current
batch is AR12 timber, DR01 oak doors and OB01 oak chest, all derived from the
existing approved scriptorium/kit artwork. AR12 is the current completed review
family; use `--family ar12` for its generator and capture commands. Select the
family explicitly rather than relying on an old default or repeating earlier work.
Source completion and successful GPU checks do not constitute user acceptance.

- AR04 round arches: 1.6 / 2.4 / 3.2 m; 1,332 / 1,692 / 1,836 triangles.
- AR05 pointed portal: 2.4 m; 2,064 triangles. Same limestone and full bearings.
- AR06 slit / barred service opening: 408 / 588 triangles including wall/backing;
  34 / 49 solids. Recessed mortar, real splay, supported lintel/sill, embedded
  grille. Last complete production review: 193 captures in 48.6 seconds on AMD.
- AR07 single / stacked burial recess candidates: 180 / 312 source triangles.
- AR08 round / pointed vault candidates: 4,308 / 5,400 source triangles.
- AR12 timber candidates: four reusable components (post, beam, brace, corbel)
  and two static supported proof assemblies, 204 / 228 renderer triangles.
  Production GPU review passed 193 captures in 42.951 seconds on AMD; user review
  remains pending. Shared longitudinal oak grain follows the actual members.
- DR01 door candidates: single and paired oak leaves/frames, with open/closed
  static construction proofs. A retracting-bolt correction and final GPU review
  are pending; do not claim runtime interaction or animation is implemented.
- OB01 chest candidate: hollow oak box, separate lid, physical hinges, straps and
  latch; 1,008 / 1,584 source triangles for closed/open static proof poses.
  Final GPU review is pending. Runtime interaction and animation are not implemented.
- Stairs and landings (retired AR09), raised floors and gallery/bridge walking
  surfaces (AR10) are sector-based level geometry, excluded from prefab authoring.
  Build their connected floor/ceiling heights in the level-design workflow and
  validate actual traversal there. Relevant trims, supports and railings remain
  distinct kit uses, with matching material scale at the joins.
- Check the current catalog, inventory and validation record before choosing
  another family; do not infer acceptance from this resume note.

Preview last used `https://dev.kefniark.dev/proxy/33593/`. Numeric specimen keys
now depend on the selected quick family; X/V cycle previous/next. Review galleries
use `https://dev.kefniark.dev/proxy/33591/vision/`, including `ar12/`, `dr01/`
and `ob01/`; the last two remain pending until their exports and evidence exist.
Earlier `ar04/quick/`, `ar05/`, `ar06/`, `ar07/` and `ar08/` are historical.
Preserve listeners and check health before claiming they are running. Use the
workspace's authenticated port proxy.

Reference inventory/art: CLI `tmp/dungeon-kit/vision/asset-inventory.json` and
`tmp/dungeon-kit/concepts/`. Current kit exports:
`levels/workshop/kit/` contains reusable prefabs, family catalogs and the active
review catalog. Preserve approved geometry/materials and review each family
against the original art. Parallel family work requires explicit authorization,
as provided for this batch. Remaining common polish: edges and wear are
cleaner/sharper than concept art. No full dungeon assembly, new baked collision
acceptance or package publication has been claimed.

Measured capture examples: AR04 289 images ~64 s, AR05 97 images ~30 s, AR06
193 images ~49 s, AR12 193 images 42.951 s. Production builds ~5–8 s with warmed
caches. These measure build/capture work, not total authoring or human review.
Recent families needed correction passes; do not claim every family was completed
in five minutes. Hardware-only rendering and lighting-independent diagnostic
coverage described above remain mandatory.

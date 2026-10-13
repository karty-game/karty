# Sample development and publishing

The [sample READMEs](../samples/README.md) introduce the playable demos.
This guide covers contributor checks, source structure and Pages maintenance.
Samples and tests share `CurrentSDK` in
[internal/release/version.go](../internal/release/version.go). Run
`mise run prepare-release` after changing that selection.

## Focused validation

From the CLI root, install the published bundle and run the source/CPU checks:

```sh
./dist/karty sdk install "$(./dist/karty sdk current)"
mise run check-sample-clients
mise run check-world-camera-hooks
```

`check-sample-clients` compiles actual sample clients against their pinned SDK
bindings and all `.kui` components/layouts. All managed samples select the current
published SDK, which must be installed before these checks. A small flight fixture dispatches
generated widget callbacks and checks gameplay effects, preferences,
pause/remount and idle frames that emit no commands. World-camera component
fixtures cover movement, accepted host poses and UI refresh.

These checks read source, themes and metadata only. They do not load sample
textures, media, geometry, bakes, graphical hosts or browsers. Native transport
is captured; this is Go compilation and CPU execution, not WASM or graphics
evidence. Each test process has a nine-second timeout; compilation/setup is separate.

The ordinary `mise run test` checks sample pins, compiles UI sources and runs
compatible world-camera component fixtures. The explicit tasks also check the
selected SDK client bindings. Full sample builds and browser/manual review are
separate diagnostics, not prerequisites for these behavior tests.

## Orbital flight deck

The interactive flight deck uses KartUI `.kui` single-file components with Go
setup blocks and indented styles. Start with the
[console](../samples/ui-demo/ui/views/game-screen.kui),
[inventory](../samples/ui-demo/ui/views/inventory.kui),
[window layout](../samples/ui-demo/ui/layouts/window-layout.kui) and
[game](../samples/ui-demo/src/main.go).

The game owns tab selection and flight preferences, which survive pause/resume
and destination changes. Input editing uses host text-focus capture. UI clicks
do not open the pause screen. Components update through callbacks/invalidation;
the frame hook advances the ship without re-projecting idle UI text.

Each keyed inventory row owns a use counter. Reordering preserves it;
closing/reopening Inventory resets row state while game-owned potion quantity
survives. Layouts provide named slots and scoped styles, and screens share a
theme. The levels also package static `.kui` HUDs; the flight console uses a
generated client component.

`check-sample-clients` exercises the current flight screen. The optional
`check-ui` browser task uses the older dedicated compatibility fixture.
Manual graphics, keyboard navigation and touch review remain separate.

## Other sample sources

- [World camera notes](sample-world-camera.md): authored geometry, materials,
  lighting, component hooks, camera configuration and explicit graphics checks.
- [Media lab notes](sample-media-lab.md): source formats, clip generation and
  optional browser playback checks.
- [Particle lab](../samples/particle-lab/README.md): reusable fire and smoke
  recipes, reference sources, touch controls and isolated review rooms.
- [Physics lab](../samples/crate-room/README.md): box colliders, pushing, shooting,
  explosions and sloped banks.

## Local docs gallery

`mise run docs` refreshes the guide and gallery, then serves them on
`http://127.0.0.1:8081/`. The local gallery includes Hangar, Physics lab, Environment, World camera, UI,
Media and Particle lab, in that order, with screenshots and uses complete snapshots of their `dist/web` builds when available.
Build samples with their pinned public SDK and matching host before opening
their playable cards. A checkout without these builds shows setup instructions instead;
docs generation needs no private engine checkout or unreleased SDK download.
`mise run preview-docs` refreshes the files without starting another server.

## Published demos and pull request previews

`mise run build-samples` builds all seven demos into `dist/samples-site/`, with
screenshot cards, the game-making guide and redistribution notices. It installs each sample's exact published
SDK and downloads the compatible host and Go browser runtime. No private engine
access is required. SDK/host artifacts must be published before advancing this
public build to a new version.

The local default includes a web host in each sample. Run
`mise run build-samples -- --cdn` to share one host per SDK version. The samples
CI workflow uses this shared mode for published artifacts.
The builder verifies the signed release manifest and host checksum, copies the
host to `runtime/VERSION/SHA256.wasm`, then gives each web build its relative
URL. Relative URLs keep the same layout in `main` and pull request previews.

**Build samples** runs on `main` pushes and pull requests, including forks, and
uploads a `sample-site` artifact. It also supports manual dispatch.
**Publish samples** updates GitHub Pages:

- `/main/`: latest successful default-branch samples.
- `/pr/NUMBER/`: the pull request's merge-commit build.
- `/`: latest samples and active preview links.

Each published sample has its own folder: `hangar/`, `crate-room/`,
`environment/`, `world-camera/`, `ui-demo/`, `media-lab/` and `particle-lab/`.
The published and local galleries use the same sample list and priority order.
Each demo includes its own SDK-pinned web host unless the build
selects the shared CDN option.
Deployment summaries include the actual preview URL; PR build
summaries link to the expected URL. Closing/merging a PR removes its preview;
subsequent deployments also prune closed previews. Failed builds preserve the
last successful preview, and superseded builds are skipped.

### Sample cards and screenshots

Each published sample has `preview.toml` with its title, a short description of
what it teaches, and screenshot alt text. Its `preview.jpg` is a 960 by 540 JPEG
under 512 KiB. The gallery copies that image beside the web build and links to
the sample's GitHub folder. New samples need both files before site building.

Build the changed samples, then refresh screenshots from the CLI root:

```sh
mise run capture-samples
mise run check-samples
```

The capture task opens existing `dist/web` builds in Chromium. It binds a free
loopback port and closes its server and browser when it finishes. It captures
Orbital's menu, the Roman courtyard, Hangar's reactor hall, Environment's daylight courtyard and the playing media
test video, plus Particle Lab's medium fire. It hides the optional
runtime debug controls for the image. Capture one changed sample with
`mise exec -- node cmd/karty-samples/capture.mjs particle-lab`.

Review each image before keeping it. `preview.capture.json` records the source
commit and source hash, cartridge and host hashes, image hash, browser version
and page errors. Source hashes include tracked and untracked files while excluding
ignored build output, README text and preview files. These records identify the captured artifacts. The task does
not rebuild them or prove that an old artifact matches the current source.
For WASM provenance, the local server hashes the bytes it serves and the browser
confirms successful response completion. The report records each loaded WASM
path, checksum and byte count; it does not depend on Chromium retaining large
response bodies in its inspector cache.
It does not test physical audio, native rendering or real mobile hardware.

The guide generator writes `guide/index.html` and `guide/guide.md` into the site.
Publishing validates both files and keeps the existing path, file-count and
200 MiB limits. A guide alone does not count as a playable sample site.
The shared `runtime/` folder also does not count as a sample. Publishing checks
its version folders, WASM headers and checksum-named files before copying it.

### One-time repository setup

1. Merge the workflows into `main`; GitHub triggers `workflow_run` publishers
   only after they exist on the default branch.
2. Set **Settings → Pages → Build and deployment** to **GitHub Actions**.
3. Allow `github-pages` environment deployments from the default branch.
4. Allow Actions to create/update `gh-pages-state`, which preserves the combined
   site. It is generated state, not the Pages source or a normal PR target.
5. Push to `main` or manually run **Build samples** on `main` for the first site.

The usual base URL is `https://karty-game.github.io/karty/`. A custom domain may
change it; set repository variable `PAGES_BASE_URL` for PR build summaries.
Deployment summaries always use the actual Pages URL.

Fork builds use read-only permissions and may need GitHub contributor approval.
No PAT or engine secrets are needed. The privileged publisher runs trusted
default-branch code, validates static artifacts and never executes PR source or
downloaded JavaScript. Previews are public and share an origin; keep credentials
and authenticated application data out of the site.

Pages uses the regular WASM host, since this deployment does not configure
Brotli `Content-Encoding`. Update `CurrentSDK` through
[release preparation](releases.md#sdk-pins); preparation updates all sample pins,
and tests reject drift. The Go module version does not select the sample SDK.

See [repository tooling](development.md#repository-tooling) for artifact limits
and generated-state hook handling, and GitHub's
[concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
and [Pages deployment](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)
documentation for workflow configuration.

# Sample development and publishing

The [sample READMEs](../samples/README.md) introduce the playable demos.
This guide covers contributor checks, source structure and Pages maintenance.
All four samples pin the published SDK 0.0.9 / API 0.0.7. `SampleSDK` in
[internal/release/version.go](../internal/release/version.go) selects their baseline;
the CLI's default SDK is selected separately.

## Focused validation

From the CLI root, install the published bundle and run the source/CPU checks:

```sh
./dist/karty sdk install 0.0.9
mise run check-sample-clients
mise run check-world-camera-hooks
```

`check-sample-clients` compiles the actual sample clients against public SDK
bindings and all `.kui` components/layouts. A small flight fixture dispatches
generated widget callbacks and checks gameplay effects, preferences,
pause/remount and idle frames that emit no commands. World-camera component
fixtures cover movement, accepted host poses and UI refresh.

These checks read source, themes and metadata only. They do not load sample
textures, media, geometry, bakes, graphical hosts or browsers. Native transport
is captured; this is Go compilation and CPU execution, not WASM or graphics
evidence. Each test process has a nine-second timeout; compilation/setup is separate.

The ordinary `mise run test` checks sample pins, compiles UI sources and runs
compatible world-camera component fixtures. The explicit tasks also check the
SDK 0.0.9 client bindings. Full sample builds and browser/manual review are
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

## Published demos and pull request previews

`mise run build-samples` builds the three published demos into `dist/samples-site/`, with
an index and redistribution notices. It installs each sample's exact published
SDK and downloads the compatible host and Go browser runtime. No private engine
access is required. SDK/host artifacts must be published before advancing this
public build to a new version.

**Build samples** runs on `main` pushes and pull requests, including forks, and
uploads a `sample-site` artifact. It also supports manual dispatch.
**Publish samples** updates GitHub Pages:

- `/main/`: latest successful default-branch samples.
- `/pr/NUMBER/`: the pull request's merge-commit build.
- `/`: latest samples and active preview links.

Each sample has its own folder: `ui-demo/`, `media-lab/` or
`world-camera/`. Deployment summaries include the actual preview URL; PR build
summaries link to the expected URL. Closing/merging a PR removes its preview;
subsequent deployments also prune closed previews. Failed builds preserve the
last successful preview, and superseded builds are skipped.

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
Brotli `Content-Encoding`. Update `SampleSDK` through
[release preparation](releases.md#sdk-pins); preparation updates all sample pins,
and tests reject drift. The Go module version does not select the sample SDK.

See [repository tooling](development.md#repository-tooling) for artifact limits
and generated-state hook handling, and GitHub's
[concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
and [Pages deployment](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages)
documentation for workflow configuration.

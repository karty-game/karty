# Samples

All four samples use SDK **0.0.9 / API 0.0.7**, selected by `SampleSDK` in
[internal/release/version.go](../internal/release/version.go). They use typed
lifecycle/input hooks. Authored interfaces use `.kui` single-file components with
Go setup, templates and indented styles; UI demo showcases the current widgets.
The released CLI default remains separate from this candidate sample baseline.

SDK 0.0.9 is currently a candidate. Build the CLI, install a checksummed candidate
bundle and select its matching host:

```sh
mise run build
./dist/karty sdk install --archive /path/to/sdk-0.0.9.zip --sha256 CHECKSUM 0.0.9
cd samples/ui-demo
KARTY_HOST_WEB=/path/to/matching/karty-host.wasm ../../dist/karty dev
```

After publication, `./dist/karty sdk install 0.0.9` supplies the public bundle and
normal builds select its published host. Local host overrides do not install an
SDK or change project pins. Generated `.karty/` and `dist/` outputs remain derived.

- [UI demo](ui-demo/README.md): an Orbital flight deck with Mission, Loadout and
  Settings tabs, a ship combo, call-sign input, thruster slider, autopilot checkbox,
  tooltips, pause/navigation and a keyed inventory.
- [Pong](pong/README.md): sprites, vectors, keyboard/pointer input and independent
  level mount/release/remount.
- [Media lab](media-lab/README.md): image/audio conversion, overlapping sounds,
  streaming music and MPEG-1 video. Browser audio requires user interaction.
- [World camera](world-camera/README.md): portal worlds, camera controls, authored
  actors/actions and accepted-pose component hooks. Its UI includes view and
  G-buffer channel switching.

## Focused validation

From the CLI root with the candidate bundle installed:

```sh
KARTY_HOME=/path/to/candidate-sdk-cache mise run check-sample-clients
```

This compiles every actual sample client against public SDK bindings, compiles
all `.kui` components/layouts and checks source diagnostics. A small CPU flight
fixture dispatches real generated widget callbacks and checks gameplay effects,
persistent preferences, pause/remount and an idle console that emits no commands.
The world-camera component fixtures also run. Only source, theme and metadata
are read: sample textures, media, geometry, bakes and graphical hosts are not loaded.
Native transport is captured; this does not claim WASM or graphics execution.
Each test process has a nine-second timeout. Compilation/setup is separate.

The ordinary `mise run test` checks sample pins and compiles the UI sources without
requiring candidate SDK installation. The explicit task additionally checks all
candidate client bindings and executes the CPU behavior fixtures. Full sample
builds/previews are for running the examples, separate from this validation.

## Published demos and pull request previews

`mise run build-samples` builds all four checked-in samples into `dist/samples-site/`,
including an index page and redistribution notices. It installs each sample's
exact published SDK and downloads the compatible host and Go browser runtime.
It needs no private engine access. The selected candidate SDK/host must be published
before this public-only task can build the updated samples.

The **Build samples** workflow runs for `main` pushes and pull requests, including
forks, and uploads a `sample-site` artifact. It can also be dispatched manually.
The **Publish samples** workflow then updates GitHub Pages:

- `/main/`: latest default-branch samples.
- `/pr/NUMBER/`: samples built from that pull request's merge commit.
- `/`: links to the latest samples and active previews.

Each sample has its own subdirectory (`pong/`, `ui-demo/`, `media-lab/`, or
`world-camera/`). The deployment
summary includes the preview link, and the PR build summary links to its expected URL. Closing or merging a PR removes its preview;
each subsequent deployment also prunes closed previews. Failed builds leave the
last successful preview in place. Superseded builds are skipped.

### One-time repository setup

1. Merge these workflows and scripts into `main` first: GitHub only triggers
   `workflow_run` publishers once they exist on the default branch.
2. In **Settings → Pages → Build and deployment**, choose **GitHub Actions**.
3. Allow the `github-pages` environment to deploy from the default branch. The
   publisher always runs trusted default-branch code, even for fork previews.
4. Allow Actions to create/update the generated `gh-pages-state` branch. This
   branch stores the combined site so each deployment preserves other previews.
   It is not the Pages publishing source and should not receive normal PRs.
5. Push to `main` or manually run **Build samples** on `main` for the first site.

For `karty-game/karty`, the usual URLs are
`https://karty-game.github.io/karty/main/` and
`https://karty-game.github.io/karty/pr/NUMBER/`. A configured custom domain may
change the base URL; set repository variable `PAGES_BASE_URL` for the PR build
summary. The deployment summary always uses the actual Pages URL.

No PAT or engine secrets are needed. Fork builds use read-only permissions and
may require GitHub's contributor workflow approval. The privileged publisher
never runs PR source or downloaded JavaScript; it validates and copies static
files only. Previews are public and share the Pages origin, so this site should
contain demos only, with no credentials or authenticated application data.

Pages serves the regular WASM host. The Brotli artifact is not selected because
this deployment does not configure the required `Content-Encoding` response.
To update the SDK used by the demos, update `SampleSDK` and run the documented
[release preparation](../docs/releases.md#sdk-pins). Public previews also require
compatible published engine artifacts. Preparation updates all sample pins;
tests reject drift. The Go module version alone does not select the sample SDK.

The publisher queues deployments with GitHub's [concurrency queue](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
and uses the official [Pages Actions deployment flow](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).

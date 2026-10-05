# Samples

Stable samples follow `CurrentSDK` in [internal/release/version.go](../internal/release/version.go).
All four samples select released SDK 0.0.8, including world-camera for static solids.
Build the CLI and install that SDK before working on a sample:

```sh
mise run build
./dist/karty sdk install "$(./dist/karty sdk current)"
cd samples/pong
../../dist/karty build --target web
../../dist/karty dev
```

Use `samples/ui-demo` for KartUI composition and game menus. Generated `.karty`
and `dist` directories are derived and excluded from Git.

Use `samples/media-lab` for PNG/JPEG/WebP conversion, WAV/QOA conversion,
overlapping one-shot stress, a host-owned streaming QOA music loop with
crossfades, and MPEG-1 video with MP2 audio. Browser audio requires user interaction.
See [Assets](../docs/assets.md) for discovery, transforms and playback APIs.

Use `samples/world-camera` for the portal renderer, perspective/isometric camera
switching, authored world actors, sprite modes, tag queries, and the live camera
diagnostic panel. Its current input handlers do not expose G-buffer switching.

## Published demos and pull request previews

`mise run build-samples` builds all four checked-in samples into `dist/samples-site/`,
including an index page and redistribution notices. It installs each sample's
exact published SDK and downloads the compatible host and Go browser runtime.
It needs no private engine access; all selected SDKs are available from public releases.
The task pins Python for TOML parsing.

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
To update the SDK used by the demos, update `CurrentSDK` and run the documented
[release preparation](../docs/releases.md#one-sdk-pin) after compatible public
engine artifacts are available. Preparation updates all sample pins;
tests reject drift. The Go module version alone does not select the sample SDK.

The publisher queues deployments with GitHub's [concurrency queue](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
and uses the official [Pages Actions deployment flow](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).

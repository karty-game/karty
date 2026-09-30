# Samples

The checked-in samples pin SDK **0.0.6** in their `karty.toml`. Build the CLI and
install that published SDK before working on a sample:

```sh
mise run build
./dist/karty sdk install 0.0.6
cd samples/pong
../../dist/karty build --target web
../../dist/karty dev
```

Use `samples/ui-demo` for KartUI composition and game menus. Generated `.karty`
and `dist` directories are derived and excluded from Git.

Use `samples/media-lab` for PNG/JPEG/WebP conversion, WAV/QOA conversion,
timed music replay, and overlapping one-shot stress. Its BGM replay is not a
gapless loop because the current public audio API only exposes one-shot playback.

Use `samples/world-camera` for the portal renderer, perspective/isometric camera
switching, authored world actors, sprite modes, tag queries, and G-buffer views.

## Published asset pipeline

SDK 0.0.5 processes image sources as QOI and sound sources as QOA. Pages and
pull request previews consume its signed public artifacts. Sound discovery,
transforms, and `Game.PlaySound` are covered in the
[asset guide](../docs/assets.md).

## Published demos and pull request previews

`mise run build-samples` builds all four checked-in samples into `dist/samples-site/`,
including an index page and redistribution notices. It installs each sample's
exact published SDK and downloads the compatible host and Go browser runtime.
It needs no private engine access. The task pins Python for TOML parsing.

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
To update the SDK used by the demos, change both sample `karty.toml` pins after
publishing the corresponding engine artifacts; `go.mod` alone does not select it.

The publisher queues deployments with GitHub's [concurrency queue](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency)
and uses the official [Pages Actions deployment flow](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).

## Build and share

Use `--target web` for browsers and `--target native` for desktop games.
The supported game runtimes are:

| Game platform       | `--platform`    |
| ------------------- | --------------- |
| Browser             | Omit this flag  |
| Linux x64           | `linux-amd64`   |
| Linux ARM64         | `linux-arm64`   |
| macOS Apple silicon | `darwin-arm64`  |
| Windows x64         | `windows-amd64` |
| Windows ARM64       | `windows-arm64` |

Every supported development machine can package any listed game platform.
The CLI compiles one WASM cartridge and downloads the destination's
checksum-verified host. Players need neither the CLI nor TinyGo.

### Publish a browser game

From your project's folder:

```sh
karty build --target web
karty serve dist/web --addr 127.0.0.1:8080
```

Open `http://127.0.0.1:8080/` and test the built game. Upload the whole
`dist/web/` directory to a static host, including `content/` when present.
Use HTTP hosting; opening `index.html` directly is not the preview workflow.

### Build and run a native game

Build for your current machine, then launch it on Linux or macOS:

```sh
karty build --target native
cd dist/native
./karty-host --cartridge game.kart
```

On Windows, run `.\karty-host.exe --cartridge game.kart` in PowerShell from
`dist/native/`. The build summary prints the command for your machine.

To package another platform, run these from the project root:

```sh
karty build --target native --platform windows-arm64
karty build --target native --platform linux-arm64
karty build --target native --platform darwin-arm64
```

Explicit platforms use separate folders, such as
`dist/native/windows-arm64/`. On the destination Windows machine, open that
folder and run `.\karty-host.exe --cartridge game.kart`.

Keep a `LICENSES/` folder in your project containing Karty's runtime and library
notices from the [release downloads](https://github.com/karty-game/karty-sdk/releases/latest),
plus notices required by your own assets and dependencies. Commit this folder
with your source. Copy it into each distribution before sharing:

```sh
cp -R LICENSES dist/web/
cp -R LICENSES dist/native/
```

For a named native platform, use its output folder instead, such as
`cp -R LICENSES dist/native/windows-arm64/`.
Share the whole selected folder, including its `content/` and `LICENSES/` files.
Browser and desktop use the same source. Test each platform you ship.

### Share reusable code, prefabs and UI

To publish your own package, follow [Write and publish a package](packages.html).

Install published code and assets from the
[Plugins catalog](https://karty-game.github.io/karty-packages/) by name:

```sh
karty package import fps-kit@0.0.1
```

Omit the version to select the latest stable release. Commit the resolved
archive URL and SHA-256 in `karty.toml`; future builds restore that same pin.
Use `karty package import --replace fps-kit` for an explicit update.

For a package folder containing `karty-package.toml`, export a local archive:

```sh
karty package export --output placeholder-kit-0.1.0.kpkg ../placeholder-kit
```

From a consuming game, import the archive using the name in its package manifest:

```sh
karty package import placeholder-kit-0.1.0.kpkg
```

Commit the referenced archive and updated `karty.toml`/`go.mod`; the CLI restores
its checked contents under ignored `.karty/packages/`. To edit the package live,
use `karty package import --replace ../placeholder-kit` instead.
`karty dev` watches that working folder, including outside the game. The current
package CLI supports Go behavior, static/nested prefabs, PNG world
materials and KartUI components/layouts with images, fonts and theme defaults;
use a CLI build containing that feature. UI exports compile against the consuming
game and keep normal typed props/callbacks. The game owns state and lifecycle.

### Build in CI

Commit `karty.toml`, `go.mod`, source code, and source assets. Include `go.sum`
when your dependencies create it.
The runner recreates `.karty/` and `dist/`; do not commit those folders.

Save this as `.github/workflows/game.yml` in your game project. It builds a browser game and a Windows ARM64 distribution on Linux,
then uploads each folder for review. Include the `LICENSES/` folder described above
so the artifacts contain the required notices. The workflow installs the latest CLI.
No private engine checkout or credentials are needed.

```yaml
name: Build game
on: [push, pull_request]
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-24.04
    env:
      GOEXPERIMENT: simd
      GOWORK: "off"
    steps:
      - uses: actions/checkout@v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@v7.0.0
        with:
          go-version: "1.27.3"
          cache: false
      - name: Install Karty
        run: |
          go install github.com/karty-game/karty/cmd/karty@latest
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"
      - name: Prepare material generation
        run: |
          sudo apt-get update
          sudo apt-get install -y --no-install-recommends libegl1 libgl1-mesa-dri
      - name: Build distributions
        run: |
          karty build --target web
          karty build --target native --platform windows-arm64
          cp -R LICENSES dist/web/
          cp -R LICENSES dist/native/windows-arm64/
      - uses: actions/upload-artifact@v7.0.0
        with:
          name: game-web
          path: dist/web/
          if-no-files-found: error
      - uses: actions/upload-artifact@v7.0.0
        with:
          name: game-windows-arm64
          path: dist/native/windows-arm64/
          if-no-files-found: error
```

Change the native `--platform`, artifact name, and artifact path together.
For Linux or macOS artifacts, upload a tar archive of the folder to
[preserve the host's executable permission](https://github.com/actions/upload-artifact/tree/v7.0.0#permission-loss).
For offline lighting, add `karty render bake` before the builds. Material generation
uses Mesa software OpenGL when the runner has no GPU; the libraries above
provide its EGL backend. Packaging a game in CI does not test its graphics,
input, or sound on the destination platform.

Choose a sample to extend:

- [The Hangar](../hangar/): a portal loop, detailed materials, baked lights, and a torch.
- [Orbital flight deck](../ui-demo/): menus and reusable KartUI components.
- [Media lab](../media-lab/): sound, music, and video.

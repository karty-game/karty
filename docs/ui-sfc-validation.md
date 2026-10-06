# SFC-only UI validation

Current authoring uses `.kui` SFCs and indented styles. Samples, current references,
snippets and candidate starters put the template before the script. Released SDK
snapshots remain unchanged. UI scaffolding needs the candidate's 0.0.2 templates;
released game scaffolding remains supported. `mise run check-ui` selects SDK 0.0.9.

Checks from the corresponding repository roots, with Go cache in
`/tmp/kartui-go-cache`:

- KartUI: `GOWORK=off GOPROXY=off mise run check` passed tests, build and editor tests.
  `mise run lint` passed. `mise run package-editor` created extension 0.1.21.
- CLI: `KARTY_HOME=/tmp/karty-sfc-home KARTY_TEST_SDK=0.0.9 mise run test`
  passed, including SFC starter rendering, reachability and staging. Loopback HTTP
  fixtures required execution outside the filesystem/network sandbox.
  `mise run build` and `mise run lint` passed with the local public-module workspace.
- CLI: `mise run check-sample-clients` passed all sample UI compilation and native
  Go source fixtures, including flight-console and camera controls/lifecycle.
  These CPU fixtures are not WASM or graphics execution.
- CLI: `mise run check-ui` passed TinyGo compilation and actual browser WASM UI
  execution on desktop and high-DPI primary touch layouts. It used the locally
  built candidate web host, existing Chromium installation and an isolated SDK
  cache. No manual UI or VS Code Extension Development Host review was performed.
- Engine: `mise run test`, `mise run build` and `mise run bundle-sdk -- 0.0.9`
  passed. New coverage checks the candidate template pin and compiles all eleven
  UI starter sources without style warnings. Generated-host drift checks passed.

The isolated test SDK cache contains copies of installed released bundles plus
an engine-built candidate; it does not change the normal SDK cache. Existing tool
and Chromium installations were reused.

Standalone CLI build verification could not complete: its declared public
`karty-sdk v0.0.9` and `karty-ui v0.0.5` module ZIPs are absent from the local
module cache. A retry using a writable temporary module cache and the local
read-only download cache confirmed these missing files. Dependency pins were
preserved. No modules, SDKs, extension or other artifacts were published.

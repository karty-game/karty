# Contributing to Karty

Contributions to CLI commands, project scaffolding, builds, development mode, and samples are welcome. Small bug fixes,
documentation improvements, and reproducible bug reports are good starting points.
For a substantial feature or a compatibility change, open an issue first so we
can agree on scope before implementation.

## Local setup

Fork and clone this repository, install [mise](https://mise.jdx.dev/), then run
these commands from the repository root:

```sh
mise install
mise run fmt
mise run check-fmt
mise run test
mise run build
mise run lint
```

`mise install` also installs the repository hk pre-commit hook. Formatting and
lint use the same pinned hk configuration locally and in CI; see the
[development guide](docs/development.md) for tool ownership and exclusions.

Public development needs only the public KartUI and SDK modules. You do not need
private engine access or release secrets. See [development](docs/development.md)
for local `go.work` overrides while the initial module tags are being published.

For build, browser, or development-flow changes, run the relevant integration
checks described there. The full `mise run check-integration` suite consumes
published SDK/host artifacts; install browser dependencies with
`mise run install-browser` first. Linux native checks also need graphics libraries
and Xvfb. Record which checks you actually ran in your pull request.

## Source ownership

CLI behavior belongs in `internal/`; examples belong in `samples/`. Change shared
formats in `karty-sdk` and UI compilation in `karty-ui`. Protocol bindings arrive
in the selected SDK bundle. Do not patch generated project `.karty/` files or
commit `dist/` output. Keep SDK-versioned documentation aligned with behavior.

## Sending a pull request

Keep each pull request focused on one problem. Explain the user-visible result,
include a reproduction for a bug where possible, and list your validation results.
Add regression coverage when it protects changed behavior. Run `mise run fmt`
for Go changes and review the result before submitting. Preserve unrelated work.

Dependabot proposes weekly Go module and GitHub Actions updates, plus npm updates. Minor and patch
updates are grouped; major updates stay separate. Changes to pinned tools in
`mise.toml` and SDK-selected toolchains are reviewed manually.

## License

Contributions are made under this repository's [MIT license](LICENSE.md).
Only submit material you have the right to contribute, and retain existing
third-party copyright and license notices.

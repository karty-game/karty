# Placeholder kit and controller

A local package containing configurable FPS behavior, optional input collection,
static crate/stack prefabs and original checker/wood/frame PNGs.

Import this folder for development:

```sh
karty package import /path/to/library
```

Or export and import a fixed archive:

```sh
karty package export --output placeholder-kit-0.1.0.kpkg /path/to/library
karty package import placeholder-kit-0.1.0.kpkg
```

The commands run in the consumer project except export, which works anywhere.
`karty-package.toml` is the required package definition. Its `name = "placeholder-kit"`
determines the project dependency key, cache link and export namespace. This is
an ordinary example package, with no built-in standard-library treatment.

Use `placeholder-kit.crate` or `placeholder-kit.crate-stack` in ordinary world YAML instances. The
materials are `placeholder-kit.checker`, `placeholder-kit.wood` and `placeholder-kit.frame`; each instance can override
them with game-owned materials. Go imports retain their module path:
`github.com/karty-game/stdlib-spike/fps` and `/controls`. The library has no
project-specific engine imports; the consumer supplies a small body adapter and
owns camera, physics, input routing and lifecycle.

Original textures can be regenerated with `mise exec -- go run ./testdata/local-packages/generate.go` from the CLI repository
root. This package's source and original assets are distributed under the
included MIT license. The two consumers exercise folder edits and portable archive builds.

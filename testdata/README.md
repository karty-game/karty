# Shared test fixtures

These sources are required by the CLI package tests and integration checks.
They stay in Git; experimental resolvers, reports and build output do not.

- `roman-court/`: retained geometry, textures and chunk areas for world compilation regressions.
- `outdoor-lighting/`: retained daylight, sky and vegetation inputs for compilation and bake regressions.
- `local-packages/`: Go behavior, prefabs, original textures and two consumer games.
- `package-ui/`: a shared HUD package and consumer templates.
- `tiled-room/`: an independent consumer of the Tiled package, with authored maps.

Run `mise run test-local-packages` for the focused unit checks. Run
`mise run check-local-packages` and `mise run check-package-ui` for actual
Go/TinyGo builds and Chromium checks with an installed matching SDK and host.
Those checks copy fixtures into `dist/validation/` before editing them.

Regenerate the original checker textures from the repository root:

```sh
mise exec -- go run ./testdata/local-packages/generate.go
```

Investigation folders such as root `docs/`, `spikes/` and `concept-art/` are
ignored. Promote inputs needed by a project check here before using them.

The camera lifecycle/component fixture lives in `cmd/karty-check/testdata/camera-client/`.
These retained inputs are test assets, excluded from playable sample discovery.
Regenerate fixture artwork and geometry with `mise run generate-roman-court-materials`,
`mise run generate-roman-court-fixtures` and `mise run generate-outdoor-lighting-fixture`.

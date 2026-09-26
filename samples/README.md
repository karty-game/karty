# Samples

Pong and UI demo pin the supported migration SDK 0.0.1. Build the CLI first.

```sh
cd samples/pong
../../dist/karty build --target web --host /absolute/path/karty-host.wasm
../../dist/karty dev --host /absolute/path/karty-host.wasm
```

Use `samples/ui-demo` for KartUI composition and game menus. Generated `.karty`
and `dist` directories are derived and excluded. The embedded SDK provides
build inputs; running requires a compatible host artifact. Until the split's
first public SDK/host release, use locally built candidate hosts.

# Karty design

Karty brings Go game code, authored levels and a shared runtime together.
These pages explain the choices behind that structure and what they mean when
you build a game.

- [Why Karty?](why.html) describes the workflow I want and the games it serves.
- [Architecture](architecture.html) explains the host, client and level, and how they work together.
- [SDK versions and toolchains](sdk-and-toolchain.html) explains project pins, compilers and compatible hosts.
- [Asset pipeline](asset-pipeline.html) follows source files through a build to the running game.
- [Packages](packages.html) explains how games share code, objects, materials and UI.
- [Processor textures](processor-textures-v1.html) specifies generated level textures and their build-time bounds.

To start making something, choose a [game guide](../guide/index.html).
For exact APIs and supported features, read your project's `.karty/docs/`,
which matches the SDK pinned in `karty.toml`.

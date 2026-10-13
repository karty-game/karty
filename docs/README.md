# CLI documentation

Choose a path below. Repository guides describe current CLI behavior; a project's
`.karty/docs/` contains the API and authoring reference for its exact SDK.

## Make and share a game

- [Get started](../README.md#start-a-game): install the CLI, create a project and preview it.
- [Game authoring guide](https://karty-game.github.io/karty/main/guide/): choose 2D, first-person or isometric; UI, media and distribution apply to every path.
- [Samples](../samples/README.md): source examples and playable previews.
- [Assets](assets.md): images, audio, video, generated world materials, mapping and authored lights.
- [Static solids](static-solids.md): source-v6 geometry and room-free detail prefabs.
- [Environments](environments.md): procedural skies, directional sunlight and sky boundaries.
- [GPU sprite particles](particles.md): YAML emission, lifecycle, movement and sample effects.
- [Lightmaps](lightmaps.md): receiver coordinates, runtime recipes, strict imports and offline `karty bake`.
- [Chunked levels](chunk-levels.md): build budgets, child meshes/lightmaps, staging and portal visibility.
- [Native screenshots](screenshot.md): capture a level at an explicit camera pose.
- [Distribution](distribution.md): supported platforms and required packaged files.

## Understand project contracts

- [Implemented behavior](implemented.md): current capabilities and SDK selection.
- [World YAML schemas](level-yaml-schema-v1.md): editor completions, material catalogs and validation.
- [Client hooks and authored actions](client-hooks.md): typed catalogs, editor schemas and invocation adapters.
- [UI sources](ui-source-discovery-v1.md): `.kui` discovery, staging and packaging.
- [SDK bundles](sdk-bundles.md): installation, trust and compatibility.
- [Bake storage v1](bake-storage-v1.md): project cache location and per-level lighting ownership.
- [Complete static bake decision](decisions/static-light-bake-v1.md): source selection and count bounds.
- [Incremental bake decision](decisions/bounded-incremental-bake-v1.md): transport distance and content-addressed pixel reuse.
- [Shared web runtime decision](decisions/web-cdn-v1.md): verified hosts and browser delivery.

## Contribute to the CLI

- [Development](development.md): pinned checks, overrides and repository tooling.
- [Sample development](sample-development.md): focused checks, source notes and Pages publishing.
- [Guide generation](guide-generator.md): edit, generate and preview the game authoring guide.
- [Repository ownership](repository-split.md): generators, formats and licenses.
- [Releases](releases.md): SDK selection, candidate samples and publication gates.
- [Proposals](proposals.md): unresolved work and acceptance requirements.

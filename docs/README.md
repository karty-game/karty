# CLI documentation

Start with [implemented behavior](implemented.md). Project-local `.karty/docs`
contains the API/authoring reference matched to the project's exact SDK;
repository guides describe current CLI behavior.

## Authoring and distribution

- [World YAML schemas](level-yaml-schema-v1.md): local editor completions, level material catalogs and build validation.
- [Client hooks and authored actions](client-hooks.md): SDK 0.0.9 candidate catalogs, editor schemas and typed adapters.
- [UI sources](ui-source-discovery-v1.md): `.kui` discovery, staging and packaging.
- [Assets](assets.md): images/audio/video, generated world materials, mapping and authored lights.
- [Static solids](static-solids.md): source-v6 geometry and room-free detail prefabs.
- [Lightmaps](lightmaps.md): receiver coordinates, runtime recipes, strict imports and offline `karty bake`.
- [Bake storage v1](bake-storage-v1.md): project cache location and per-level lighting ownership.
- [Distribution](distribution.md): native/browser targets and required packaged files.
- [Native screenshots](screenshot.md): capture a level at an explicit camera pose and return the PNG path.
- [Samples](../samples/README.md): examples and Pages previews.

## Contributor workflows

- [Development](development.md): pinned checks, overrides and sample-site tooling.
- [Sample development](sample-development.md): focused checks, source notes and Pages publishing.
- [Repository ownership](repository-split.md): generators, formats and licenses.
- [SDK bundles](sdk-bundles.md): installation, trust and compatibility.
- [Releases](releases.md): SDK selection and publication gates.
- [Proposals](proposals.md): unfinished work and contract decisions.

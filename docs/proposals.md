# Proposals

Future work, not supported settings or a release schedule.
See [Implemented behavior](implemented.md) for the current inventory.

## CLI/build

- **Optimized/Brotli web hosts:** add explicit artifact selection and delivery
  with correct HTTP encoding; current builds and Pages use the regular host.
- **Image transforms/import:** cropping, sprite atlases and streaming/tiled imports
  need bounded allocation and deterministic, versioned processing recipes.
  World material atlases and mip chains are already supported.
- **Resize coordinate mapping:** preserve logical UI sizes and sprite regions
  through an explicit consumer contract rather than silently changing layout.
- **Cache retention:** automatic size-based eviction; manual cache deletion
  already works. Conservative static sound stripping needs reference analysis.

## Shared contracts and acceptance

- **Media controls:** video seek/pause/loop/completion, audio seeking and
  level-scoped audio lifetimes need engine/SDK API decisions.
- **Video import:** transcoding additional authoring formats needs an explicit
  CLI tool/dependency policy and bounded output contract.
- **Regression budgets:** establish representative fixtures and numerical
  build/startup/memory thresholds; codec choice alone is not a speed guarantee.

Existing configuration and media limits remain in [Assets](assets.md).

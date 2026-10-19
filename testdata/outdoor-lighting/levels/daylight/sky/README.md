# Authored sky sources

These original PNGs and generation prompts moved unchanged from the engine's
`host/internal/forwardgpu/skydomeassets`. The sample owns the artwork and its
presentation settings; the host owns sky rendering. The daylight world references
clouds-sparse, clouds-dense and mountains. City and space are retained source
variants and are not packaged unless referenced by a world.

The reusable CLI processor `internal/assetpipeline/world_sky.go` replaces the
old handwritten `skydomeassets/generate.go`: PNG sRGB is converted to linear
premultiplied RGBA8 and compressed with deterministic gzip, preserving dimensions,
alpha, composition and edges. Rebuild the sample to compile edited PNGs.
Edit `scripts/outdoor-lighting-fixture.go` to change its authored paths or settings,
then run `mise run generate-outdoor-lighting-fixture` from the CLI root.

Original source SHA-256:

| PNG               | SHA-256                                                          |
| ----------------- | ---------------------------------------------------------------- |
| city.png          | ae9bb09d05bbc4c17bcd22c68c67a07ef421cd0f4cad1c69b392c1a6caf72a90 |
| clouds-dense.png  | 90eed4bce15c00c017260dc855dc29fe28bda0a5cf0c0600933a4cf87e20bbd6 |
| clouds-sparse.png | 5efc0def5ef200aa4db849d8d6500cb85315a56b496f7485aad2cfba6458042b |
| mountains.png     | b66de483ec5b8c5dd605b1b3e06cb6d1dd66a667b69009130ed2b761a3159073 |
| space.png         | 84bfc128597c7d6fd65fab7dc8ad917535ce6bb70ff7804fd28485e9e462e35c |

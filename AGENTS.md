# Contributor instructions

Read README.md and CONTRIBUTING.md for source ownership and migration
limits. Preserve unrelated changes. Do not commit or publish automatically.
Run workflows from this repository root using pinned mise tasks. `mise run test`
and `mise run build` are the minimum validation for module changes.

Keep handwritten generators and templates in Git. Do not edit generated host
bindings by hand; update the owning generator and regenerate. Project .karty/
and dist/ outputs are ignored. Leave local .codex/ and .agents/ directories alone.
Keep protocol bounds, complete-batch validation, generated-file ownership,
path confinement and allocation constraints intact. Public API/wire changes
need a versioned decision. Public language/SDK changes must update their pinned
documentation and coverage tests. Distinguish compilation, actual WASM
execution, browser graphics, and manual review in the handoff.

Keep investigation notes, spikes and concept art local in ignored folders.
Publish documentation in website/content/ and retain required test fixtures in
testdata/. Do not make builds or tests depend on ignored investigation files.

The engine owns core/api, core/internal/codegen and host. KartUI owns compiler,
schema and codegen project adapters. The CLI owns project building. The public SDK owns cartridge/level
format packages. Public repositories must remain buildable without private
source or credentials. Record exact checks, outcomes and remaining limits.

## GPU rendering

### Build-time processing and runtime approval

- Any new processing during asset loading or rendering must be justified and
  explicitly approved by the user before implementation. Describe the exact
  work, why the build cannot perform it, and its CPU, memory and latency cost.
  Moving work from rendering to loading does not bypass this requirement.
  Prior explicit approvals remain valid only within their agreed scope.
- The build must produce GPU-ready asset payloads, including channel packing,
  texture resizing, mip generation, RNM packing and geometry conversion.
  Do not shift these operations into loading or rendering.
- KLD chunk selection and lookahead are driven by portal traversal, including
  transformed cameras and apertures through non-Euclidean portals. Coordinate
  proximity does not imply adjacency or reachability. Spatial bounds partition
  and cull geometry within each traversed space; unrelated overlapping spaces
  retain distinct ownership. Do not substitute a whole-world texture array or
  global preload for this streaming. Chunk loading must not freeze presentation.

Use hardware GPU rendering for all visual review, screenshots and graphics
validation. Software renderers (including SwiftShader, llvmpipe and softpipe)
are prohibited; do not offer a software mode or silently fall back. Verify and
record the actual renderer and accelerated WebGL/OpenGL status. If hardware
acceleration is unavailable, stop the graphics check and fix access/configuration
or report the blocker. This workspace has an AMD GPU; Linux headless browser
reviews use EGL/ANGLE (`--use-gl=angle --use-angle=gl-egl`,
`EGL_PLATFORM=surfaceless`, `LIBGL_ALWAYS_SOFTWARE=0`) with
`--enable-gpu --ignore-gpu-blocklist --disable-software-rasterizer`.

## 3D asset authoring

For reusable 3D prefab/material modeling and visual review, use
[the Karty 3D modeling skill](skills/karty-3d-modeling/SKILL.md). Its optional
workbench reference records the existing dungeon-kit workflow and pause point.

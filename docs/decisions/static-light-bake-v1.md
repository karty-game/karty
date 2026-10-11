# Complete static lighting bake, version 1

Status: candidate SDK 0.0.11, 2026-10-11. Released SDK contracts remain immutable.

An enabled `[lightmap]` with `offline = true` and no `light` or `lights` selector
selects every fixed authored point source without a nonzero linked material
emission pulse. Selection follows authored order. Explicit directional lists
retain partial bake behavior; layout-only builds retain their existing meaning.
The single `light` visibility recipe cannot request offline radiosity.

Authoring and offline transport accept up to 4,096 sources. The live renderer
retains its independent limit of 50 lights. Final packaging counts the actual
completed bake recipes and rejects a remainder over that bound. A missing or
stale development bake reports a clear error when live fallback exceeds 50;
sources are never silently dropped. `karty bake` itself accepts the larger
authored set so it can produce the required completed artifact.

Chunk catalogs use the SDK's `KCC2` version-2 binary, with sorted unique authored
indices for the union of completed child bake sources. A producer adds a source
only after its child mesh and complete coefficient pair have been generated.
Cache reuse checks each child recipe against the catalog selection filtered to
its ordinary physical component, in addition to existing size, SHA-256,
passive-WASM, receiver and QOI validation. Mesh and shadow formats remain
unchanged. The private chunk cache revision is 2.

The coefficient atlas still contains three RNM tiles at the selected receiver
resolution. More sources increase offline work and bounded identity metadata;
they do not require additional lightmap tiles or runtime static light loops.
Runtime payloads remain typed binary and QOI. Source manifests remain TOML/YAML.

Coverage includes a real 1,000-source bake and packaged passive-WASM execution,
automatic selection of 24 steady sources while excluding moving/pulsing
sources, chunk cache invalidation after editing the last selected source,
catalog/child selection substitution, zero eligible sources, and the separate
authored and live count bounds.

# Chunked world levels

Chunk packaging is an opt-in capability, `world/chunks@1`. Select an SDK and host
that advertise that capability. Existing levels retain their current packaging.

Add budgets to a world's `level.toml`:

```toml
[chunks]
enabled = true
max_sectors = 256
max_mesh_bytes = 2097152
max_encoded_bytes = 8388608
max_lightmap_texels = 262144
```

These values are the defaults. `max_mesh_bytes` counts decoded vertex, index and
triangle metadata arrays; `max_encoded_bytes` covers the complete child KLD,
including baked images and its Wasm wrapper. All budgets must be positive.
`max_sectors` defaults to the current world's 256-sector topology bound, so mesh
and lightmap budgets drive grouping; lower it only for an explicit sector cap.
`max_lightmap_texels` must accommodate the configured maximum page (512 square
by default). With `page_size = 1024`, each chunk first tries a 512 square page
and uses 1024 only when packing requires it. The current lightmap contract
permits one 512 or 1024 square page per chunk; it never reduces requested texel
density to make geometry fit. Legacy whole-world builds keep their fixed page
size.

The builder grows connected sector groups deterministically. Ordinary reciprocal
connections may join a group; transformed portal edges do not join groups. An
ordinary path can still connect their endpoints. A sector that cannot fit alone
fails the build with its identity and budget error.

Optional authored areas keep meaningful regions separate:

```toml
[chunks]
enabled = true
areas = [
  { name = "courtyard", instances = ["roman-court"] },
  { name = "hall", rooms = ["hall"] },
  { name = "galleries", instances = ["gallery-a", "gallery-b"] },
]
```

Instance selectors match the exact expanded instance path and its descendants.
Room selectors match the qualified authored identity, such as `hall` or
`gallery-a/room`, including every convex sector produced from that room. Unknown
or empty selectors, empty areas, duplicate names and selections overlapping
different areas fail the build. Unselected sectors share a default group, so
partial area hints are useful. Separate areas never join one chunk; disconnected
regions and budget limits may split an area into several chunks. Omitting areas
retains automatic budget grouping. Areas affect the build only: all mesh and
lighting compilation still use the full original world.

The world-camera sample selects six areas. Its 61 courtyard sectors stay together
in a 1024 page at the existing density of 16 texels per unit; the hall, three
galleries and combined corner/ascent each use a 512 page at that same density.

The root level retains the entire sector and portal graph, authored actors,
processed textures and shared material atlas. Its catalog binds each child to a
chunk ID, geometry digest, content hash, exact byte length and sector ownership.
Children contain prebuilt mesh arrays and, when enabled, a baked lightmap. They
are render resources of one logical level: the game does not mount them as
additional levels or create duplicate actors.

For offline lighting, keep the existing lightmap controls:

```toml
[lightmap]
enabled = true
offline = true
# All eligible steady sources are selected automatically.
density = 4
page_size = 512
bake_samples = 16
bake_bounces = 1
bake_denoise = "medium"
# Optional: bound each lighting transport segment to enable local rebakes.
# bake_max_ray_distance = 16
```

A normal build produces missing child receiver atlases against their complete ordinary
physical component, including geometry belonging to neighboring chunks. Imported
whole-world prebakes cannot be split. Chunk builds require `offline = true` when
lightmaps are enabled and include the complete prebake pair; they never fall back
to a runtime bake. `karty bake` directs chunked levels to `karty build`; the
whole-world bake cache is not consumed by chunk builds. Mesh-only chunk levels
need no lightmap section. SDK 0.0.12 catalogs record the union of
completed baked source indices, so static sources are removed from the runtime
point-light table before loading a child. Each child recipe must match its
physical component's catalog selection. Moving and pulsing sources remain live.

`karty dev` runs `karty build --development`. It reuses a complete matching
prebuilt chunk set from `.karty/bakes/<level-directory>/`. When a baked set is
missing, stale or corrupt, dev builds mesh-only chunks and keeps live lighting;
it never traces lightmap rays. A missing bake that would leave more than 50
live authored lights reports an error instead of omitting lights. Preview meshes use a separate cache and cannot
overwrite a completed bake. An ordinary `karty build` produces missing baked
chunks and also reuses matching cached results. World and lighting data, source
texture bytes, chunk/lightmap settings and the selected SDK bind cache identity.
Every cached child must exist and match its catalog's byte length and SHA-256.
Cache reuse also decodes the canonical passive envelope without executing Wasm
and validates child metadata, mesh ownership and bounds, receiver charts and the
complete prebake pair. Invalid children make the entire cached set unavailable.
Roots cannot declare whole-world lightmap or prebake markers; the builder owns
the `@world/chunks/` and `@world/lightmaps/` entry namespaces.
Topology, collision, actors and shared atlases remain in the root in both modes.
The current forward host uses live lighting without a prebake; it has no runtime
lightmap baker. Levels without chunking still reuse matching whole-world bakes.

Chunk cache containers are bounded binary level envelopes; children keep their
existing passive KLD format. Publication happens only after all children are
complete. Caches are generated output, with no JSON index or new runtime format.
The dev watcher excludes `.karty/`, so restart dev or edit a watched source after
producing a new prebuild. The level builder can produce preview assets without
compiling game code, while the existing `karty dev` command still performs a
full cartridge rebuild.

Offline builds also keep independent, immutable pixel entries under
`.karty/bakes/<level-directory>/pixels/<input-sha256>.bin`. When a complete
chunk set becomes stale, the SDK hashes each receiver's current packing,
lighting controls, geometry, material UVs and decoded albedo dependencies.
Matching entries reuse complete checksummed QOI pixels; the builder then emits
a fresh layout, full-document manifest, mesh and catalog. Cache hits do not
rewrite pixel entries. Missing, incomplete, corrupt or escaping entries trigger
a cold bake. The build reports baked and reused chunk counts.

With an explicit positive `bake_max_ray_distance`, dependencies cover the
receiver bounds expanded by `(bake_bounces + 1) * distance`, including conservative
rounding guards. Sources are included when their clipped influence reaches the
receiver's bounce region. Far edits can therefore reuse unaffected receiver
pixels; long walls crossing the dependency region, local edits, added or removed
walls and moved lights change the relevant keys. Packing or range changes also
invalidate pixels. Without a distance, dependencies retain the complete ordinary
physical component, preserving the existing unbounded result.

Reuse depends only on current content, not Git diffs, modification times or a
previous checkout. CI can restore a cache from hundreds of commits ago and
reuse any matching historical input, including a later revert. Entries contain
typed binary metadata and QOI, never JSON or PNG. This is a build accelerator;
complete artifact integrity and development's no-bake behavior remain enforced.
See [bounded incremental chunk bake v1](decisions/bounded-incremental-bake-v1.md).

For CI, restore and save the project's `.karty/bakes/` directory, including each
level's `pixels/` entries. A broad restore key may return old matching inputs;
correctness comes from the current SDK input digest and complete validation,
not the CI cache label. A cache that predates pixel records can still supply an
exact complete artifact hit. Its first changed build needs a cold bake for any
missing pixel entries, then later builds can reuse those records. Restoring
only `dist/` or a chunk catalog does not provide an incremental pixel cache.

Web and native distributions stage roots and children together at
`content/<sha256>.kld`. Child filenames derive solely from their lowercase hash.
The browser verifies the streamed size, SHA-256 and bounded passive KLD ABI before
returning the decoded envelope to the host. Cancellation suppresses late or
replaced requests. The host checks the descriptor against the mounted root
catalog and manages residency separately from portal view instances: one chunk
asset may appear through several transforms and apertures.

The first catalog records directed portal reachability per sector as a
conservative visibility set. It can contain every chunk in a connected world;
it is not an aperture-based potential visibility solver. Runtime portal
traversal refines those candidates before requesting and rendering assets. This
version preserves the current logical world bounds, including the 256-sector
limit. Horizontal upward liquid faces are tessellated during building, keeping
wave samples and receiver UVs consistent across chunk boundaries. The host
animates those prebuilt vertices without runtime tessellation.

New roots contain a shared offline opaque caster proxy at
`@world/chunks/shadow`, schema `karty.world-chunk-shadow@1`, bounded to 8192
triangles. It preserves offscreen wall/solid shadows independently of render
chunk residency. Shadows retain the existing single connected ordinary-component
constraint; transformed or one-way shadow topology is rejected. Root proxies
are separate bounded resident assets alongside shared atlases.
When opaque geometry exceeds the caster bound, the builder omits the proxy;
that level must keep shadows disabled. Invalid source geometry remains a build
error.

CLI checks execute the passive parent and child Wasm ABI, validate receiver-scoped
prebakes, compare deterministic artifacts, and exercise bounded browser fetches,
cancellation and native/web staging. These checks do not establish browser
graphics parity or continuous streaming on a device.

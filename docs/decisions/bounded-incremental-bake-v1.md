# Bounded incremental chunk bake, version 1

Status: introduced in SDK 0.0.11 and supported in SDK 0.0.12. Owners: public SDK baker and CLI cache.
Decision date: 2026-10-11. Current reference: [lightmaps](../lightmaps.md).

The optional TOML `bake_max_ray_distance` is finite, positive and no greater
than the authored coordinate bound, 1,000,000 world units. It limits each
lighting transport segment. Absence remains unbounded and selects the existing
version-2 producer. Positive distances select version 3 and include the exact
distance in the generated typed binary prebake manifest. Imported and automatic
artifacts must match the configured distance; a valid artifact with a different
distance is stale.

The SDK owns the conservative influence region and pixel dependency digest.
Receiver charts/packing, actual participating polygons, material assignments,
UV projections, original decoded albedo, locally selected sources, RGBM range,
sampling, bounces, seed, denoising and producer identity enter the digest.
Geometry dependencies expand receiver bounds by `(bounces + 1) * distance`,
with guards for ray origins and raster rounding. Local source inclusion uses
the clipped source radius and the receiver bounce region. Unbounded output
keeps complete ordinary physical-component dependencies. Transformed portals
retain their existing transport restriction.

The CLI stores one bounded level envelope per exact SDK input digest under
`.karty/bakes/<level-directory>/pixels/`. It contains typed metadata and one
complete linear RGBA QOI stream, with a SHA-256 checksum. Reads check confinement,
entry completeness, schema, digest, producer, range, dimensions and the complete
QOI stream. Incomplete or corrupt entries cold bake; directory symlinks are
rejected and escaping leaf symlinks are replaced without following them. Writes
are atomic. A successful hit preserves the entry's modification time.

Pixels are reusable independently of the global geometry identity. Every hit
recomputes the current reflectance provenance and emits a fresh manifest bound
to the current full world and layout. Meshes, child content hashes and the parent
catalog are regenerated when required. Existing runtime checksums, source
classification and allocation bounds are unchanged. Development never traces
rays and can reuse only complete validated artifacts.

Input identity is content-addressed, with no Git history, timestamp or latest
snapshot requirement. Historical matching entries are reusable after arbitrary
intervening edits or reverts. Reordering may conservatively miss; missing
dependencies never imply a hit. Pixel entries are optional generated output,
not a new runtime resource or format. They contain no JSON or PNG.

CI restores and saves the project `.karty/bakes/` directory, including level
`pixels/` subdirectories. A historical cache predating pixel entries can reuse
an exact complete artifact; its first changed build cold bakes missing pixel
records. Subsequent reuse is independent of cache age and Git history.

Coverage includes far/local edits, wall additions/removals, old/new influence of
moved lights, current-input reuse from historical entries and reverts, changed
sampling/distance/producer, incomplete/corrupt entries, path confinement,
cancellation, exact equality with fresh bounded output, unbounded broad
invalidation, and automatic/imported distance mismatch.

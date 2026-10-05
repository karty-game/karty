# Static solids

SDK 0.0.8 adds source version 6 and the independently
versioned compiled-world capability `world/static-solids@1`. This keeps placed
opaque details independent of convex room decomposition. Polygon detail grows
object triangles rather than room sectors or portals. The public SDK owns the
records; this CLI owns authoring expansion and material resolution.

Root documents and prefabs accept optional `solids` and `contents` lists. Prefabs
may contain only solids, only sprite contents, or nested instances of those
prefabs; no room is generated for them. The complete level must still contain
room sectors. Solid and sprite instance identities retain their instance path.
Uniform scale, yaw and translation apply through nested instances; material
substitutions apply to solids and loose sprite textures. Sprite anchors are
assigned to the first containing compiled sector in stable order, including
floor/ceiling elevation and shared boundaries. An anchor outside all sectors
rejects the complete world. Surface extent may cross sector boundaries.

```yaml
version: 6
solids:
  - id: pillar
    footprint: [{x: 1, y: 1}, {x: 2, y: 1}, {x: 2, y: 2}, {x: 1, y: 2}]
    bottom: {c: 0}
    top: {c: 4}
    side_material: marble
    top_material: marble
    bottom_material: marble
    collision: true
```

The footprint is strictly convex and CCW, with 3–64 vertices. The complete
expanded world permits at most 1,024 solids. IDs are unique, nonempty UTF-8, at
most 128 bytes. Coordinates and plane coefficients are finite and bounded to
±1,000,000; evaluated bottom/top elevations obey the same bound. Every vertex
has at least 1e-6 thickness, and edges have at least 1e-6 length. All three named
materials must resolve to nonzero packaged texture IDs. Solids may overlap
sectors and other solids. `collision` defaults false and only describes blocking
volume; this contract does not add walkable tops or runtime solid mutation.

Optional `top_uv` and `bottom_uv` inherit the root horizontal UV settings;
triplanar and planar projections are compiled into affine mappings. Inherited
root wrap becomes planar on caps and anchors remain world-based. Optional
`side_uv` accepts only planar/world mapping in this first contract, shared across
all sides as XY-position X and height Z before scale, rotation and offset. An
absent side mapping uses the host's geometric side projection. A shared mapping
cannot express per-edge wrap or varying triplanar weights, so those side modes
are rejected. UV scale stays in world units per repeat, and world anchoring is
applied after instance geometry transforms, as for rooms.

Compiled world v3 retains optional `static_solids: {version: 1, items: [...]}`.
Records carry footprint, bottom/top planes, three material IDs, optional baked
mappings and omitted-false collision. A mapped solid requires the existing
`material_mapping` marker and its capability. All count, geometry, identity and
mapping validation precedes publishing a world. Canonical decoding rejects
unknown/duplicate fields, missing required fields and explicit null optional
payloads, including `static_solids: null`. A nil `items` slice canonically encodes
as `items: null` and denotes zero solids, following the lighting list precedent.
Source versions 1–5 reject the new fields; absent fields preserve existing source
and compiled encodings.

The CLI checks the selected SDK's runtime capabilities and declares
`world/static-solids@1` in the cartridge. Material atlas enumeration includes
room floor/ceiling/walls followed by solid side/top/bottom, with stable deduplication.
The existing total material and encoded-size limits still apply.

See the [isolated column fixture](../internal/worldbuild/geometry/testdata/static-solids.world.yaml).
Rendering, collision and shadow implementation live in the engine. This format
extension is not evidence of GPU correctness or phone performance.

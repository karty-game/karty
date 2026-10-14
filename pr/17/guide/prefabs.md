# Reuse a static object

A prefab groups geometry so you can place it more than once. Add this at the
top level of the room's world file:

```yaml
prefabs:
  - id: crate
    solids:
      - id: body
        footprint: [{x: 0, y: 0}, {x: 1, y: 0}, {x: 1, y: 1}, {x: 0, y: 1}]
        bottom: {c: 0}
        top: {c: 1}
        side_material: surface
        top_material: surface
        bottom_material: surface
        collision: true
instances:
  - id: crate-a
    prefab: crate
    transform:
      translation: {x: 4, y: 4, z: 0}
      yaw_degrees: 20
```

Solid footprints must be convex and counterclockwise. Split an L-shaped object
into several solids. Use room geometry for passages and platforms: a solid's
collision blocks movement, but does not make its top walkable.

An instance can translate, rotate, uniformly scale, or substitute materials.
Its expanded object keeps a stable name such as `crate-a/body`. Prefabs can
also contain rooms and sprites. Light positions remain global; placing a
prefab does not place or move lights for it.

Keep static objects static. Visual animation does not move collision or
gameplay query positions. For a door or moving platform, check the pinned SDK's
supported behavior before designing the interaction.

![The Hangar combines static machinery, repeating materials, and room lights.](assets/hangar.jpg)

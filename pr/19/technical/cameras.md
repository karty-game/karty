# Cameras

Use a 2D camera for canvas layers or a 3D camera for a mounted world.

## Camera 2D

`Camera2DSettings` pans and zooms an inclusive range of drawing layers. Layers
outside that range stay fixed for HUDs and menus. This API requires SDK 0.0.13.

```go
func showWorld(game *engine.Game) engine.Camera2DSettings {
    camera := engine.Camera2DSettings{
        Position: engine.NewVec2(400, 300),
        Zoom: 2,
        WorldMinLayer: 0, WorldMaxLayer: 100,
    }
    game.SetCamera2D(camera)
    return camera
}
```

At 960 by 540, world point `(400, 300)` appears at screen centre `(480, 270)`.
Zoom two doubles displayed distances. A HUD on layer 1000 stays fixed.

| Method                                         | Effect                                                   |
| ---------------------------------------------- | -------------------------------------------------------- |
| `game.SetCamera2D(settings)`                   | Replaces the active 2D camera settings                   |
| `game.ClearCamera2D()`                         | Restores ordinary canvas presentation                    |
| `settings.ScreenToWorld(point, width, height)` | Converts a logical pointer position to world coordinates |
| `settings.WorldToScreen(point, width, height)` | Converts a world point to logical canvas coordinates     |

Conversions must use the settings sent to the host and the project's authored
width and height. Browser CSS dimensions do not belong here.

Zoom must be finite and within `1/64` to `64`. The minimum layer cannot exceed
the maximum. There is one active ordinary 2D camera.

[Camera 3D](cameras.html#camera-3d) · [Willowbrook camera and picking](https://github.com/karty-game/karty/blob/main/samples/willowbrook/src/main.go)

Exact bounds are in `.karty/docs/cameras.md`.

## Camera 3D

`EntityCamera3D` projects a mounted world into a 2D output rectangle. Create it
after `OnLevelReady`, using that event's handle.

```go
func roomCamera(game *engine.Game, level engine.LevelHandle) *engine.EntityCamera3D {
    return game.NewCamera3D(level, engine.CameraSettings{
        Projection: engine.CameraProjectionPerspective,
        Output: engine.CameraOutputFull,
        Position: engine.NewVec3(2, 2, 1.7),
        FOVY: 1.05, OrthoHeight: 12,
        Near: 0.05, Far: 100,
        ViewportWidth: 960, ViewportHeight: 540,
    })
}
```

| Setting                           | Meaning                                                      |
| --------------------------------- | ------------------------------------------------------------ |
| `Projection`                      | `CameraProjectionPerspective` or `CameraProjectionIsometric` |
| `Position`, `Yaw`, `Pitch`        | World viewpoint, with angles in radians                      |
| `FOVY`                            | Vertical perspective field of view in radians                |
| `OrthoHeight`                     | Visible vertical world span in isometric projection          |
| `Near`, `Far`                     | Positive clipping distances, with far greater than near      |
| `ViewportWidth`, `ViewportHeight` | Render dimensions                                            |
| `Output`                          | Full rendering or a diagnostic view such as depth or normals |

Use `camera.Ref().SetPose(pose)` to propose a free camera's world pose and
`camera.SetSettings(settings)` to change projection and other camera settings.
An attached body controls position; use its movement methods or `Body.Teleport`
for positional changes. The entity's 2D transform places the rendered output on
the canvas. Use a lower drawing layer to put it behind HUD elements.

Use accepted poses for movement, as described in [Transform 3D](transforms.html#transform-3d).
`Destroy` removes the camera; level release also removes it. Camera count and
viewport bounds are in `.karty/docs/cameras.md`.

[Hangar camera setup](https://github.com/karty-game/karty/blob/main/samples/hangar/src/main.go) · [Isometric guide](../guide/isometric.html)

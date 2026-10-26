# Cameras

Use a 2D camera for canvas layers or a 3D camera for a mounted world.

## Camera 2D

`Camera2DSettings` pans and zooms an inclusive range of drawing layers. Layers
outside that range stay fixed for HUDs and menus.

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

[Camera 3D](cameras.html#camera-3d) · [Willowbrook camera](https://github.com/karty-game/karty/blob/main/samples/willowbrook/src/camera.go) · [Pointer picking](https://github.com/karty-game/karty/blob/main/samples/willowbrook/src/game_input.go)

Exact bounds are in `.karty/docs/cameras.md`.

## Camera 3D

`Camera3D` projects a mounted world into a 2D output rectangle. Create it
inside a level's ready callback, using that event's handle.

```go
func roomCamera(game *engine.Game, level engine.LevelHandle) *engine.Camera3D {
    return game.NewCamera3D(level, engine.Camera3DSettings{
        Projection: engine.CameraProjectionPerspective,
        Output: engine.CameraOutputFull,
        Position: engine.NewVec3(2, 2, 1.7),
        VerticalFOV: 1.05, OrthoHeight: 12,
        NearClip: 0.05, FarClip: 100,
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

[Hangar camera setup](https://github.com/karty-game/karty/blob/main/samples/hangar/src/tour.go) · [Isometric guide](../guide/isometric.html)

## First-person player

The `controllers-3d` package supplies the first-person controller. Its
`controllers-3d.sdk` build processor generates a typed adapter for your project.
After creating a camera in a ready level, attach the controller:

```go
// Import the package-generated adapter as fpscontroller from
// "YOURMODULE/.karty/processed/code/controllers-3d.sdk/controller".
func attachPlayer(game *engine.Game, camera *engine.Camera3D) (*fpscontroller.FirstPersonController, error) {
    options := fpscontroller.DefaultFirstPersonOptions()
    options.Speed = 4 // metres per second
    return fpscontroller.AttachFirstPerson(*game, camera, options)
}
```

It owns keyboard and pointer input, mouse capture, visible touch sticks and a
physics body with gravity and collision. It moves before the game's `OnUpdate`;
keep ordinary hooks for interactions and other game actions.

The game owns the camera. The controller cleans up its body and display when
closed, when the camera is destroyed, when its level is released, or at cartridge
shutdown. Only one local controller can be attached at a time.
`SetEnabled(false)` releases held controls; `Teleport` clears input and momentum
while retaining the camera and body. `FirstPersonFree` mode omits the body for
an inspection camera. `BeforeMove` and `Move` extend movement for authored rules.

[First-person guide](../guide/fps.html) · [Hangar traversal](https://github.com/karty-game/karty/blob/main/samples/hangar/src/main.go)

Exact options and lifecycle rules are in the installed package's README and
component documentation under `.karty/packages/controllers-3d/`.

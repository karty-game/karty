# API reference

Examples on these pages target SDK 0.0.17 / Go API 0.0.13.

Look up API behavior, coordinate systems and resource lifetimes. For project
setup and complete games, use the [Guides](../guide/).

| Page                                   | Covers                                            |
| -------------------------------------- | ------------------------------------------------- |
| [Runtime](runtime.html)                | Game lifecycle, entities and levels               |
| [Input](input.html)                    | Keyboard, pointers and capture                    |
| [Transforms](transforms.html)          | 2D and 3D positions, rotation and scale           |
| [Cameras](cameras.html)                | 2D scrolling, perspective and isometric views     |
| [Sprites and particles](graphics.html) | Image entities and authored effects               |
| [Audio](audio.html)                    | Playback, spatial emitters and listener direction |

Go snippets omit package and import declarations. `engine` and `assets` refer
to your project's `.karty/engine` and `.karty/assets` packages. Exact signatures
and version-specific limits live in `.karty/docs/`.

Camera 2D requires SDK 0.0.13. See
[SDK versions and toolchains](../design/sdk-and-toolchain.html) before changing a pin.

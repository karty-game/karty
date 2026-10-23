# Karty samples

Small games and interactive demos showing what you can build with Karty.
[Play them in your browser](https://karty-game.github.io/karty/main/), or pick a
sample and explore its code.

| Sample                                   | What it showcases                                             |
| ---------------------------------------- | ------------------------------------------------------------- |
| [Hangar](hangar/README.md)               | First-person exploration, physics, lighting and spatial audio |
| [Crate room](crate-room/README.md)       | Shooting, pushing and blasting physics objects                |
| [Terrain](terrain/README.md)             | Outdoor terrain, vegetation and swimming                      |
| [Vigil](vigil/README.md)                 | Isometric exploration, navigation and lighting                |
| [Willowbrook](willowbrook/README.md)     | A 2D treasure hunt with Tiled maps and a following camera     |
| [Last Train](last-train/README.md)       | Branching dialogue, character presentation and saved progress |
| [Orbital flight deck](ui-demo/README.md) | Interactive menus, settings and inventory                     |
| [Media lab](media-lab/README.md)         | Images, sound effects, music and video                        |
| [Retro effect lab](effect-lab/README.md) | Live comparisons of visual effects and presets                |
| [Particle lab](particle-lab/README.md)   | Fire, smoke, sparks and explosions                            |

Each sample lists the libraries, packages and creative resources it uses under
**Built with**. Find reusable game packages in the
[package catalog](https://karty-game.github.io/karty-packages/).

## Run a sample

Install [mise](https://mise.jdx.dev/), then build the CLI from the repository root:

```sh
mise install
mise run build
```

Choose a sample and follow its README. Most start like this:

```sh
cd samples/ui-demo
../../dist/karty dev
```

Open the address printed in your terminal. Edit the code and save to update the
preview. Start with `src/main.go`; each README links to the sample's main features.

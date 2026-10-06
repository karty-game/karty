# Made with Karty

Fly a ship. Explore a Roman courtyard. Mix music, sound and video.
These are playable examples of what you can make with Karty.

**[▶ Play the demos](https://karty-game.github.io/karty/main/)**

## 🚀 Orbital flight deck

Your ship. Your call sign. Your cockpit.
Switch ships, tune the thrusters and manage supplies while your flight keeps moving.
A complete game interface with tabs, menus, tooltips and inventory.

**[Play Orbital](https://karty-game.github.io/karty/main/ui-demo/)** · [Take a look](ui-demo/README.md)

## 🏛️ Roman court & galleries

Walk between marble columns, circle the basin and follow the light into the galleries.
Switch to an isometric view to take in the whole world.
Textured 3D spaces, lighting and touch controls, right in your browser.

**[Explore the court](https://karty-game.github.io/karty/main/world-camera/)** · [Take a look](world-camera/README.md)

## 🎧 Media lab

Turn up the music, layer on sound effects and play a video.
See how images, audio and video can share the same game scene.

**[Try the media lab](https://karty-game.github.io/karty/main/media-lab/)** · [Take a look](media-lab/README.md)

## 🎮 Sprite playground

A little character, a few shapes and a scene full of motion.
Move with the arrow keys or place your character with a click.
A small starting point for your own 2D game.

**[Try the playground](https://karty-game.github.io/karty/main/pong/)** · [Take a look](pong/README.md)

## Make it yours

Want to change a sample? From the repository root:

```sh
mise install
mise run build
./dist/karty sdk install 0.0.9
cd samples/ui-demo
../../dist/karty dev
```

Open the address printed in your terminal. Edit, save and see your changes.
Pick another sample by changing `ui-demo` to its folder name.
For the courtyard, follow its [launch steps](world-camera/README.md#run-it).

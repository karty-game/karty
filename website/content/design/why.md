# Why Karty?

I started Karty to make the engine and framework I wanted to use myself:
write game rules in Go, build a room from simple shapes, and try it quickly.

![Concept illustration of a room taking shape from outlines, reusable crates and material swatches. This is artwork, not an editor screenshot.](assets/design-room.png)

## Make a room and try it

Unity and Unreal feel too big, too slow, and too general-purpose for my games.
I miss the directness of Doom and Half-Life level editors. Making a room and
trying it felt simple.

Karty is aimed at small games and prototypes, including retro first-person
games, isometric adventures and 2D games. For worlds, I want to work with
floor plans, heights, materials and reusable objects. That should be enough
to make a space worth exploring without first learning a full 3D modeling tool.

## Development

- **See changes immediately.** `karty dev` watches your level files and updates
  the preview as you work.
- **Iterate quickly.** Target code recompilation in 2–3 seconds.

## Visual

- **Good lighting.** Bake bounced light with Karty's built-in light baker,
  without extra tools.
- **Give the scene personality.** Use built-in post-processing effects for
  atmosphere, storytelling and gameplay highlights.
- **Fire, smoke and more.** A flexible GPU particle system draws thousands of
  sprites.

## Performance

- **Run on modest hardware.** Target 720p at 60 FPS in WebGL on a five-year-old
  smartphone.
- **Start playing your game quickly.** Keep downloads small and aim for startup within
  a few seconds.
- **Play offline.** Prefetch and cache assets in the background, ready to play
  without internet once the download is complete.

## Assets

- **Skip image preparation.** Let the [asset pipeline](asset-pipeline.html)
  resize textures, pack atlases and handle compression.
- **One texture is enough.** Generate normal, height and ambient occlusion maps
  from a single albedo image.
- **Build large maps.** Automatic chunking and background streaming let your
  world grow.

## Flexibility

- **UI with less code.** KartUI provides a dedicated language and reusable
  components for game interfaces.
- **Bring other tools.** [Packages](packages.html) let the community share
  reusable integrations, code and components.

Continue with [Architecture](architecture.html), or [make your first game](../guide/index.html).

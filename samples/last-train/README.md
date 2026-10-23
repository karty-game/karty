# Last Train

A short mystery aboard a 1930s train. Meet three passengers, follow clues and
make choices that lead to three endings. It showcases branching dialogue,
expressive characters, music and saved progress.

![Mara aboard the last train.](preview.jpg)

[Play](https://karty-game.github.io/karty/main/last-train/) · [All samples](../README.md)

## Try it

Choose **Begin the journey**. Click or tap to finish revealing a line, then
again to advance. Choose replies to shape the story. Use **Save / Load** to
keep your progress and **Log** to revisit the conversation.

## Built with

- [Dialogue](https://karty-game.github.io/karty-packages/packages/dialogue.html): Yarn Spinner stories, choices and dialogue UI.
- [Savegame](https://karty-game.github.io/karty-packages/packages/savegame.html): save slots, preferences and unlocked endings.

## Run locally

[Set up the CLI](../README.md#run-a-sample), then from the repository root:

```sh
mise run build-last-train-web
mise run serve-last-train
```

Open the address printed in your terminal. The first build installs the dialogue compiler.

Explore [the game](src/main.go) and [story](dialogue/story.yarn).
[Art credits](assets/art/README.md) · [Music credits](assets/audio/README.md)

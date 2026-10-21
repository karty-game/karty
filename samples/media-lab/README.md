# 🎧 Media lab

**Give your game something to listen to. And watch.**

Play a burst of effects, fade music in and out, and replay a video.
Images, sound and video all come together in this little interactive scene.

**[▶ Try the media lab](https://karty-game.github.io/karty/main/media-lab/)** · [More demos](../README.md)

## Press play

- **ONE SHOT:** play a short sound effect.
- **36-START BURST:** layer a burst of overlapping effects.
- **BGM REPLAY:** fade looping music in or out. Add effects while it plays.
- **Video:** use Play / replay video and Stop video, or click the left and right halves of the video area.

Click or tap to enable browser audio. The short video is a colorful test pattern
with a tone, so you can see and hear playback together.

The title, status and media controls use KartUI's presentation layer, keeping
their text sharp independently of the game rendering resolution. The images
and video remain in the game layer; sound controls stack on narrow screens.

The music is mastered as a continuous 48 kHz loop. The burst effect leaves
headroom for all simultaneous voices plus music. Its original WAV fixtures and
reproducible mastering steps are in [sound-sources](sound-sources/README.md).

## Run it

[Set up once](../README.md#run-a-sample), then from this sample's folder:

```sh
../../dist/karty dev
```

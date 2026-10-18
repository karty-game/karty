# Audio

## Playback

| API                                  | Playback                                                             |
| ------------------------------------ | -------------------------------------------------------------------- |
| `PlaySound`                          | Decoded one-shot with no source position                             |
| `PlayMusic`, `StopMusic`             | Streaming soundtrack                                                 |
| `PlayEnvironment`, `StopEnvironment` | Streaming ambience on an environment lane                            |
| `PlaySFXFrom`                        | One-shot following an entity, with XY spatialization                 |
| `NewSoundEmitter`                    | Positioned source with XYZ distance and optional collision occlusion |
| `NewAttachedSoundEmitter`            | Persistent emitter following an entity                               |

## Spatial emitters

Spatial emitters require `SoundEmitterOptions`. Create them after level
readiness. This example uses a generated project sound ID:

```go
func reactor(game *engine.Game, level engine.LevelHandle) *engine.SoundEmitter {
    return game.NewSoundEmitter(level, assets.SoundReactorHum,
        engine.NewVec3(6, 6, 1.5), engine.SoundEmitterOptions{
            Volume: 0.6, NearDistance: 2, FarDistance: 16,
            Loop: true, Occlusion: true,
        })
}
```

`SetAudioReceiver` selects the listening entity. Receiver and emitter must
belong to the same mounted level. Its `Right` vector sets stereo direction;
for camera yaw, use `(cos(yaw), -sin(yaw))` and update it when turning.

Near distance is the full-volume range. Far distance must be greater than near
and marks silence. Occlusion uses loaded collision geometry. Spatial clips
should be mono.

Start loops once. Stop screen-owned streams when leaving the screen. Keep
emitter handles to update settings or call `Stop`; level release stops them.
Fading voices can continue occupying capacity. Browsers may require player
input before audio becomes audible.

[Asset setup and playback examples](../guide/media.html) · [Hangar emitters](https://github.com/karty-game/karty/blob/main/samples/hangar/src/audio.go)

Exact voice and clip bounds are in `.karty/docs/assets.md` and
`.karty/docs/audio-emitters.md`.

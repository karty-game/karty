# Add sound and video

Put a short WAV effect in `assets/sounds/`. The CLI converts it to QOA and
generates a typed sound ID. Declare music and video in `karty.toml`:

```toml
[[assets.sound]]
name = "ui.click"
source = "assets/sounds/click.wav"

[[assets.music]]
name = "theme"
source = "assets/music/theme.wav"
profile = "music"

[[assets.video]]
name = "intro"
source = "assets/videos/intro.mpg"
```

Add the generated assets import, then call these methods from a player button
or input handler:

```go
func (game *Game) startMedia() {
    game.PlaySound(assets.SoundUiClick)
    game.PlayMusic(assets.MusicTheme, engine.StreamOptions{Loop: true, CrossfadeMS: 500})
    game.PlayVideo(assets.VideoIntro, 300, 130, 360, 180)
}

func (game *Game) stopMedia() {
    game.StopMusic(500)
    game.StopVideo()
}
```

For `my-game`, the assets import is `example.com/my-game/.karty/assets`.
Use the generated names in `.karty/assets/`. Play the click once per action,
and start music once when entering a screen or level.

Browser audio needs player input before it can play. Video sources must already
be MPEG-1 in an `.mpg` container, optionally with MP2 audio. The CLI does not
convert MP4. One video plays at a time; pause, seeking, and video looping are
not supported. The four position values set its top-left and display size.

Keep the built `content/` folder when sharing music or video. Use
[Media lab](../media-lab/) to check playback and overlapping effects.

![Media lab shows video above three sound controls.](assets/media-lab.jpg)

# Image and sound assets

Karty processes authoring assets while it builds the cartridge. The current
implementation supports **SDK 0.0.6**: images become lossless QOI
textures and WAV effects become lossy QOA sounds. Players only need the matching
host; they do not need FFmpeg, ImageMagick, CGO, or separate conversion tools.

Projects pinned to an older SDK keep that SDK's asset behavior. In particular,
changing the CLI alone does not switch an SDK 0.0.3 project to QOI or QOA.

## Add assets

Files under `assets/textures/` with a `.png`, `.jpg`, `.jpeg`, or `.webp`
extension are discovered as textures. WAV files under `assets/sounds/` are
discovered as game-scoped, one-shot sounds. A relative path becomes a dotted
logical name: `assets/sounds/ui/open.wav` becomes `ui.open`.

Use `karty.toml` entries when you need a different name, profile, or transform:

```toml
[[assets.texture]]
name = "ui.panel"
source = "assets/textures/panel.webp"
profile = "interface"

[assets.texture.transform]
max_width = 2048
max_height = 1024
filter = "smooth-lanczos3"
bit_depth = 8

[[assets.sound]]
name = "ui.open"
source = "assets/sounds/ui/open.wav"
profile = "effect"

[assets.sound.transform]
sample_rate = 24000
channels = "mono"
```

Omitted transform fields inherit the selected SDK profile. The SDK 0.0.4
candidate provides these profiles:

| Profile | Default processing |
| --- | --- |
| `sprite` | QOI, fit within 4096×4096, `nearest`, 8-bit channels |
| `interface` | QOI, fit within 4096×4096, `smooth-lanczos3`, 8-bit channels |
| `environment` | QOI, fit within 4096×4096, `smooth-lanczos3`, 8-bit channels |
| `effect` | QOA at 48000 Hz, preserve mono or stereo |
| `music` | Streaming QOA at 48000 Hz, preserve mono or stereo |
| `environment` (audio) | Streaming QOA at 48000 Hz, preserve mono or stereo |

Image resizing preserves the aspect ratio and never upscales. `max_width` and
`max_height` replace the inherited bounds when set. The only filters are
`nearest` and `smooth-lanczos3`; QOI output always uses 8-bit straight-alpha
RGBA. A 16-bit PNG therefore loses channel precision. Animated PNG and WebP
are rejected. QOI preserves the normalized pixels, but it is not guaranteed to
be smaller than the source PNG or WebP.

The WAV importer accepts little-endian RIFF/WAVE mono or stereo integer PCM at
8, 16, 24, or 32 bits and IEEE float32. Matching PCM and float extensible
headers are supported. Compressed WAV, RF64/RIFX, unsupported extensible
formats, and non-finite float samples are rejected. Valid target rates are
`22050`, `24000`, `44100`, and `48000`; `channels` is `preserve` or `mono`.
Stereo is downmixed only when `mono` is explicit. QOA always normalizes to
PCM16 and has a fixed encoder policy without a quality setting.

Sources are limited to 64 MiB. An image source is limited to 16384 pixels per
dimension and 64 million pixels; processed textures are limited to 8192 pixels
per dimension, 16 million pixels, and 64 MiB decoded RGBA. The host limits all
textures to 256 MiB decoded memory. Sounds are limited to 30 seconds and 6 MiB
of decoded PCM, with a 64 MiB decoded catalog limit. SDK 0.0.5 adds separately
staged QOA music and environment streams. Stream metadata allows up to four
hours, while the 64 MiB WAV authoring source limit still applies. Seeking and
level-scoped audio remain outside the pipeline.

## Play a sound

A build sorts logical sound names and writes nonzero `engine.SoundID` constants
to `.karty/assets/sounds.go`. For a sound named `ui.open`, use the generated
constant from your embedded `engine.Game`:

```go
func (game *Game) openMenu() {
    game.PlaySound(assets.SoundUiOpen)
}

```

Use the generated spelling rather than guessing how punctuation is converted.
IDs are local to one build and must not be stored in save data. Missing IDs are
ignored safely by the host. One-shots are limited to 32 simultaneous voices.

## Stream music and environment audio

Declare long-running WAV sources explicitly. They are encoded to QOA, wrapped
in a masked Karty media envelope, and staged as opaque
`content/<sha256>.kaud` sidecars rather than embedded in `game.kart`:

```toml
[[assets.music]]
name = "battle"
source = "assets/music/battle.wav"
profile = "music"

[[assets.environment]]
name = "rain"
source = "assets/environment/rain.wav"
profile = "environment"
```

Builds generate independently name-sorted `engine.MusicID` and
`engine.EnvironmentID` constants in `.karty/assets`. Music has one lane and environment audio has
two lanes; replacing a lane crossfades its previous stream:

```go
game.PlayMusic(assets.MusicBattle, engine.StreamOptions{Loop: true, CrossfadeMS: 800})
game.PlayEnvironment(engine.EnvironmentLane0, assets.EnvironmentRain,
    engine.StreamOptions{Loop: true, CrossfadeMS: 500})
```

## Cache and build output

SDK 0.0.4 processed entries live under
`.karty/cache/assets/v2/<cache-key>/`. Each entry contains a verified payload
and metadata. The key includes the source bytes, resolved transform, processor,
and importer/codec/resampler revisions. File timestamps and logical names do
not define identity. A source edit, effective transform change, or processing
revision change rebuilds the affected entry; corrupt and incomplete entries
are rebuilt automatically. Deleting `.karty/cache/assets/` is a safe cold
rebuild.

`dist/raw/asset-report.json` shows the effective recipe, cache hit, source and
output hashes and sizes, original and processed properties, and estimated
decoded cost. Game textures are listed under `textures`; processed level
textures are grouped with their level under `levelTextures`. The summary keeps
their source, output, and decoded costs separate. Unused game textures may be
stripped; `keep = true` retains a texture. Declared sounds are retained in this
version.

`karty build` and `karty dev` use the same processing policy and packaged bytes.
Dev mode watches PNG, JPG, JPEG, WebP, and WAV sources and rebuilds after a
change. Release builds do not apply a second quality pass, so a successful dev
build exercises the same QOI/QOA conversion used for distribution.

Install the published SDK and pin `[sdk].version = "0.0.6"` in `karty.toml`:

```sh
karty sdk install 0.0.6
```

Installation verifies the signed public metadata and bundle checksum.

## MPEG-1 video (SDK 0.0.5+)

Declare already-encoded MPEG-PS files explicitly:

```toml
[[assets.video]]
name = "demo.pattern"
source = "assets/videos/test-pattern.mpg"
```

The CLI generates `assets.VideoDemoPattern`. Play or restart it with
`game.PlayVideo(assets.VideoDemoPattern, x, y, width, height)` and stop with
`game.StopVideo()`. Coordinates describe the top-left and size of a display
rectangle. One video plays at a time, fitted without distortion above world
entities and below UI. The final frame stays visible. No completion event,
seeking, pause, or looping API is provided yet. Host logs report failures.

Source files must be confined project `.mpg` files containing MPEG-1 video
(optionally MP2 audio), at most 1280×720 and 64 MiB each. Up to 64 videos may
be declared. No video transcoding or automatic discovery is performed.
The header must be in the first 128 KiB. MPEG-2 and MP4/H.264 are unsupported.

Video stays in an enveloped and masked `content/<sha256>.kvid` beside `game.kart`;
only the versioned catalog
is embedded in WASM. Ship the `content/` directory for native and web builds.
The host streams from disk or HTTP with bounded, verified 128 KiB chunks.
Video does not pass through Brotli-compressed level data. A cartridge supplies
only a typed ID, never an arbitrary filesystem path or URL.
Browser audio starts after user interaction. Slow downloads can stall video
without blocking the game. Audio/video synchronization is approximate.

For example, encode a clip before building:

```sh
ffmpeg -i input.mp4 -vf 'scale=640:-2' -r 25 -c:v mpeg1video -q:v 5 \
  -c:a mp2 -ar 48000 -ac 2 -f mpeg output.mpg
```

Video requires SDK 0.0.5 or newer. The samples pin the published SDK 0.0.6.
Earlier SDKs reject video declarations rather than silently omitting them.

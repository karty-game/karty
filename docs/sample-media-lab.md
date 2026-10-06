# Media lab development notes

For launch steps and controls, see the [sample README](../samples/media-lab/README.md).

## Media sources

- Transparent PNG, lossy JPEG and lossy WebP sources are converted to QOI.
- 8-bit mono, 24-bit mono and 16-bit stereo WAV sources are converted to QOA.
- One-shot playback includes a 36-call overlapping burst.
- A four-second music cue streams from a `.kaud` sidecar with a host-owned loop.
- MPEG-1 video with MP2 audio streams from a `.kvid` sidecar under `content/`,
  outside the Brotli level data. Its final frame remains visible until stopped.

Music crossfades independently of one-shot voices. Browser audio requires user interaction.
The sample uses SDK 0.0.9 typed pointer/frame hooks and its matching published host.
Start with its [client](../samples/media-lab/src/main.go) and
[assets](../samples/media-lab/assets).

## Regenerate the clip

The original clip is a three-second synthetic test pattern with a 440 Hz tone.
From the sample's directory:

```sh
ffmpeg -f lavfi -i testsrc2=size=320x180:rate=25 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 -t 3 \
  -c:v mpeg1video -bf 0 -q:v 5 -c:a mp2 -ac 2 -b:a 96k -f mpeg \
  assets/videos/test-pattern.mpg
```

## Optional browser diagnostic

After staging the web build, run `mise run check-video` from the CLI repository
root. It executes the real WASM host/cartridge in headless Chromium and checks
HTTP loading, changing pixels, stop, replay and end-of-file behavior.
It does not verify perceived audio quality or replace manual cross-browser review.
Routine [client validation](sample-development.md#focused-validation) uses
crafted CPU fixtures without loading the sample's media or a browser.

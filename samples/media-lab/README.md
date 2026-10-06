# Media lab

This sample exercises Karty's asset pipeline and runtime media paths:

- PNG with transparency, lossy JPEG, and lossy WebP sources converted to QOI.
- 8-bit mono, 24-bit mono, and 16-bit stereo WAV sources converted to QOA.
- Repeated one-shot playback and a 36-call overlapping machine-gun burst.
- A four-second music cue streamed from an opaque `.kaud` sidecar with a host-owned loop.
- MPEG-1 video with MP2 audio, streamed from an opaque `.kvid` sidecar.

Click the three panels along the bottom of the window. Browser audio starts only
after user interaction. The music control crossfades a host-owned stream in and out;
one-shot voices remain independent.

Build the sample from the repository root:

```sh
mise run build
./dist/karty sdk install 0.0.9
cd samples/media-lab
../../dist/karty build --target web
../../dist/karty dev
```

The video is an original three-second synthetic test pattern with a 440 Hz tone.
Click center-left (above the sound panels) to play/replay; center-right stops it.
It is staged under `content/`, outside the Brotli level data. The final frame remains
visible until stopped. The sample uses SDK 0.0.9 typed pointer/frame hooks and its matching host.
Until publication, follow the [candidate setup](../README.md) rather than the
normal public installation command above.

Regenerate the clip with FFmpeg:

```sh
ffmpeg -f lavfi -i testsrc2=size=320x180:rate=25 \
  -f lavfi -i sine=frequency=440:sample_rate=48000 -t 3 \
  -c:v mpeg1video -bf 0 -q:v 5 -c:a mp2 -ac 2 -b:a 96k -f mpeg \
  assets/videos/test-pattern.mpg
```

After staging the web build, run `mise run check-video` from the CLI repository
root. It executes the real WASM host/cartridge in headless Chromium and checks
HTTP loading, changing pixels, stop, replay, and end-of-file behavior. It does
not verify perceived audio quality or replace manual cross-browser review.

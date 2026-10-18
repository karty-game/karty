# Last Carriage

Original 32-bar AABA foxtrot in A minor, 92 BPM, 83.478 seconds. Piano introduces
the theme; clarinet takes the second statement and contrasting C-major bridge;
piano returns with low clarinet answers. Acoustic bass, soft nylon guitar and a
brush kit accompany it. Onsets and velocities vary with a fixed seed. There is
no artificial record crackle, bit reduction or nonlinear saturation.

`last-carriage.mid` is an editable score. `last-carriage.wav` is the ready-to-build
48 kHz stereo PCM16 master; players and ordinary builds need no music tools.
At launch the sample lets the opening draw for 120 updates (two seconds at
60 Hz) before starting audio, to avoid competing with its first image/font uploads.
The original composition is covered by the repository's MIT license.

Instruments: **GeneralUser GS 2.0.3**, S. Christian Collins, rendered with
**FluidSynth 2.2.5**. The bank's author permits private and commercial music
creation; the complete upstream terms, including its sample-origin statement,
are retained in [GeneralUser-GS-LICENSE.txt](GeneralUser-GS-LICENSE.txt).
See [the author's bank repository](https://github.com/mrbumpy409/GeneralUser-GS)
and [FluidSynth](https://www.fluidsynth.org/). The bank itself is not bundled.

To reproduce from the CLI repository root, obtain `GeneralUser-GS.sf2` at
upstream commit `684543d5e5efaef08d02be50dcda8d552478fa60`. Its SHA-256 is
`9575028c7a1f589f5770fccc8cff2734566af40cd26ed836944e9a5152688cfe`;
the generator rejects any other bank. Install FluidSynth 2.2.5 and use:

```sh
LAST_TRAIN_SOUNDFONT=/path/to/GeneralUser-GS.sf2 \
FLUIDSYNTH_LIBRARY=/path/to/libfluidsynth.so.3 mise run generate-last-train-score
```

The Python source uses only the standard library and renders directly through
FluidSynth without opening an audio device. Two warm-up cycles establish note
releases and room tails before the retained cycle. A small 2 ms correction
removes residual boundary mismatch without fading to silence. Mastering applies
DC removal and linear gain to a 0.28 peak. FluidSynth builds/platforms can differ
slightly; the checked-in WAV is the canonical build input. Musical approval still
requires listening; numerical measurements do not establish period authenticity.

Foley is independent: `mise run generate-last-train-audio` regenerates only
`night-train.wav`, `station-whistle.wav` and `ticket-punch.wav` using the Go source.
After building the web sample, `mise run check-last-train-audio` checks the actual
packaged QOA against its source and observes WebAudio for a complete music loop.

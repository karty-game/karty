# Retained Media Lab audio inputs

These are unchanged copies of the original Media Lab WAV fixtures. Ordinary
builds package the mastered files under `assets/sounds/`; these inputs stay
outside that asset directory.

`scripts/media-lab-audio.go` prepares the music from three complete cycles,
resamples with the normal asset processor and retains the middle cycle. This
provides real filter history/lookahead without fading through silence on every
repeat. The result is a four-second 48 kHz stereo PCM16 master, reduced by 6 dB
to leave room for effects.

The PCM24 mono effect retains its format and sound but is attenuated by 1/40.
That leaves headroom for the demo's complete overlapping burst and music. The
asset-pipeline test checks the sum at the 32-voice cap after QOA encoding, so
codec overshoot is included. A voice cap alone does not prevent distortion.

From the CLI repository root:

```sh
mise run generate-media-lab-audio
mise run check-media-lab-audio-assets
```

The check recomputes both mastered assets and compares every byte. Keep this
generator, its retained inputs and the mastered WAVs together in Git.

After `mise run build-media-lab-web`, run `mise run check-media-lab-audio-web`
with the pinned Playwright browser installed. This checks the actual production
WebAudio output across a complete music loop and two full effect bursts, with
zero underruns and peak output below 0.95. The JSON report records the cartridge
and host hashes under `dist/validation/media-lab/audio/`. It measures the browser
graph; physical speakers and listening quality still need device review.

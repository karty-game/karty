# Vigil audio sources

These are original synthesized compositions and sound effects created for Vigil
by the Karty contributors. They contain no recordings, purchased sample packs,
Diablo audio or other external source material. The retained standard-library
Go generator is [`scripts/vigil-sound-assets.go`](../../../../scripts/vigil-sound-assets.go).
The generator and resulting assets use the repository's MIT license.

All files are **mono, 24,000 Hz, signed 16-bit little-endian PCM in RIFF/WAVE**.
Numbers below describe the actual quantized samples, with full scale normalized
to one. The generated JSON records exact measurements, sample counts and SHA-256
hashes.

The build converts the church stream to **48,000 Hz mono**, matching the host
mixer. Keeping that conversion offline avoids recurring resampling work on the
browser's game thread. The authored WAV and the short effects remain unchanged.

| File                  |  Length |     Peak |      RMS | Playback                |
| --------------------- | ------: | -------: | -------: | ----------------------- |
| `church-ambience.wav` | 32.00 s | 0.033265 | 0.009911 | Streaming music loop    |
| `fireball-cast.wav`   |  0.35 s | 0.319987 | 0.063162 | Spatial one-shot        |
| `fireball-impact.wav` |  0.65 s | 0.359996 | 0.051296 | Spatial one-shot        |
| `shrine-chime.wav`    |  1.80 s | 0.239998 | 0.057165 | Spatial one-shot        |
| `stone-footstep.wav`  |  0.18 s | 0.319987 | 0.058600 | Subtle spatial one-shot |
| `ember-crackle.wav`   |  3.00 s | 0.074160 | 0.014000 | Optional spatial loop   |

The church bed uses a quiet organ-like blend of sine partials, a very low D
pedal and slowly overlapping D minor, B-flat major seventh, G minor seventh and
A minor seventh voicings. Harmony changes every eight seconds without a beat
or a prominent melody. Its authored gain includes a factor of 0.35 so spatial
cast, impact and chime remain audible above it. The music lane has no separate
volume control. Its 32-second duration belongs in `assets.music`;
it exceeds the decoded `assets.sound` duration limit.

The cast combines a restrained rising tone and filtered air. The impact blends
a low damped thump, soft debris noise and a faint stone overtone. The shrine
uses three gently struck, decaying bell-like tones; footsteps combine a short
low heel sound and filtered grit. Ember crackles use quiet filtered noise and
small scattered transients. Runtime gains and spatial attenuation reduce the
effects further.

The bed's oscillator frequencies, modulation and chord envelopes complete
whole cycles over 32 seconds. Its room reflections use circular delays. Ember
noise/filter state and transient tails also wrap around their complete loop.
The boundary sample step is checked against ordinary neighboring sample steps;
loops do not fade through silence on each repeat. Every one-shot starts and
ends at an exact zero sample, with softened attacks and tails.

Regenerate and verify from the **CLI repository root** using the pinned tasks:

```sh
mise run generate-vigil-sounds
mise run check-vigil-sounds
```

The check recomputes every PCM byte and the report without writing, verifies
headroom and negligible DC, checks loop boundaries and silent one-shot
endpoints, then compares retained files exactly. Fixed seeds make noise and
crackle placements reproducible. This validates the asset data; speaker,
headphone and physical-device listening remain separate checks.

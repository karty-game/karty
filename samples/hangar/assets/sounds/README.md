# Hangar sounds

These mono PCM16 WAV loops are derived from CC0 recordings:

| Output              | Recording and author                                                                                                  | Treatment                                                   |
| ------------------- | --------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `reactor-hum.wav`   | [Industrial fan — DBS_Sounds](https://freesound.org/people/DBS_Sounds/sounds/613158/), recorded with an Olympus LS-P4 | 29.75-second steady machine loop, 250 ms overlap            |
| `computer-idle.wav` | [Computer Fan — magnus589](https://freesound.org/people/magnus589/sounds/184565/), a laptop fan                       | Quiet 23.53-second loop, 250 ms overlap; no repeating beeps |
| `crate-scrape.wav`  | [Scrapes — AntumDeluge](https://opengameart.org/content/scrapes), object dragged over a cinderblock; `scrape-2.wav`   | Sustained friction at 0.2–1.0 seconds, 80 ms overlap        |
| `fire-ambience.wav` | [Bonfire — forfie](https://freesound.org/people/forfie/sounds/364992/), real burning fire                             | Quiet 29.75-second loop, 250 ms overlap                     |
| `acid-ambience.wav` | [Water bubbles — kijjaz](https://freesound.org/people/kijjaz/sounds/16741/), air bubbles under water                  | Quiet 14.61-second loop, 250 ms overlap                     |

All five recordings are [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/).
The license and source checksums are retained in
[`../../sound-sources/`](../../sound-sources/). Freesound inputs were decoded
from the publicly available high-quality MP3 previews, downmixed to mono, and
resampled to 48 kHz. They are retained as PCM16 WAVs; the crate input is the
original 44.1 kHz WAV. Original pages, preview URLs, download checksums and
conversion details are in [`provenance.json`](../../sound-sources/provenance.json).

`scripts/hangar-sound-assets.go` uses the Go standard library to prepare loops
from these retained inputs. It removes DC, crossfades the seams, normalizes RMS
and caps peaks. It adds no synthesized tones. Inputs live outside `assets/` so
only the five prepared clips are packaged into the cartridge.

Run `mise run generate-hangar-sounds` or `mise run check-hangar-sounds` from the
CLI repository root. No download or external audio tools are needed to build.
Keep the processor, source recordings, provenance and outputs together in Git.
The crate emitter volume follows accepted horizontal crate movement.

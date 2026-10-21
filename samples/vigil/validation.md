# Checking Vigil

Run these tasks from the CLI repository root with the sample's matching
SDK 0.0.17 and public host installed. Browser checks also require
`mise run install-browser`. Set `KARTY_HOST_WEB` and `KARTY_HOST_NATIVE` when
using locally prepared hosts.

| Task                             | What it checks                                                                                          |
| -------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `mise run check-vigil`           | Collision geometry, endpoint projection, navigation results and waypoint following                      |
| `mise run check-vigil-sounds`    | Deterministic PCM assets, headroom and loop endpoints                                                   |
| `mise run build-vigil-web`       | Web cartridge compilation and packaging                                                                 |
| `mise run build-vigil-native`    | Native cartridge compilation and packaging                                                              |
| `mise run check-vigil-web`       | Actual WASM execution, movement, shrine traversal, touch input, sprites and occluder fading in Chromium |
| `mise run check-vigil-audio-web` | Actual WebAudio output, stereo placement and distance attenuation during play                           |
| `mise run review-vigil-walk`     | An interactive eight-direction animation preview for manual art review                                  |

Build web and native targets sequentially; they share generated UI staging.
Browser checks consume the built web distribution. Rebuild it after source or
host changes before checking gameplay.

## Reports and limits

Browser checks write reports, artifact hashes and captures under
`dist/validation/vigil/`. Retain the matching reports and artifact hashes when
comparing revisions. A successful package build establishes compilation and
packaging; it does not establish game execution or rendering.

Software-rendered checks use SwiftShader. Movement checks use quarter render
scale; art and fireball-light captures use full scale. Separate browser
fixtures keep the 120-second deadlines while covering input, traversal,
portrait layout, prop contact, fireballs and occluder fading.

Audio checks measure software playback. Physical speakers, headphones, touch
hardware and device frame rates require separate testing. The animation
preview supports manual review; its export alone does not establish animation
quality or actual game playback.

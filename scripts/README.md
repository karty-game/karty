# CLI scripts

| Script                          | Root mise task               | Purpose                                                                                      |
| ------------------------------- | ---------------------------- | -------------------------------------------------------------------------------------------- |
| `smoke.sh`                      | `smoke`                      | Build and execute a temporary customer project with installed public SDK/host assets         |
| `check-integration.sh`          | `check-integration`          | Run the customer integration gates                                                           |
| `check-materialize-software.sh` | `check-materialize-software` | Prepare public SDK/tool/driver state, then run the bounded cold/warm native atlas regression |
| `check-video.mjs`               | `check-video`                | Exercise packaged media playback in a real browser                                           |

Sample-site building and validated Pages assembly live in `cmd/karty-samples`.
The tracked sample artwork generator lives in `cmd/world-camera-materials`.

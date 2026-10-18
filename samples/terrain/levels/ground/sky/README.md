# Cove clouds

`clouds-cirrus-v1.png` supplies the sparse layer: transparent, distant high-cloud
traces in a 2:1 equirectangular image. The original image is used without
postprocessing. Its [prompt record](../../../../../scripts/terrain/assets/clouds-cirrus-v1.prompt.md)
records generation and alpha measurements.

`clouds-dense.png` supplies the denser weather layer. [world.yaml](../world.yaml) selects both
images and starts the cove at 20% cloud amount; the ordinary CLI sky pipeline
packages them with the level.

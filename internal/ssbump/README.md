# Offline SSBump generation

`cmd/karty-ssbump` exposes the deterministic heightfield generator as a standalone
tool. WebGPU project builds also invoke its Go library automatically when the
selected SDK explicitly supports `world/material-ssbump@1` (candidate 0.0.21).
Legacy hosts and SDKs without that capability keep ordinary material data.
The CLI owns this processing; the generator itself needs only Go's standard
library and builds with `CGO_ENABLED=0`.

```sh
CGO_ENABLED=0 mise exec -- go run ./cmd/karty-ssbump \
  --height height.png --output detail-ssbump.png \
  --depth 4 --radius 16 --samples 32 --steps 16
```

Input is an opaque grayscale PNG, interpreted as linear normalized height, with
white highest. RGB input is reduced to grayscale; embedded color profiles do not
change the height values. `depth` is the height range in input texels, and
`radius` is the horizontal search distance in texels. Their artistic values need
material review; height inferred from albedo remains an approximation. Source
dimensions and maximum ray-step work are bounded before decoding/allocation.
The command accepts interruption and publishes output through a temporary file,
so cancellation or validation failure does not replace a previous output.

Output RGB is **linear data**, with no sRGB conversion. The basis is:

| Channel | Tangent-space direction                        |
| ------- | ---------------------------------------------- |
| R       | `(0.8164965809, 0, 0.5773502692)`              |
| G       | `(-0.4082482905, 0.7071067812, 0.5773502692)`  |
| B       | `(-0.4082482905, -0.7071067812, 0.5773502692)` |

PNG columns are +tangent X, rows are +tangent Y, height is +Z. UV V therefore
follows image row order. An importer that flips V must transform tangent
orientation consistently. The local normal is `normalize(-dh/dx,-dh/dy,1)`.
All height sampling wraps periodically, including normal derivatives and ray
visibility. A repeating source must itself tile correctly.

This is a bounded approximation inspired by
[Valve's self-shadowed RNM paper](https://cdn.akamai.steamstatic.com/apps/valve/2007/SIGGRAPH2007_EfficientSelfShadowedRadiosityNormalMapping.pdf),
implemented independently. It computes normalized squared positive normal
projections onto the RNM basis, then attenuates each with cosine-weighted,
directionally weighted visibility from deterministic hemisphere rays against
the periodic bilinear heightfield. It does not claim exact Source texture
compatibility or calibrated integration. Radius and discrete step spacing can
miss detail; increase quality when reviewing high-relief inputs.

An unoccluded flat input produces `(85,85,85)`. Occluded coefficients are never
renormalized: doing so would erase their darkening. The RGB values multiply RNM
irradiance coefficients directly. They are not XYZ normal components. Mips must
average coefficients linearly; do not renormalize them as normals. Alpha is 255,
reserved for a future versioned packing decision. Do not multiply the same
height-derived AO again in the receiving diffuse shader. Baked scene shadows
remain separate from this material-scale occlusion.

The command reports algorithm identity, dimensions, controls and output PNG
SHA-256 as JSON. Production builds read the unchanged public material-data B
height channel independently for each original 256px material tile. They use
depth 4 (scaled by the material height gain), radius 16, 32 samples and 16 steps.
Exact height bytes, controls, algorithm and coefficient-mip revision identify
the content-addressed per-tile cache. Cancellation publishes no partial result.

Generation precedes chart packing, so unrelated chart cells never cast rays into
each other. Chart reprojection transfers directional coefficient energy into
the final tangent frame; masked bands composite it without normalizing away
occlusion. Source normal, height, AO and reflectance data stay intact. The new
public companion uses explicit entries, a digest-bound descriptor, a per-level
marker and a cartridge feature. Models and terrain retain the ordinary normal
path initially; world RNM diffuse selects the companion. This does not claim
measured device performance or exact Source texture compatibility.

# Project-authored sky assets, version 1

SDK 0.0.12 adds source-v12 `environment.sky` and the `world/sky@1`
runtime capability. Released source/environment contracts remain unchanged.
The host keeps dome geometry, compositing, lighting, sun rendering and bounded
GPU resources; each project owns sky PNGs and appearance settings.

The CLI compiles only explicitly referenced level-relative PNGs into the public
SDK's versioned `@world/sky.bin` level sidecar. Sources use existing confined
regular-file reads and bounded PNG headers before pixel decoding. The reusable
processor preserves authored dimensions/alpha while converting sRGB colors to
linear premultiplied RGBA8, then deterministic gzip. Runtime loading validates
the whole resource, including gzip lengths and checksums, before allocating GPU
images. No sky declaration means no artwork and no new capability requirement.

Optional sparse/dense clouds, background and horizon are independent slots.
Optional settings override SDK defaults individually; explicit null is rejected.
Cloud mapping, relative motion, parallax, horizon vertical UV scale, haze and horizon emission belong to the
project. An authored horizon draws independently of legacy `backdrop` names;
omit the `horizon` path to leave it out.
Time/cloud game controls continue using the released environment API.

The environment sample retains original artwork and prompts in its daylight
level's `sky` directory. Its generator owns the authored paths and presentation
settings. City and space remain optional source variants; the daylight cartridge
contains only its referenced cloud layers and mountains.

Validation covers deterministic conversion/packaging, color and alpha, source
version/schema/null gates, capability selection, omitted settings, decoded image
bounds, escaped symlinks and asset-free legacy levels. Host execution and graphics
qualification are recorded separately by the engine.

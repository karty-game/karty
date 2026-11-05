# Managed host bundles

SDK 0.1.1 explicitly selects complete WebGPU host delivery with
`host.format = "host-bundle@1"` and `host.renderer = "webgpu"`. The signed SDK
manifest pins ZIP archives for web and supported native platforms. SDK metadata without that declaration is unsupported by the current CLI.
The CLI's current SDK selection advances only after the corresponding signed
release is available; local candidate validation does not publish a release.

SDK 0.1.0 introduced the breaking WebGPU cartridge format. Rebuild cartridges
and their level resources with the current SDK 0.1.1 and use its matching host.
Older cartridges are incompatible with the new host, and new cartridges do not
run on older hosts. Changing a version label cannot migrate compiled resources.

The CLI verifies the archive SHA-256, then the bounded `host-bundle.toml` file
inventory, renderer, target and native executable architecture. Managed archives
contain only declared regular files and their structural directories. Paths,
file counts, compressed and decoded sizes are bounded; symlinks, traversal,
case collisions, extra files and partial inventories fail. Native bundles carry
their GPU/SDL libraries and licenses. Web bundles carry the complete module set,
matching Go runtime glue and identical WASM aliases.

Installation publishes a complete verified directory atomically. The cache
retains the pinned ZIP outside its payload directory so future verification can
compare the cache descriptor against authenticated archive bytes. Rewriting a
payload and its descriptor does not create a new trust anchor. Corrupt cache
entries fail rather than being silently accepted or overwritten.

Managed renderer resolution happens before level compilation. A verified
WebGPU bundle selects the offline geometry/material profile and its optional
SDK capabilities, then native/web staging copies the same verified inventory.
SDKs declaring bundle delivery require complete verified bundles for native
local overrides too; selecting a raw executable is rejected before asset
processing. Select the bundle directory or its declared executable entrypoint.
Explicit local host overrides continue through that shared verifier. A raw-only
build still uses the selected renderer's resource profile without adding a
native distribution as a side effect.

The CLI stages complete verified host bundles. Single-file hosts, the old CDN
launcher and its flags are removed; there is no renderer fallback. Browser
bundles include the complete licensed texture-codec family.

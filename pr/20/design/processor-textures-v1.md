# Processor-generated level textures, version 1

Build processors may return an optional `assets.json` alongside ordinary files in
the existing KPR1 response. This extends the build contract; it changes neither
the cartridge wire protocol. Processors without a catalog keep
returning opaque resources through `processed.Read`.

```json
{
  "version": 1,
  "textures": [
    {
      "level": "village",
      "name": "generated-background",
      "source": "generated/background.png",
      "key": "generated/background.png#0,0,1024,1024",
      "profile": "sprite"
    }
  ]
}
```

`level` names an existing directory under the project's `levels/`. `source`
names a PNG in the same response; it is a logical path, never a filesystem
write destination. `key` is the processor's lookup identity. Names must be
unique within a level, and keys unique within a processor. Paths cannot escape,
contain NUL, use backslashes or contain drive prefixes. Unknown catalog fields,
invalid PNGs, missing sources and collisions with authored texture names fail
the build.

The CLI validates every result before replacing its owned `.karty/processed/`
transaction. Images live under that ignored directory and pass through the normal
texture processing profile and level packager. They are not embedded into generated Go
source. The catalog and generated lookup are also ignored build outputs:

```go
id, ok := processed.TextureID("tiled.compile", imageKey)
```

Authored texture IDs retain their existing name-sorted order. Generated textures
follow them in name order. A changed authored catalog requires rebuilding the
processor output; stale generated IDs are rejected. Rebuilding creates a complete
matching map, lookup and level distribution without editing authored assets.

KPR1 retains its limits of 256 files and 16 MiB of aggregate file content per
request or response. The catalog is at most 64 KiB and 256 textures. Generated
PNG decoding is bounded to 4096 pixels per edge and 256 MiB of aggregate decoded
pixels per processor. Normal asset-pipeline limits apply afterward.

The Tiled processor uses this contract for explicitly selected static background
layers. Its original map output retains gameplay data; a separate render map
references the generated textures. Dynamic scenery, animation, collisions,
quests and original source images remain authored inputs. The build never writes
merged PNGs back into `levels/`.

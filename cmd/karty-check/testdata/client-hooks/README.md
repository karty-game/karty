# Client hooks fixture

One square room, one authored actor, one generated solid texel and one hidden
160×120 camera exercise the SDK 0.0.9 hooks/actions contract. No sample assets,
UI, materials toolchain, offline bake or runtime lightmap bake are loaded.

The authored condition resolves the mounted actor, waits two frames and invokes
a typed action. The game releases/remounts once; native/browser runners require
fresh actor IDs. Keyboard down/up, pointer release and the accepted transform
callback emit tags on the remounted actor, which the runners independently check.

The camera is hidden because its transform feedback is under test, while
material drawing, lightmaps and graphical pixel correctness belong to dedicated
engine fixtures. A passing hooks check does not establish world graphics or
manual/device acceptance. The browser process has a 9-second deadline and is
killed on cleanup so a stalled driver cannot leak fixture processes.

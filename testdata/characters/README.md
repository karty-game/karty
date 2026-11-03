# Character source fixtures

`model.glb` is a synthetic white triangle with one skin influence.
`animations.glb` moves its named `hip` node over one second. They contain no
project artwork or external resources and are owned by the CLI release gate.

Regenerate both files with `python3 testdata/characters/generate.py` from the
repository root. Test execution reads the checked-in GLBs and needs no Python,
Node, native decoder or sibling repository.

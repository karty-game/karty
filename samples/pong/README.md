# Sprite and input sample

This small client demonstrates sprites, text/vector entities, typed keyboard and
pointer hooks, and independent level mount/release/remount. Arrow keys move the
player; clicking places it and the marker. Continuous animation stays in the
frame hook, while input and level results use typed handlers.

The sample pins the current SDK 0.0.9 candidate. Follow the
[SDK and matching-host setup](../README.md), then run `../../dist/karty dev` here.
`mise run check-sample-clients` from the CLI root compiles this client with crafted
asset identifiers and no texture or graphical host loading.

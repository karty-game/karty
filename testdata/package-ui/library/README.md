# Shared HUD kit

A UI-only package: no `go.mod`, game engine imports or generated bindings.
The game supplies values and callbacks to the typed `HudBadge` child and owns
its root screen and lifetime. `HudCard` supplies a reusable layout. The unused
component exercises normal UI asset stripping.

```sh
karty package import /path/to/hud-kit
karty dev
```

Or export this directory and import the resulting `.kpkg`. The package name
comes from `karty-package.toml`, independently of this directory's name.

Compose the child in a game-owned `.kui` file:

```html
<template>
  <panel modal="true">
    <HudBadge props={ HudBadgeProps{Title: title, Value: value, Change: change} }/>
  </panel>
</template>

<script setup lang="go">
func setup(title string, value func() string, change func()) {}
</script>
```

The component asset is `hud-kit.badge`; component/layout/Go declaration names
retain KartUI's filename conventions. Authors should prefix these names to avoid
collisions with other libraries. The consuming game's theme overrides package
defaults. The `hud-panel` image token resolves to the qualified `hud-kit.panel`
asset; source paths remain inside the package. Fonts are optional defaults for
the existing body/display/mono roles; this example uses the built-in font.

The original panel PNG is reused from the placeholder kit's original frame
texture. Source and texture are MIT licensed; see LICENSE.md.

From the CLI root, `mise run check-package-ui` runs two disposable consumers
with different props, callbacks and theme colors. It adds a local test font,
checks folder edits and clean archive restoration, then executes both games
with Go and TinyGo in Chromium. Supply `KARTY_PACKAGE_UI_FONT` when DejaVu Sans
is not installed at the default Linux font location. The check also needs a
verified SDK 0.0.13 bundle, matching web host and Chromium installation.

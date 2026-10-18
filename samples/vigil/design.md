# Vigil design

Vigil is a short exploration game about bringing light back to an abbey crypt.
Its charcoal limestone, aged ivory, tarnished gold and oxblood cloth keep a
restrained palette. Warm candles and shrine lights guide the route through
cooler burial chambers.

## Space and presentation

The fixed camera uses 45° yaw and 30° elevation: a 2:1 dimetric projection.
The SDK names its parallel projection `CameraProjectionIsometric`. Upright
sprites compensate their height for this camera and retain grounded visual
origins. Pillars, choir stalls and sarcophagi use actual 3D solids with matching
blocking footprints; their surfaces write depth.

Masonry walls are 0.5 units thick. Camera-facing sections have a low cutaway;
far walls retain their full height. Their inner faces remain the walking and
sight boundary. A local foreground fade preserves the pilgrim's silhouette
without changing collision or revealing rooms through walls.

## Movement and visibility

Click routes use engine-owned A* and continuous clearance checks. The game
copies borrowed path results, projects inaccessible endpoints onto walkable
floor and follows the route from the last accepted position. New input, reset
or focus loss cancels stale movement. Keyboard movement uses the same physical
footprint.

The selected humanoid render supplies 24 whole-body phases per direction and
a dedicated idle pose. Every frame contains the body, costume and equipment;
there is no separately warped limb layer. A cycle covers approximately 1.782
world units. Normal movement is 4.0 units/second and haste is 5.2; turns retain
phase. The [art source](../../scripts/vigil/source/README.md) records attribution and rendering.

Player sight uses the same wall outline as navigation. Rays stop at walls;
projected character and decoration silhouettes prevent the ground mask from
cutting off visible heads or column caps. Low furniture remains visible but
blocks movement. There is no remembered exploration or radial darkness.
The mask is specific to this sample's fixed camera, not a general engine API.

## Ground material and wear

A dedicated 1774×887 flagstone source repeats over 2.4×1.2 world units.
The selected material recipe supplies relief and restrained dielectric
reflectance. A separately mapped periodic secondary texture adds broad colour
variation; placed transparent planes add damp dirt, stone chips, blood and wall
seepage. Those planes are ordinary textured sprites, not new decal commands.

Cool stone fill separates surfaces from tight warm candle pools. Fireballs
carry moving lights and use upright sprite cards for trails and impacts.
The current sample does not use mounted GPU particle emitters for this effect.

## Responsive framing

The camera preserves vertical field of view. Sight overlays and pointer
coordinates share its presentation scale, while the HUD uses the actual
viewport. [Browser checks](validation.md) cover portrait, landscape and
ultrawide sizes; software captures do not establish physical-device frame rates.

## References

The direction draws on Diablo II's [controls](https://classic.battle.net/diablo2exp/basics/controls.shtml)
and [grounded whole-body presentation](https://www.youtube.com/watch?v=iThWJo3Hi-k&t=31),
with an original abbey, characters and artwork. No Blizzard assets are shipped.
The Met's essays on [pilgrimage](https://www.metmuseum.org/essays/pilgrimage-in-medieval-europe)
and [reliquaries](https://www.metmuseum.org/essays/relics-and-reliquaries-in-medieval-christianity)
inform the processional route and devotional details. Artwork credits, original
prompts and licenses remain with the [art sources](../../scripts/vigil/assets/README.md).

# Camera framing v1

Status: additive local-source extension to controllers 0.1.0. No public module
release, engine API change or wire change is implied.

`CameraComposer` owns the tracking history for one shot. It has no engine
imports, target/entity ownership, automatic updates or rendering resources.
Construct with `NewCameraComposer(DefaultCameraFramingOptions())`; configure
options before construction or transactionally with `Configure`. `Options`
returns a value snapshot. Configure and Reset clear velocity history.

`Compose(current, target, viewSize, dt, snap)` works in screen-right/screen-down
coordinates. Positions and visible view extents use the same units. Damping
is the per-axis half-life in seconds, using `1-exp(-ln(2)*dt/halfLife)`;
zero means immediate correction. dt must be finite and in (0,1].

DeadZone and SoftZone are full rectangle sizes normalized to the supplied
projection rectangle, centered on ScreenPosition. ScreenPosition uses [0,1],
with (.5,.5) at the center. DeadZone must fit inside SoftZone; SoftZone must
fit inside the view. A stationary target inside DeadZone needs no correction.
Between the rectangles the camera damps toward the nearest dead-zone boundary.
Beyond SoftZone it corrects enough to retain the real target inside the outer
rectangle, even when that overrides damping or anticipation. A zero-sized
DeadZone follows continuously; zero SoftZone locks framing immediately.

Lookahead estimates velocity from successive accepted target positions, filters
it with LookaheadDamping (another half-life), multiplies by LookaheadTime and
caps the vector length to MaxLookahead. The cap uses camera-plane world units:
pixels for an unscaled 2D map and projected metres for a fixed isometric view.
Use zero LookaheadTime or MaxLookahead to disable anticipation. Never feed
unaccepted movement proposals or sprite-animation offsets into the target.

Pass snap on spawn, teleport or a discontinuous portal transition. It places
the target at ScreenPosition and clears velocity. Reset clears history without
moving the camera; its next ordinary update seeds velocity without assuming
travel from the old target. Invalid options/updates preserve all prior state
and return ErrOptions/ErrCameraInput. Construction may allocate; steady
composition and its Planar adapter allocate no memory.

`Planar.ComposeCamera(composer, current, accepted, width, height, snap)` replaces
Follow for games opting in. It evaluates at 1/60 second, includes FollowOffset,
uses the controller's current zoom and projects isometric targets into camera
right/down axes. It returns the ground-plane focus expected by CameraPosition
and ScreenToGround. Follow retains its existing behavior for other consumers.
Reset the composer as well as input history when replacing a player or shot.

Supply the same projection rectangle used by picking. The samples use their
1280x720 authored logical rectangle. Responsive portrait rendering crops that
rectangle horizontally; this package cannot infer the physical window's aspect
ratio. Their narrow horizontal zones leave room for that crop, but do not
establish arbitrary-aspect framing guarantees. A caller with viewport information
can supply the corresponding visible rectangle instead. Re-evaluate after zoom
or size changes; visible extents are not cached.

This first version implements target composition for 2D and fixed isometric
views. It does not add confinement, occlusion, collision avoidance, perspective
tracking, target groups, virtual-shot selection/blends, shake or render-rate
interpolation. Games retain their current geometry, sight and input policies.

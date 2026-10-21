# Controller presets v1

Status: implemented in local package `controllers` 0.1.0.

The distribution unit is one package and one portable Go module. First-person,
top-down 2D and isometric presets include their input and camera calculations.
Orbit controls and complete genre frameworks are outside this contract.

The module imports no engine internals or generated game bindings. A game owns
its hooks, entities, physics, UI, resources and release. Package calls are
explicit, run once per fixed 60 Hz update, and retain no global game state.
First-person Step reads accepted yaw/pitch and returns displacement plus camera
angles; callers send horizontal velocity to the host, preserving gravity.
Planar Step proposes movement and Follow uses the accepted position.
The optional [camera composer](camera-framing-v1.md) adds screen-axis damping,
dead/soft zones and bounded lookahead through Planar.ComposeCamera. It does not
change Follow or movement behavior for existing consumers.

Input storage is fixed. Controls include independent key aliases; repeated
keydown is idempotent. Each touch slot owns one nonzero finger until release
or reset. Mouse displacement is consumed once. UI hit tests apply to new
gestures, while move/end/cancel always reach an owned finger. Focus loss, pause
and player/view replacement reset keys, mouse and touch; the game also stops
the old body and releases cursor capture. Controllers never destroy entities.

Options are validated transactionally. Start from a preset's default options;
explicit zero values are not replaced with defaults. Construction can allocate;
steady input, movement, following and projection do not allocate. A configured
controller must not be copied with active input. Configure resets input, and
games must update their overlay/camera presentation after changing settings.

Top-down coordinates use screen-down Y. Isometric movement and Z=0 picking use
the preset's fixed yaw and negative pitch. Picking uses logical viewport size
and the current camera focus; it does not infer CSS size, elevation or an
independently modified camera transform. Zoom is scale in 2D and orthographic
height in isometric. Collision response and route search stay with the game.

This package changes no engine API, wire protocol, SDK manifest, generated-file
ownership or package archive format. Consumers declare `controllers` normally
and may use a source folder or a checked archive. No private source is needed.

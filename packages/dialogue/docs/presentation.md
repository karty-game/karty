# Presentation model

An engine-independent Go model for visual novels. It owns a speaker, current
line, choices, scene/portrait IDs, a bounded 24-line conversation log and audio
preferences. The game owns the model; UI components display it and invoke game
callbacks. It does not run scripts or access storage.

`Line(id, text)` splits `Speaker: dialogue` at the first colon. The reserved
`Narrator` speaker becomes an empty name; `Narration()` identifies prose. Binary
checkpoints normalize that name in the current line and history on restore. `SetChoices`
accepts at most 32 choices with unique IDs from 0 to 65,535 and enabled flags. Complete lines
appear immediately by default. Call `Reveal()` after `Line` for optional paced
text, then `StepReveal()` once per 60 Hz update. `DisplayText()` returns a
Unicode-safe character prefix with short punctuation pauses. `RevealCharacters()`
returns its Unicode scalar count for a renderer that retains full-line layout.
The novel UI binds the complete `Text` plus that independent count, keeping
wrapping and shaping fixed while characters appear. `DisplayWords()` remains
available for custom older interfaces that prefer complete words. `FinishReveal()` completes
the line without advancing the script. The game decides when the next click
continues. `AdvanceLabel()` supplies icon prompts (`»` for reveal, `▾` for advance, `…`
for a pause). `Paused` disables the advance action during authored pauses. These transient
fields are excluded from checkpoints. `Save`/`Restore` use a bounded, versioned binary snapshot;
restore validates UTF-8 and UI field bounds into temporary state before replacing the model. Scroll/log
visibility is transient. `SaveRun` excludes audio preferences and transient status;
`RestoreRun` preserves the receiving model's current audio preferences. It also
accepts binary full snapshots, discarding their preferences and status. These
are presentation codecs; they impose no format on engine storage.

The model is included alongside the compiler, runtime and UI; it can also be
used with another dialogue runner or a custom interface.

Run `mise run test-dialogue-packages` from the CLI repository root.

See the [binary runtime v2 decision](binary-runtime-v2.md) for resource layout and candidate save compatibility.

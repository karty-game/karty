# Visual novel UI

Reusable KartUI components and a slotted `NovelFrame` layout. No engine-specific
Go bindings are distributed: the consuming game generates its own bindings.
These components ship with the dialogue model, runtime and compiler in the same
`dialogue` package. No separate UI import is needed.

`NovelDialogue` separates anonymous narration from speech. Narration uses a
warm prose face without a nameplate; speech has a character name and regular
body text. Both bind the complete line as layout text and pass
`Model.RevealCharacters()` to the label's `reveal` attribute. The renderer lays
out and shapes that complete line, then reveals Unicode characters without
changing wrapping or moving existing glyphs. This UI requires SDK 0.0.17 with
`UI.Reveal`; the model/runtime alone still work with custom interfaces.
A compact 48×44 icon button shows `»` while revealing and `▾` when ready; the
first activation finishes a line and the next advances. `Paused` disables that
action during authored beats.
`NovelHistory` renders a scrollable conversation log. Both receive
the game's `*dialogue.Model` and callbacks. The game keeps its model pointer
stable and invalidates the root UI after mutation. `NovelFrame` offers `scene`,
`portrait`, `heading`, `toolbar`, default dialogue and `overlay` slots. It adapts
to narrow portrait screens. Pass `onBack` to provide Escape/back behavior.

The consumer supplies `body` for speech, `display` for names, and `mono` for
narration, plus its own image theme. The existing third font slot is used as
a prose role here; Last Train supplies an italic serif face, not a monospace
face. History uses the same prose/speech distinction.
Artwork, audio, save buttons and story commands stay with the game. See
[Last Train](../../../samples/last-train/README.md) for a complete consumer.

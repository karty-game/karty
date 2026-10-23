# Package-generated Go adapters v1

Packages may generate typed adapters for a game's engine bindings through an
explicit build processor. The package owns its implementation and templates;
the CLI supplies generic confined output handling. No engine-specific package
knowledge is required.

The existing bounded KPR1 request/response envelope remains unchanged. A
selected processor can read authored inputs such as the project's `go.mod`
and return files with paths beginning `go/`. These are Go source outputs:
`go/controller/player.go` from `controllers-3d.sdk` is published at
`.karty/processed/code/controllers-3d.sdk/controller/player.go`. The game's
module can import that package alongside its generated `.karty/engine` bindings.

The CLI parses and formats source, requires ordinary non-test `.go` files and
consistent package declarations within each directory, and adds a generated
ownership header. Module/workspace files and paths escaping the processor's
namespace are rejected. Existing response size/file bounds and output ownership
checks apply. Other paths retain their existing embedded-resource semantics.

All processor code, resources and generated assets are staged together. Invalid
source or any failed processor leaves the previous complete generated output
intact. Removing a processor removes its old generated source during the same
owned publication. User-owned `src/` files and immutable package archives are
never modified by adapter generation.

Import, export and restore do not execute processors. A game opts in through
`[[build.processors]]`; normal builds and development previews then regenerate
the adapter. The package version and archive SHA-256 pin the generator and its
templates. Changes to controller policy therefore require a package release,
while generic engine lifecycle, input, camera and physics contracts remain
independent.

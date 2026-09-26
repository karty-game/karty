# Project builds and staging

Read the root instructions and `docs/repository-split.md`. This package owns
compiler invocation, generated project files, cartridge/asset staging, and the
browser shell in embedded `templates/index.html.tmpl` and
`templates/launcher.js.tmpl`.

- Preserve exact SDK selection, verified toolchains, independent-project
  compilation (`GOWORK=off`), and compiler-specific WASI reactor flags.
- Never write generated infrastructure into the user-owned `src/` directory.
  Legacy generated registration shims may be removed only after an explicit
  user-owned registration export replaces them. Derived files retain explicit
  generated warnings and accurate template/regeneration instructions.
- Preserve manifest bounds, content-addressed path derivation, and native/web
  staging parity.
- Shell changes must retain memory-range checks, event-buffer copying, frame
  identity, lifecycle ordering, and synchronous batch submission semantics.
- Edit template sources rather than generated project output. Add tests for
  ownership collisions, environment handling, and any changed shell behavior.

Run `mise run test` and `mise run smoke`; shell changes also require
`mise run check-web`, which executes actual staged JavaScript and TinyGo WASM.
That check is not a browser visual review. Escalate changes requiring project
file migration, broader deletion, or a new SDK/toolchain compatibility choice.

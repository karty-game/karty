# Karty website

Sources and build tools for the public Samples, Guides, API and Design site.
The CLI implementation lives in the repository's `cmd/` and `internal/` folders.

## Source layout

| Folder                                       | Contents                                                                |
| -------------------------------------------- | ----------------------------------------------------------------------- |
| [`content/guides/`](content/guides/)         | Guide introduction, game paths and shared chapters                      |
| [`content/api/`](content/api/)               | Grouped API reference pages                                             |
| [`content/design/`](content/design/)         | Why Karty, architecture, SDK/toolchains, assets and packages            |
| [`assets/`](assets/)                         | Images, diagrams and capture records                                    |
| [`templates/`](templates/)                   | Documentation, gallery and local sample HTML                            |
| [`styles/`](styles/)                         | Shared handwritten CSS                                                  |
| [`cmd/`](cmd/)                               | Documentation generator and sample-site commands                        |
| [`scripts/`](scripts/)                       | Browser checks, sample captures and Materialize illustration generation |
| [`internal/generator/`](internal/generator/) | Go Markdown renderer and confined output writer                         |

`files.go` embeds the content, templates and assets. `theme.go` embeds the CSS.
Build tools share the repository's pinned Go and Node toolchains and module
dependencies. The CLI does not import website packages; `boundary_test.go`
checks that rule. Go also restricts the renderer to tools inside `website/`.

Sample projects, descriptions and preview images live in the repository's
`samples/` directory. Website tooling reads their metadata and invokes the CLI
to build games. Set `hidden = true` in a sample's `preview.toml` to omit its
public gallery card while retaining builds and direct demo URLs.

Templates emitted into game builds belong to `internal/build/templates/`.
The public site's templates belong to `website/templates/`.

## Preview and generate

Run tasks from the repository root:

```sh
mise run docs
```

Open `http://127.0.0.1:8081/`. The task generates the documentation and gallery
in `dist/samples-site/`, includes complete existing sample builds, and serves
the result through `karty serve`. Restart it after editing sources.
Use `KARTY_DOCS_PORT=8091 mise run docs` for another port.

`mise run preview-docs` refreshes that site without starting a server.
`mise run generate-guide` writes just the documentation into sibling
`dist/guide/`, `dist/technical/` and `dist/design/` directories. The public API
section uses the existing `technical/` URL.

For a complete site with playable samples, run
`mise run build-samples -- --cdn`. The Pages assembly task validates its input
before replacing a published preview.

## Edit content

Guides contain setup and complete game examples. API pages describe methods,
fields and constraints, with short snippets and links to the guides. Related
APIs share six pages, with second-level headings for section links. Design
pages explain the framework's decisions.

Edit layout in `templates/` and styling in `styles/theme.css`. The renderer
supports headings, paragraphs, one-level bullets, fenced code, inline code,
links, tables and local images. Its supported Mermaid chain renders as offline
SVG. Raw HTML is escaped. Generated HTML and Markdown downloads use the same
sources and need no remote scripts, fonts or image hosting.

Go reference snippets omit package and import declarations. Tests supply
`engine` and `assets` imports and type-check the displayed functions against
the selected SDK. The particle excerpt compiles within a room fixture. Keep
candidate requirements explicit and use the project's `.karty/docs/` for exact
SDK contracts.

## Images and diagrams

Architecture and coordinate diagrams are handwritten SVG. `design-room.png`
is an AI-generated concept illustration; its prompt and checksum are in
`design-room.art.json`.

The Materialize comparison embeds five unedited native map outputs. Regenerate
it with the executable selected by the SDK:

```sh
KARTY_DOCS_MATERIALIZE=/path/to/pinned/materialize-cli mise run generate-design-materialize
```

The script processes Hangar's brick source with the `brick` preset and records
hashes, arguments and graphics backend in `assets/design-materialize.generation.json`.
Game builds also normalize materials to 256-pixel tiles before generation.

Sample preview images are copied from their projects. Capture records retain
the original source paths, hashes, inputs and backend limits from capture time.
The 2D and isometric captures qualify their recorded artifacts. They do not
establish execution of later examples. `particle-spark.png` is an unchanged
copy of Particle lab's `levels/lab/textures/lab-fleck.png`.

## Check changes

```sh
mise run check-website
mise run build
```

The website checks cover its dependency boundary, generation, links and
anchors, redirects, downloads, safe output paths, sample-site assembly, Go
examples and authoring data. The repository's `mise run test` includes them.
The selected SDK is installed by `mise run test`; its examples include Camera 2D.

After preparing a site, run `mise run check-docs-web`. Set `KARTY_DOCS_SITE` to
another prepared site directory. Chromium checks desktop/mobile navigation,
section links, redirects, decoded images, Markdown downloads and page overflow.
Screenshots and HTML hashes go to `dist/validation/docs-web/`.

Type checking, actual WASM execution and game graphics have separate checks.
Local review notes in `validation/` are ignored. Public content, assets and
handwritten generators remain in Git. Root `docs/`, `spike/`, `spikes/` and
`concept-art/` are also ignored investigation folders.

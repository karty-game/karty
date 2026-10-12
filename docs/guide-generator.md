# Game-making guide

Review the guide locally from the CLI repository root:

```sh
mise run docs
```

Open `http://127.0.0.1:8081/guide/`. The task regenerates
`dist/samples-site/guide/`, refreshes the sample gallery and serves the site on localhost. Press Ctrl+C to
stop it; restart after editing the guide. For another port, run
`KARTY_DOCS_PORT=8091 mise run docs`.

The gallery and guide use the same handwritten theme in `internal/sitetheme/`.
Existing sample builds work at the same address. For playable sample links,
build them once with `mise run build-samples -- --cdn`.

To generate files without starting a server:

```sh
mise run generate-guide
```

This writes `dist/guide/index.html` with the CLI introduction and a choice of
2D, first-person, or isometric game guides. Each path has its own HTML page and
downloadable Markdown. The 3D paths share pages for rooms, materials, objects,
lighting, and interactions. All paths share media, UI, and distribution guides. The optional complete
guide lives at `complete.html` and `guide.md`.

An optional "Why Karty?" page explains the creator's goals for the framework.

The 2D path covers sprites, vectors, and text on the logical canvas. KartUI
menus, HUDs, and layouts are a shared topic for all game paths.

The images and their capture records live under `dist/guide/assets/`.
Open the HTML locally to review it. Its sample links work when the guide sits
beside the sample folders; the samples builder puts it at `guide/`.

To choose another destination:

```sh
mise exec -- go run ./cmd/karty-guide -output /tmp/karty-guide
```

Edit the landing introduction in `internal/guide/landing.md`, game paths in
`internal/guide/tracks/`, and shared topics in `internal/guide/chapters/`. Keep each chapter
focused on one playable step: preview, room, material, object, light,
interaction, media, menus, or sharing. Use short paragraphs, direct verbs, and concrete
examples. Explain the next action before introducing more options.

The generator uses embedded handwritten Markdown and standard Go packages.
It needs no SDK installation, engine checkout, credentials, or network access.
Its HTML renderer supports headings, paragraphs, one-level bullets, fenced
code, inline code, links, simple tables, and standalone images from the embedded `assets/`
directory. It renders a small Mermaid `flowchart TD` chain as offline SVG.
Other Mermaid graphs and richer Markdown are outside its scope. Raw HTML is
escaped. Keep source examples in the chapters so the downloadable guide and
website stay aligned.

Four JPEG images are exact copies of the samples' `preview.jpg` files.
Their original capture records live beside those sample images; the generator
includes copies with the guide. After updating
a sample preview, refresh its copy in `internal/guide/assets/`. The animated
`live-light.svg` illustrates a changing lamp; it is not a runtime capture.
It stops moving when the reader requests reduced motion. The generated guide
needs no remote scripts, fonts, or image hosting.

The isometric image is a separate capture of the already-built Roman court
sample. Its record is `internal/guide/assets/isometric.capture.json`, with the
exact host and cartridge hashes, inputs, software graphics backend, and capture
limits. It does not claim execution of the guide's new orbit controller.

The 2D image captures the exact runnable sprite/vector/text scene in `tracks/2d.md`,
built with public SDK 0.0.11. `2d-scene.capture.json` records its source and artifact
hashes. The downloadable `player.png` is the small sprite from that SDK's game
template. This capture checks browser drawing and keyboard movement, with no
physical-device or focus-loss qualification.

Generation tests check stable output, navigation links, safe destinations,
world expansion and compilation, texture references, and Go example type checking
against the installed public SDK selected by this CLI. They also compile the
menu and pause components with their shared KartUI layout.
Sample build, WASM execution, and graphics checks remain separate.

# Web delivery

Web delivery version 1 uses a service worker to download the rest of a game after
initialization, reuse unchanged assets across releases and support offline play.
The CLI stages the worker and browser shell with each web build. Development
mode keeps its existing live-update flow.

The server handles HTTP compression normally. Delivery does not require Brotli
sidecars, a custom decompressor or special compression configuration. Cache
identities and integrity checks describe the decoded bytes, whether the server
sends gzip, Brotli or an uncompressed response.

## Release identity

Each build writes an immutable JSON manifest at
`karty-releases/<sha256>.json`. The SHA-256 identifies the exact manifest bytes.
A small `karty-latest.json` pointer identifies the release to launch. Both use
format version 1.

The manifest lists the browser shell, runtime glue, launcher, host, cartridge,
optional debugging scripts and every staged KLD, audio and video file. Each entry
records its URL, decoded SHA-256, decoded size, content type and startup or
background role. Assets embedded inside KLD files travel with those files.

Local files have exact sizes. Signed shared-host metadata currently provides a
digest without a decoded size; that entry uses size zero. The worker still
requires its digest to match and enforces the host's 128 MiB decoded limit. A
shared host remains shared: making a release manifest does not copy it into each
game distribution.

Downloads enter the persistent content cache only after their decoded length
and digest pass validation. Cache keys use content digests, so an unchanged file
is reused even when a new manifest lists it. A partial or corrupt response never
becomes a valid cached asset. The worker hashes streams incrementally to avoid
buffering entire audio and video files in JavaScript.

## Starting and updating

At an online launch, the browser checks `karty-latest.json` with HTTP caching
bypassed. It verifies the manifest and prepares the release's startup files
before starting the game. Once the server confirms a newer release, a failed
manifest, integrity check or required startup download produces an update error;
it does not silently launch an older cached game.

A running session stays pinned to the release it selected. Its cartridge,
levels and assets keep their original identities when another tab starts a
newer release. Pins survive service-worker restarts. Checking for an update does
not replace the game underneath the player; the update applies on a new launch.
Worker-code updates also wait for existing sessions to finish. The restart
control can activate an available worker update explicitly; other open sessions
retain their release pins.

When the network is unavailable, a new session can use the last complete
downloaded release. The previous complete release remains available while its
replacement is incomplete. Completing a download in an older open tab does not
replace a newer complete release as the default for offline play.

## Background downloads

After game initialization, the browser starts downloading remaining assets in
the background. Foreground requests take priority; an interrupted background
download can be resumed later. Content-addressed KLD files include levels,
chunks and their packed assets, so later visits can reuse them without a
network round trip.

The shell reports progress and exposes pause and resume controls. Automatic
downloads pause when the browser requests data saving or reports a slow 2G
connection. A deliberate resume can override that policy.

Offline readiness means every file required by the selected release is present
and verified. Partial downloads, storage errors or insufficient quota keep the
release incomplete. A background cache failure leaves online game access
available and reports why offline downloading paused.

## Cache lifetime and browser support

Each game's cache is scoped to its own deployment URL. Cleanup retains files
referenced by active sessions, the selected release and the last complete
offline release. Files shared by those releases remain stored once. Obsolete
files and release metadata can then be removed without clearing another game's
cache.

Offline play requires service-worker and CacheStorage support in a secure
context, normally HTTPS or localhost. Registration restrictions fall back to
ordinary online loading. Browser storage is subject to quota limits and
eviction; a previous successful download is not a permanent storage guarantee.
The shell checks current cache contents before reporting offline readiness.

## Publishing a release

Upload the game files and immutable release manifest first. Publish
`karty-latest.json` last, after those files are available. This prevents the
discovery pointer from announcing an incomplete deployment. Serve the discovery
pointer and root HTML with revalidation or no-store behavior; the worker also
bypasses HTTP caches when checking for a release.

Keep previously published hash-named content files available while older
sessions may still request them. An open old tab might not have downloaded every
level before the update. Its release pin prevents switching to a different
level, but cannot recover an old file that the server has deleted and the browser
never cached. Build output describes the current release; deployment tooling is
responsible for retaining older published content when needed.

Mutable startup paths also need coherent deployment. Keep old sessions' startup
files cached and publish the new files before their pointer. The complete old
offline release is retained until the new release is fully downloaded.

## Validation

Run `mise run check-web-delivery` after installing the browser tooling with
`mise run install-browser`. It creates synthetic releases through the production
staging code and executes update, prefetch, cancellation and offline-cache checks
in real Chromium. Reports go to `dist/validation/web-delivery/results.json`.

This validates browser delivery behavior. The fixture's small WASM files do not
establish game execution, rendering, device performance or physical audio.
`mise run check-web` separately executes the staged JavaScript and Go/TinyGo
cartridges.

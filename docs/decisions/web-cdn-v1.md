# Shared web runtime, version 1

Status: CLI option, 2026-10-12. No SDK or wire format changes.

`karty build --target web` keeps the host in `dist/web`. Add `--cdn <host-url>`
to load the host from a shared mirror instead:

```sh
karty build --target web --cdn https://games.example/runtime/0.0.11/host.wasm
```

The mirror must serve the exact `karty-host-web.wasm` from the project's pinned
SDK release. The CLI verifies the release manifest signature and embeds its
host SHA-256 in the launcher. The browser checks that digest before compiling
or running the host. It reads at most 128 MiB. A failed download or checksum
shows the startup error and retry button. There is no bundled fallback.

The small matching `wasm_exec.js` stays local. A `--host` override must match
the signed release checksum. Custom hosts use an ordinary build. CDN mode
rejects native targets and unpublished SDK versions. Its output summary counts
local files and names the remote dependency.

## Sample hosting

`karty-samples build --cdn` downloads and verifies each pinned release host,
then writes it once under `runtime/<sdk-version>/<sha256>.wasm`. Every sample
using that release loads `../runtime/<sdk-version>/<sha256>.wasm`. The site
publisher verifies the shared files again before assembly. Preview paths keep
the runtime beside their sample directories.

Other hosts can use an absolute HTTPS URL or a relative path in the current
directory or one parent directory. Publish the mirror with the game. A mirror
on another origin needs CORS access. Serve over HTTPS or localhost so the
browser can verify the digest.

## Why the browser uses a mirror

On 2026-10-12, Chromium rejected both the `sdk-v0.0.11` GitHub release download
and the public release-asset API download. The release redirect lacked
`Access-Control-Allow-Origin`. The API redirect had that header, but its final
`release-assets.githubusercontent.com` response did not. We tested the real
requests from the `https://karty-game.github.io` origin.

GitHub Pages therefore serves a shared copy of the verified release bytes.
The CLI rejects direct GitHub release downloads and their expiring redirects.
It never resolves a mutable `latest` release.

GitHub documents the API's binary download redirects in its
[release asset reference](https://docs.github.com/en/rest/releases/assets#get-a-release-asset).

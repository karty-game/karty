# Remote package archives, version 1

A project can pin a package release archive over HTTPS. This extends CLI package
restoration while retaining the existing `.kpkg` archive format, package
namespace, source confinement and complete content validation. It changes no
engine API, world schema or cartridge wire format.

## Declaration

Set `archive` to a versioned HTTPS `.kpkg` URL and `sha256` to the exact 64-character
lowercase hexadecimal SHA-256 digest of those archive bytes:

```toml
[packages.warehouse-kit]
archive = "https://example.com/releases/warehouse-kit-0.1.0.kpkg"
sha256 = "<replace with the publisher's 64-character lowercase SHA-256>"
```

The URL and checksum above illustrate the fields; use an available release URL
and its actual digest. A dependency declares exactly one `archive` or `folder`.
Local archive paths retain their existing confinement and checksum rules.

Remote sources require HTTPS. Credentials, fragments and unsupported URL forms
are rejected. Downloads remain bounded by the package archive size limit, and
redirects must preserve the HTTPS policy. The release URL and checksum identify
a fixed dependency; restoration never resolves a newer release automatically.

## Restoration and cache

`karty package restore`, builds and development previews resolve the same package
declarations. A first remote restore downloads the archive, verifies its SHA-256
and performs complete archive validation before exposing package contents.
Restoration checks cached bytes against the declared checksum before reuse.
A verified cached archive allows later restoration without another download;
an unavailable release with no valid cached copy is an error.

Decoded packages remain under `.karty/packages/`. Existing archive bounds,
duplicate-entry checks, path confinement and symlink restrictions continue to
apply to remote archives. Downloads never execute package scripts. Existing
explicitly selected build processors retain their ordinary build behavior.

This decision originally limited `karty package import` to local folders and
local `.kpkg` files, with remote pins configured in `karty.toml`. The
[named package imports v1 decision](named-package-imports-v1.html) supersedes
that command limitation: a catalog name now resolves to the same archive URL
and checksum declaration. Local imports retain their behavior. Direct URL
imports, automatic updates and transitive resource dependencies remain outside
these decisions.

## Starter integration

A starter can declare a versioned release URL and checksum. The CLI restores
the package through the ordinary resolver; artwork and package source are owned
by the package repository. Package documentation is available in the restored
`.karty/packages/<name>/` directory.

The FPS starter pins the published FPS Kit archive at
`https://github.com/karty-game/karty-packages/releases/download/fps-kit-v0.0.1/fps-kit-0.0.1.kpkg`.
Its sources and independent releases live in
[`karty-packages`](https://github.com/karty-game/karty-packages).

Older CLIs reject remote archive URLs. Projects declaring them require a CLI
implementing this decision. Existing local folder and archive dependencies keep
their behavior.

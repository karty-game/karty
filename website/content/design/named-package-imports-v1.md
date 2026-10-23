# Named package imports, version 1

The CLI can import a published package by name using the official package
catalog. Import resolves the selected version once and records its immutable
archive URL and SHA-256 using the existing
[remote package archive declaration](remote-package-archives-v1.html). This is
a CLI authoring feature. It keeps the existing package, world and cartridge
formats.

## Commands and resolution

Run these commands from a game project:

```sh
# Resolve the latest stable version at import time.
karty package import fps-kit

# Select this exact version when published in the catalog.
karty package import fps-kit@0.0.1
```

The CLI reads
`https://karty-game.github.io/karty-packages/master.json`. A bare name selects
that entry's latest stable version. `name@version` requires an exact matching
version; an unknown name, unavailable version or package with no published
version is an error. The catalog can reference packages maintained in different
GitHub repositories. This version adds no custom catalog option.

Existing local paths retain priority. An existing folder named `fps-kit` is
imported locally by `karty package import fps-kit`. Use `./fps-kit` to explicitly
select a local path, including when diagnosing a missing folder. Local `.kpkg`
archive imports and their project-local copies keep their existing behavior.

## Validation and project changes

Catalog resolution supplies the archive URL and SHA-256; it does not execute
package code. The CLI downloads a bounded HTTPS archive and verifies its
checksum. It then validates the complete archive, package name and version,
exports and compatibility with the project before committing
changes to authored project files. Invalid catalog data, mismatched identity,
corrupt contents and incompatible packages fail the import. Existing archive
path confinement, symlink restrictions and content bounds remain in force.

The dependency stored in `karty.toml` contains the resolved `archive` URL and
`sha256`, rather than a floating name or latest-version selector. The project
therefore retains the selected bytes even when the catalog gains new versions.
Code packages retain their ordinary Go module integration.

Upgrade an installed archive dependency to the latest stable catalog release:

```sh
# Update to the latest stable release available now.
karty package upgrade fps-kit

# Replace the dependency with this exact published version.
karty package import --replace fps-kit@0.0.1
```

Upgrade accepts one installed package name without `@version`. It always resolves
that name through the catalog, bypassing same-named local folders. Missing
dependencies and linked development folders are rejected; changing a link to a
release requires an explicit `import --replace` instead. Checksum, identity,
inventory, compatibility and transactional project-update checks remain the same
as import. Only an explicit upgrade or replacement changes a saved archive pin.

Review and commit the resulting manifest and any Go module changes as part of
the project's dependency update. The example version is illustrative; these
commands require that release to exist in the catalog.

## Restore and compatibility

`karty package restore`, builds and development previews use the recorded URL
and checksum without contacting the catalog. An existing verified cached
archive supports offline restoration; a first restore still needs the release
archive to be reachable. Cached bytes are verified against the recorded digest
before reuse. Catalog availability is needed when importing or explicitly
upgrading or replacing a dependency by name, not when restoring its saved pin.

Named imports require a CLI implementing this decision. Older CLIs with remote
archive support can restore the resulting URL-and-checksum declaration without
understanding the original named command. Local folder and archive workflows
remain available. Automatic updates, direct URL-import commands and transitive
resource dependencies are outside this version.

package packages

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

// DefaultCatalogURL is the public discovery index used only by named imports.
const DefaultCatalogURL = "https://karty-game.github.io/karty-packages/master.json"

const maximumCatalogBytes = 16 << 20

var catalogNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,47}$`)

type catalogSelection struct {
	Name    string
	Version string
	Source  Source
}

type catalogDocument struct {
	FormatVersion int              `json:"format_version"` //nolint:tagliatelle // Published catalog schema uses snake_case.
	Packages      []catalogPackage `json:"packages"`
}

type catalogPackage struct {
	Name     string           `json:"name"`
	Latest   *string          `json:"latest"`
	Versions []catalogVersion `json:"versions"`
}

type catalogVersion struct {
	Version string `json:"version"`
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
}

// Existing local inputs win over catalog names. Missing explicit paths remain
// local errors, so a misspelled ./folder never triggers a network request.
func catalogImportReference(input string) (catalogSelection, bool, error) {
	if filepath.IsAbs(input) || strings.ContainsAny(input, `/\`) || strings.HasPrefix(input, ".") || strings.HasSuffix(input, ".kpkg") {
		return catalogSelection{}, false, nil
	}

	if _, err := os.Lstat(input); err == nil {
		return catalogSelection{}, false, nil
	} else if !os.IsNotExist(err) {
		return catalogSelection{}, false, err
	}

	name, version, pinned := strings.Cut(input, "@")
	if !catalogNamePattern.MatchString(name) || (pinned && !validCatalogVersion(version)) {
		return catalogSelection{}, false, fmt.Errorf("%w: expected a package name or name@x.y.z, or an explicit local path", ErrPackage)
	}

	return catalogSelection{Name: name, Version: version}, true, nil
}

func validCatalogVersion(version string) bool {
	return len(version) <= 64 && semver.Canonical("v"+version) == "v"+version
}

func resolveCatalogImport(ctx context.Context, selected catalogSelection, address string, client *http.Client) (catalogSelection, error) {
	parsed, err := url.Parse(address)
	if err != nil || !safeArchiveURL(parsed) || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(address, "#") {
		return selected, fmt.Errorf("%w: package catalog requires an HTTPS URL", ErrPackage)
	}

	data, err := downloadRemoteArchive(ctx, client, address, maximumCatalogBytes)
	if err != nil {
		return selected, fmt.Errorf("fetch package catalog: %w", err)
	}

	var catalog catalogDocument
	// Descriptive metadata is deliberately ignored. Install decisions use only
	// the versioned format, package identity, release URL and content checksum.
	if err := json.Unmarshal(data, &catalog); err != nil {
		return selected, fmt.Errorf("%w: decode package catalog: %w", ErrPackage, err)
	}

	if err := validateCatalog(catalog); err != nil {
		return selected, err
	}

	for _, entry := range catalog.Packages {
		if entry.Name != selected.Name {
			continue
		}

		if selected.Version == "" {
			if entry.Latest == nil {
				return selected, fmt.Errorf("%w: package %s has no published release", ErrPackage, selected.Name)
			}

			selected.Version = *entry.Latest
		}

		for _, version := range entry.Versions {
			if version.Version == selected.Version {
				selected.Source = Source{Archive: version.Archive, SHA256: version.SHA256}

				return selected, nil
			}
		}

		return selected, fmt.Errorf("%w: package %s has no published version %s", ErrPackage, selected.Name, selected.Version)
	}

	return selected, fmt.Errorf("%w: package %s is not in the catalog", ErrPackage, selected.Name)
}

func validateCatalog(catalog catalogDocument) error {
	if catalog.FormatVersion != 1 || catalog.Packages == nil || len(catalog.Packages) > 1024 {
		return fmt.Errorf("%w: unsupported package catalog format or bounds", ErrPackage)
	}

	names := make(map[string]bool)
	count := 0

	for _, entry := range catalog.Packages {
		if !catalogNamePattern.MatchString(entry.Name) || names[entry.Name] || entry.Versions == nil {
			return fmt.Errorf("%w: invalid or duplicate catalog package %s", ErrPackage, entry.Name)
		}

		names[entry.Name] = true
		versions := make(map[string]bool)
		latest := ""

		for _, version := range entry.Versions {
			count++

			digest, err := hex.DecodeString(version.SHA256)
			if count > 16384 || !validCatalogVersion(version.Version) || versions[version.Version] || err != nil || len(digest) != 32 ||
				strings.ToLower(version.SHA256) != version.SHA256 ||
				!validRemoteArchiveSource(version.Archive) {
				return fmt.Errorf("%w: invalid or duplicate catalog release for %s", ErrPackage, entry.Name)
			}

			versions[version.Version] = true
			if semver.Prerelease("v"+version.Version) == "" && (latest == "" || semver.Compare("v"+version.Version, "v"+latest) > 0) {
				latest = version.Version
			}
		}

		if (entry.Latest == nil && latest != "") || (entry.Latest != nil && (*entry.Latest == "" || *entry.Latest != latest)) {
			return fmt.Errorf("%w: catalog latest version is invalid for %s", ErrPackage, entry.Name)
		}
	}

	return nil
}

func openCatalogImport(
	ctx context.Context,
	root *os.Root,
	project string,
	selected catalogSelection,
	address string,
	client *http.Client,
) (catalogSelection, *os.Root, error) {
	selected, err := resolveCatalogImport(ctx, selected, address, client)
	if err != nil {
		return selected, nil, err
	}

	if err := MakeDirectories(root, ".karty/packages/.store"); err != nil {
		return selected, nil, err
	}

	_, _, packageRoot, err := resolveSource(ctx, root, project, selected.Source, client)

	return selected, packageRoot, err
}

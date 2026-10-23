package packages

import (
	"context"
	"fmt"
	"net/http"
)

// Upgrade replaces an existing archived dependency with its latest stable
// catalog release. The resolved archive and checksum remain exact project pins.
func Upgrade(ctx context.Context, project, name string) (Manifest, error) {
	return upgradeWithCatalog(ctx, project, name, DefaultCatalogURL, nil)
}

func upgradeWithCatalog(ctx context.Context, project, name, catalogURL string, client *http.Client) (Manifest, error) {
	return importPackage(ctx, project, importRequest{input: name, replace: true, upgrade: true}, catalogURL, client)
}

type importRequest struct {
	input   string
	replace bool
	upgrade bool
}

func (request importRequest) selection(sources map[string]Source) (catalogSelection, bool, error) {
	if !request.upgrade {
		return catalogImportReference(request.input)
	}

	name := request.input
	if !catalogNamePattern.MatchString(name) {
		return catalogSelection{}, false, fmt.Errorf("%w: upgrade expects a package name without a version or path", ErrPackage)
	}

	source, exists := sources[name]
	if !exists {
		return catalogSelection{}, false, fmt.Errorf("%w: package %s is not installed; use karty package import %s", ErrPackage, name, name)
	}

	if source.Folder != "" {
		return catalogSelection{}, false, fmt.Errorf(
			"%w: package %s uses a development folder; use karty package import --replace %s@<version> to switch to a catalog release",
			ErrPackage, name, name,
		)
	}

	return catalogSelection{Name: name}, true, nil
}

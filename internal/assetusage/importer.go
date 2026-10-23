package assetusage

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	errGeneratedGoImport staticError = "invalid generated Go import"
	errGeneratedGoCycle  staticError = "generated Go import cycle"
)

type packageImporter struct {
	assetsPath string
	assets     *types.Package
	uiPath     string
	ui         *types.Package
	enginePath string
	engine     *types.Package
	fallback   types.Importer

	directory, generatedPrefix string
	fileSet                    *token.FileSet
	context                    build.Context
	info                       *types.Info
	generated                  map[string]*types.Package
	loading                    map[string]bool
	generatedFiles             []*ast.File
}

func (resolver *packageImporter) Import(path string) (*types.Package, error) {
	if resolver.assets != nil && path == resolver.assetsPath {
		return resolver.assets, nil
	}

	if resolver.ui != nil && path == resolver.uiPath {
		return resolver.ui, nil
	}

	if path == resolver.enginePath {
		return resolver.engine, nil
	}

	if relative, generated := strings.CutPrefix(path, resolver.generatedPrefix); generated {
		return resolver.importGenerated(path, relative)
	}

	return resolver.fallback.Import(path)
}

func (resolver *packageImporter) importGenerated(path, relative string) (*types.Package, error) {
	if !fs.ValidPath(relative) || relative == "." || strings.ContainsAny(relative, "\\:") {
		return nil, fmt.Errorf("%w %q", errGeneratedGoImport, path)
	}

	if imported := resolver.generated[path]; imported != nil {
		return imported, nil
	}

	if resolver.loading[path] {
		return nil, fmt.Errorf("%w involving %q", errGeneratedGoCycle, path)
	}

	resolver.loading[path] = true
	defer delete(resolver.loading, path)

	directory := filepath.Join(resolver.directory, ".karty", "processed", "code", filepath.FromSlash(relative))

	files, err := parsePackageFiles(resolver.fileSet, directory, resolver.context)
	if err != nil {
		return nil, err
	}

	configuration := &types.Config{Importer: resolver}

	imported, err := configuration.Check(path, resolver.fileSet, files, resolver.info)
	if err != nil {
		return nil, err
	}

	resolver.generated[path] = imported
	resolver.generatedFiles = append(resolver.generatedFiles, files...)

	return imported, nil
}

// Package assetusage finds statically referenced generated assets in cartridge source.
package assetusage

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty-ui/codegen"
	"github.com/karty-game/karty/internal/project"
)

const (
	ReasonTypedReference = "typed reference"
	ReasonStaticName     = "static logical name"
	ReasonDynamicLookup  = "dynamic texture lookup retains the project texture scope"
	ReasonAnalysisFailed = "type analysis was incomplete; retaining the project texture scope"
)

type staticError string

func (err staticError) Error() string { return string(err) }

const errNoBuildableFiles staticError = "package contains no buildable Go files"

// Result describes the conservative live texture set.
type Result struct {
	Live       map[string]string
	KeepAll    bool
	Diagnostic string
}

// Analyze type-checks the generated engine package together with project source.
// An analysis uncertainty retains all assets instead of risking a false strip.
func Analyze(directory, modulePath string, names []string) Result {
	known := make(map[string]struct{}, len(names))
	for _, name := range names {
		known[name] = struct{}{}
	}

	fileSet, enginePackage, sourceFiles, info, err := loadTypeInfo(directory, modulePath)
	if err != nil {
		return keepAll(err)
	}

	result := Result{Live: make(map[string]string)}
	inspectUses(directory, fileSet, enginePackage, info, known, &result)
	inspectCalls(directory, fileSet, enginePackage, sourceFiles, info, known, &result)

	return result
}

func loadTypeInfo(directory, modulePath string) (*token.FileSet, *types.Package, []*ast.File, *types.Info, error) {
	fileSet := token.NewFileSet()
	enginePath := modulePath + "/.karty/engine"

	engineFiles, err := parsePackageFiles(fileSet, filepath.Join(directory, ".karty", "engine"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	enginePackage, err := (&types.Config{Importer: importer.Default()}).Check(enginePath, fileSet, engineFiles, nil)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	sourceFiles, err := parsePackageFiles(fileSet, filepath.Join(directory, "src"))
	if err != nil {
		return nil, nil, nil, nil, err
	}

	sourceFiles, err = appendUIFiles(fileSet, directory, modulePath, sourceFiles)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	resolver := packageImporter{
		enginePath: enginePath,
		engine:     enginePackage,
		fallback:   importer.Default(),
	}
	configuration := &types.Config{Importer: resolver}

	var (
		uiFiles   []*ast.File
		mainFiles []*ast.File
	)

	for _, file := range sourceFiles {
		if file.Name.Name == "ui" {
			uiFiles = append(uiFiles, file)
		} else {
			mainFiles = append(mainFiles, file)
		}
	}

	if len(uiFiles) > 0 {
		uiPath := modulePath + "/.karty/ui"

		uiPackage, err := configuration.Check(uiPath, fileSet, uiFiles, info)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		resolver.uiPath, resolver.ui = uiPath, uiPackage
		configuration.Importer = resolver
	}

	if _, err := configuration.Check(modulePath+"/src", fileSet, mainFiles, info); err != nil {
		return nil, nil, nil, nil, err
	}

	return fileSet, enginePackage, sourceFiles, info, nil
}

func inspectUses(
	directory string,
	fileSet *token.FileSet,
	enginePackage *types.Package,
	info *types.Info,
	known map[string]struct{},
	result *Result,
) {
	for identifier, object := range info.Uses {
		if object.Pkg() != enginePackage {
			continue
		}

		if typedConstant, ok := object.(*types.Const); ok && isTextureID(typedConstant.Type()) {
			name := constant.StringVal(typedConstant.Val())
			if _, exists := known[name]; exists {
				result.Live[name] = ReasonTypedReference
			}
		}

		switch object.Name() {
		case "DynamicTexture", "NewSprite2DName", "UpdateAssetName", "TextureID":
			result.KeepAll = true
			result.Diagnostic = fmt.Sprintf("%s at %s", ReasonDynamicLookup, sourcePosition(directory, fileSet.Position(identifier.Pos())))
		}
	}
}

func inspectCalls(
	directory string,
	fileSet *token.FileSet,
	enginePackage *types.Package,
	sourceFiles []*ast.File,
	info *types.Info,
	known map[string]struct{},
	result *Result,
) {
	for _, file := range sourceFiles {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			name, belongsToEngine := calledEngineFunction(call.Fun, info, enginePackage)
			if !belongsToEngine || (name != "NewSprite2D" && name != "UpdateAsset") || len(call.Args) == 0 {
				return true
			}

			value := info.Types[call.Args[0]].Value
			if value == nil || value.Kind() != constant.String {
				result.KeepAll = true
				result.Diagnostic = fmt.Sprintf(
					"%s at %s",
					ReasonDynamicLookup,
					sourcePosition(directory, fileSet.Position(call.Args[0].Pos())),
				)

				return true
			}

			logicalName := constant.StringVal(value)
			if _, exists := known[logicalName]; exists {
				result.Live[logicalName] = ReasonStaticName
			}

			return true
		})
	}
}

type packageImporter struct {
	uiPath     string
	ui         *types.Package
	enginePath string
	engine     *types.Package
	fallback   types.Importer
}

func (resolver packageImporter) Import(path string) (*types.Package, error) {
	if resolver.ui != nil && path == resolver.uiPath {
		return resolver.ui, nil
	}

	if path == resolver.enginePath {
		return resolver.engine, nil
	}

	return resolver.fallback.Import(path)
}

func parsePackageFiles(fileSet *token.FileSet, directory string) ([]*ast.File, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		matched, err := build.Default.MatchFile(directory, entry.Name())
		if err != nil {
			return nil, err
		}

		if !matched {
			continue
		}

		file, err := parser.ParseFile(fileSet, filepath.Join(directory, entry.Name()), nil, parser.AllErrors)
		if err != nil {
			return nil, err
		}

		files = append(files, file)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%s: %w", directory, errNoBuildableFiles)
	}

	return files, nil
}

func calledEngineFunction(expression ast.Expr, info *types.Info, engine *types.Package) (string, bool) {
	switch function := expression.(type) {
	case *ast.Ident:
		object := info.Uses[function]

		return objectName(object, engine)
	case *ast.SelectorExpr:
		if selection := info.Selections[function]; selection != nil {
			return objectName(selection.Obj(), engine)
		}

		return objectName(info.Uses[function.Sel], engine)
	default:
		return "", false
	}
}

func objectName(object types.Object, engine *types.Package) (string, bool) {
	if object == nil || object.Pkg() != engine {
		return "", false
	}

	return object.Name(), true
}

func isTextureID(assetType types.Type) bool {
	named, ok := assetType.(*types.Named)

	return ok && named.Obj().Name() == "TextureID"
}

func sourcePosition(directory string, position token.Position) string {
	relative, err := filepath.Rel(directory, position.Filename)
	if err == nil {
		position.Filename = filepath.ToSlash(relative)
	}

	return position.String()
}

func keepAll(err error) Result {
	return Result{
		Live:       make(map[string]string),
		KeepAll:    true,
		Diagnostic: fmt.Sprintf("%s: %v", ReasonAnalysisFailed, err),
	}
}

func appendUIFiles(fileSet *token.FileSet, directory, modulePath string, sourceFiles []*ast.File) ([]*ast.File, error) {
	if _, err := os.Stat(filepath.Join(directory, "karty.toml")); os.IsNotExist(err) {
		return sourceFiles, nil
	}

	config, err := project.Load(directory)
	if err != nil {
		return nil, err
	}

	views, err := project.CompileUI(
		directory, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source,
	)
	if err != nil {
		return nil, err
	}

	generated, err := codegen.UIClientFiles(views, modulePath)
	if config.SDK.Version == "0.0.1" {
		generated, err = codegen.UIPackageFiles(views, modulePath)
	}

	if err != nil {
		return nil, err
	}

	for _, view := range views {
		if !view.Local {
			continue
		}

		file, err := parser.ParseFile(fileSet, view.Source, generated[view.Source], parser.AllErrors)
		if err != nil {
			return nil, err
		}

		sourceFiles = append(sourceFiles, file)
	}

	return sourceFiles, nil
}

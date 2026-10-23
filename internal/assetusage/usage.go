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
	"github.com/karty-game/karty-ui/compiler"
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

const (
	errNoBuildableFiles    staticError = "package contains no buildable Go files"
	errQuotedGoFlags       staticError = "quoted GOFLAGS require conservative asset retention"
	errCompilerConstraints staticError = "compiler release or experiment build constraints require conservative asset retention"
)

// Result describes the conservative live texture set.
type Result struct {
	Live       map[string]string
	KeepAll    bool
	Diagnostic string
}

// Options selects the cartridge compiler whose source constraints are analyzed.
// An empty Compiler selects the standard Go WASI target.
type Options struct {
	Compiler string
}

// Analyze type-checks the generated engine package together with project source.
// An analysis uncertainty retains all assets instead of risking a false strip.
func Analyze(directory, modulePath string, names []string) Result {
	return AnalyzeWithUI(directory, modulePath, names, nil)
}

// AnalyzeWithUI uses the build's resolved components, including package exports.
// Nil components preserve standalone project discovery and conservative fallback.
func AnalyzeWithUI(directory, modulePath string, names []string, components []uicompiler.Component) Result {
	return AnalyzeWithOptions(directory, modulePath, names, components, Options{})
}

// AnalyzeWithOptions uses the same WASI source selection as the cartridge build.
func AnalyzeWithOptions(directory, modulePath string, names []string, components []uicompiler.Component, options Options) Result {
	known := make(map[string]struct{}, len(names))
	for _, name := range names {
		known[name] = struct{}{}
	}

	fileSet, enginePackage, assetsPackage, sourceFiles, info, err := loadTypeInfo(directory, modulePath, components, options)
	if err != nil {
		return keepAll(err)
	}

	result := Result{Live: make(map[string]string)}
	direct := inspectCalls(directory, fileSet, enginePackage, sourceFiles, info, known, &result)
	inspectUses(directory, fileSet, enginePackage, assetsPackage, info, known, direct, &result)

	return result
}

func loadTypeInfo(
	directory, modulePath string,
	components []uicompiler.Component,
	options Options,
) (*token.FileSet, *types.Package, *types.Package, []*ast.File, *types.Info, error) {
	fileSet := token.NewFileSet()
	enginePath := modulePath + "/.karty/engine"

	context, err := cartridgeBuildContext(options)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	engineFiles, err := parsePackageFiles(fileSet, filepath.Join(directory, ".karty", "engine"), context)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	fallback := importer.ForCompiler(fileSet, "gc", nil)

	enginePackage, err := (&types.Config{Importer: fallback}).Check(enginePath, fileSet, engineFiles, nil)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	resolver := &packageImporter{
		enginePath: enginePath, engine: enginePackage, fallback: fallback,
		directory: directory, generatedPrefix: modulePath + "/.karty/processed/code/",
		fileSet: fileSet, context: context, info: info,
		generated: make(map[string]*types.Package), loading: make(map[string]bool),
	}
	assetsPath := modulePath + "/.karty/assets"

	assetsFiles, err := parsePackageFiles(fileSet, filepath.Join(directory, ".karty", "assets"), context)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	assetsPackage, err := (&types.Config{Importer: resolver}).Check(assetsPath, fileSet, assetsFiles, nil)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	resolver.assetsPath, resolver.assets = assetsPath, assetsPackage

	sourceFiles, err := parsePackageFiles(fileSet, filepath.Join(directory, "src"), context)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	sourceFiles, err = appendUIFiles(fileSet, directory, modulePath, sourceFiles, components, uiAdapterVersion(enginePackage))
	if err != nil {
		return nil, nil, nil, nil, nil, err
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
			return nil, nil, nil, nil, nil, err
		}

		resolver.uiPath, resolver.ui = uiPath, uiPackage
		configuration.Importer = resolver
	}

	if _, err := configuration.Check(modulePath+"/src", fileSet, mainFiles, info); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return fileSet, enginePackage, assetsPackage, append(sourceFiles, resolver.generatedFiles...), info, nil
}

func inspectUses(
	directory string,
	fileSet *token.FileSet,
	enginePackage *types.Package,
	assetsPackage *types.Package,
	info *types.Info,
	known map[string]struct{},
	direct map[*ast.Ident]bool,
	result *Result,
) {
	for identifier, object := range info.Uses {
		if typedConstant, ok := object.(*types.Const); ok &&
			(object.Pkg() == assetsPackage || object.Pkg() == enginePackage) && isTextureID(typedConstant.Type()) {
			name := constant.StringVal(typedConstant.Val())
			if _, exists := known[name]; exists {
				result.Live[name] = ReasonTypedReference
			}
		}

		if object.Pkg() != enginePackage {
			continue
		}

		switch object.Name() {
		case "DynamicTexture", "TextureID":
			result.KeepAll = true
			result.Diagnostic = fmt.Sprintf("%s at %s", ReasonDynamicLookup, sourcePosition(directory, fileSet.Position(identifier.Pos())))
		}

		if textureFunction(object.Name()) && !direct[identifier] {
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
) map[*ast.Ident]bool {
	direct := make(map[*ast.Ident]bool)

	for _, file := range sourceFiles {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			name, belongsToEngine := calledEngineFunction(call.Fun, info, enginePackage)
			if !belongsToEngine || !textureFunction(name) {
				return true
			}

			function := unparen(call.Fun)
			argumentIndex := 0

			switch function := function.(type) {
			case *ast.Ident:
				direct[function] = true
			case *ast.SelectorExpr:
				direct[function.Sel] = true
				if selection := info.Selections[function]; selection != nil && selection.Kind() == types.MethodExpr {
					argumentIndex = 1
				}
			}

			if argumentIndex >= len(call.Args) {
				return true
			}

			argument := call.Args[argumentIndex]

			value := info.Types[argument].Value
			if value == nil || value.Kind() != constant.String {
				result.KeepAll = true
				result.Diagnostic = fmt.Sprintf(
					"%s at %s",
					ReasonDynamicLookup,
					sourcePosition(directory, fileSet.Position(argument.Pos())),
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

	return direct
}

func textureFunction(name string) bool {
	return name == "NewSprite2D" || name == "UpdateAsset" || name == "SetTexture"
}

func parsePackageFiles(fileSet *token.FileSet, directory string, context build.Context) ([]*ast.File, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(directory, entry.Name())

		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if err := checkCompilerConstraints(contents); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		matched, err := context.MatchFile(directory, entry.Name())
		if err != nil {
			return nil, err
		}

		if !matched {
			continue
		}

		file, err := parser.ParseFile(fileSet, path, contents, parser.AllErrors)
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
	switch function := unparen(expression).(type) {
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

func unparen(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}

		expression = parenthesized.X
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

func appendUIFiles(
	fileSet *token.FileSet,
	directory, modulePath string,
	sourceFiles []*ast.File,
	views []uicompiler.Component,
	version int,
) ([]*ast.File, error) {
	if views == nil {
		if _, err := os.Stat(filepath.Join(directory, "karty.toml")); os.IsNotExist(err) {
			return sourceFiles, nil
		}

		config, err := project.Load(directory)
		if err != nil {
			return nil, err
		}

		views, err = project.CompileUI(directory, config.Assets.UI, config.Assets.Layouts, config.Assets.Theme.Source)
		if err != nil {
			return nil, err
		}
	}

	clientFiles, err := codegen.UIClientFilesForVersion(views, modulePath, version)
	if err != nil {
		return nil, err
	}

	packageFiles, err := codegen.UIPackageFilesForVersion(views, modulePath, version)
	if err != nil {
		return nil, err
	}

	for _, view := range views {
		for _, generated := range [][]byte{clientFiles[view.Source], packageFiles[view.Source]} {
			file, err := parser.ParseFile(fileSet, view.Source, generated, parser.AllErrors)
			if err != nil {
				return nil, err
			}

			sourceFiles = append(sourceFiles, file)
		}
	}

	return sourceFiles, nil
}

const (
	legacyUIAdapterVersion   = 2
	reviewedUIAdapterVersion = 3
)

func uiAdapterVersion(engine *types.Package) int {
	object := engine.Scope().Lookup("UI")
	if object == nil {
		return legacyUIAdapterVersion
	}

	method, _, _ := types.LookupFieldOrMethod(types.NewPointer(object.Type()), true, engine, "SetText")
	if method != nil {
		return reviewedUIAdapterVersion
	}

	return legacyUIAdapterVersion
}

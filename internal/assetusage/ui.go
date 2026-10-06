package assetusage

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
)

// AnalyzeUI retains named UI references and conservatively retains the complete
// UI scope when a lookup is dynamic, a method escapes, or typing is incomplete.
func AnalyzeUI(directory, modulePath string, names []string) Result {
	fileSet, enginePackage, _, files, info, err := loadTypeInfo(directory, modulePath)
	if err != nil {
		return keepAll(err)
	}

	files, info = reachableUI(fileSet, files, info)

	known := make(map[string]bool, len(names))
	for _, name := range names {
		known[name] = true
	}

	result := Result{Live: make(map[string]string)}
	direct := make(map[*ast.Ident]bool)

	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			name, belongs := calledEngineFunction(call.Fun, info, enginePackage)
			if !belongs || name != "ShowUI" || len(call.Args) == 0 {
				return true
			}

			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				direct[selector.Sel] = true
			}

			value := info.Types[call.Args[0]].Value
			if value == nil || value.Kind() != constant.String {
				result.KeepAll = true

				return true
			}

			if name := constant.StringVal(value); known[name] {
				result.Live[name] = "static UI reference"
			}

			return true
		})
	}

	inspectUIUses(enginePackage, info, known, direct, &result)

	return result
}

// Generated functions for unused screens must not retain their own assets.
// Traverse references from authored Go into UI functions and between screens.
func reachableUI(fileSet *token.FileSet, files []*ast.File, info *types.Info) ([]*ast.File, *types.Info) {
	live := map[string]bool{}

	for _, file := range files {
		name := fileSet.Position(file.Pos()).Filename
		if !isUISource(name) {
			live[name] = true
		}
	}

	for changed := true; changed; {
		changed = false

		for identifier, object := range info.Uses {
			if !live[fileSet.Position(identifier.Pos()).Filename] {
				continue
			}

			target := fileSet.Position(object.Pos()).Filename
			if isUISource(target) && !live[target] {
				live[target] = true
				changed = true
			}
		}
	}

	result := make([]*ast.File, 0, len(files))
	for _, file := range files {
		if live[fileSet.Position(file.Pos()).Filename] {
			result = append(result, file)
		}
	}

	filtered := *info

	filtered.Uses = map[*ast.Ident]types.Object{}
	for identifier, object := range info.Uses {
		if live[fileSet.Position(identifier.Pos()).Filename] {
			filtered.Uses[identifier] = object
		}
	}

	return result, &filtered
}

func inspectUIUses(enginePackage *types.Package, info *types.Info, known map[string]bool, direct map[*ast.Ident]bool, result *Result) {
	for identifier, object := range info.Uses {
		if object.Pkg() != enginePackage {
			continue
		}

		if object.Name() == "ShowUI" && !direct[identifier] {
			result.KeepAll = true
		}

		if object.Name() == "UIAsset" {
			result.KeepAll = true
		}

		value, isConstant := object.(*types.Const)
		if !isConstant || value.Val().Kind() != constant.String {
			continue
		}

		typeName, ok := value.Type().(*types.Named)
		if ok && typeName.Obj().Name() == "UIAsset" && known[constant.StringVal(value.Val())] {
			result.Live[constant.StringVal(value.Val())] = "typed UI reference"
		}
	}
}

func isUISource(name string) bool {
	return strings.HasSuffix(name, ".kui")
}

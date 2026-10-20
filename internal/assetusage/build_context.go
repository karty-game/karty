package assetusage

import (
	"fmt"
	"go/build"
	"go/build/constraint"
	"os"
	"strings"
)

func cartridgeBuildContext(options Options) (build.Context, error) {
	context := build.Default
	context.GOOS, context.GOARCH = "wasip1", "wasm"
	context.Compiler = "gc"
	context.CgoEnabled = false
	context.ToolTags = nil

	for feature := range strings.SplitSeq(os.Getenv("GOWASM"), ",") {
		if feature != "" {
			context.ToolTags = append(context.ToolTags, "wasm."+feature)
		}
	}

	switch options.Compiler {
	case "", "go":
		flags := os.Getenv("GOFLAGS")
		if strings.ContainsAny(flags, "\"'") {
			return context, errQuotedGoFlags
		}

		for flag := range strings.FieldsSeq(flags) {
			if tags, ok := strings.CutPrefix(flag, "-tags="); ok {
				context.BuildTags = strings.Fields(strings.ReplaceAll(tags, ",", " "))
			}
		}
	case "tinygo":
		// tinygo list supplies these tags to go list, overriding GOFLAGS -tags.
		// The Go loader still supplies the gc compiler tag.
		context.BuildTags = []string{
			"tinygo.wasm", "tinygo", "purego", "osusergo", "math_big_pure_go",
			"gc.precise", "scheduler.none", "serial.none", "tinygo.unicore",
		}
	default:
		return context, fmt.Errorf("unknown cartridge compiler %q: %w", options.Compiler, os.ErrInvalid)
	}

	return context, nil
}

// The SDK's compiler can differ from the compiler used to build the CLI.
// Preserve every asset if source selection depends on its release or experiments.
func checkCompilerConstraints(source []byte) error {
	for line := range strings.SplitSeq(string(source), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			break
		}

		if !constraint.IsGoBuild(line) && !constraint.IsPlusBuild(line) {
			continue
		}

		expression, err := constraint.Parse(line)
		if err != nil {
			return err
		}

		if uncertainCompilerConstraint(expression) {
			return errCompilerConstraints
		}
	}

	return nil
}

func uncertainCompilerConstraint(expression constraint.Expr) bool {
	switch expression := expression.(type) {
	case *constraint.TagExpr:
		return strings.HasPrefix(expression.Tag, "go1.") || strings.HasPrefix(expression.Tag, "goexperiment.")
	case *constraint.NotExpr:
		return uncertainCompilerConstraint(expression.X)
	case *constraint.AndExpr:
		return uncertainCompilerConstraint(expression.X) || uncertainCompilerConstraint(expression.Y)
	case *constraint.OrExpr:
		return uncertainCompilerConstraint(expression.X) || uncertainCompilerConstraint(expression.Y)
	default:
		return true
	}
}

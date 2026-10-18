package website

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCLIDoesNotImportWebsite(t *testing.T) {
	t.Parallel()

	const websitePath = "github.com/karty-game/karty/website"

	for _, directory := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join("..", directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}

			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}

			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}

				if importPath == websitePath || strings.HasPrefix(importPath, websitePath+"/") {
					t.Errorf("CLI source %s imports website code %s", path, importPath)
				}
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

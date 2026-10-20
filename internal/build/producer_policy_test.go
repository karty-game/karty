package build

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestResourceProducersAndShippedTemplatesExcludeJSONAndReflection(t *testing.T) {
	t.Parallel()

	for _, directory := range []string{"../levelbuild", ".", "../assetpipeline"} {
		t.Run(directory, func(t *testing.T) {
			t.Parallel()
			checkProducerImports(t, os.DirFS(directory), directory == ".")
		})
	}

	t.Run("templates", func(t *testing.T) {
		t.Parallel()
		checkShippedTemplates(t, os.DirFS("templates"))
	})
}

func checkProducerImports(t *testing.T, source fs.FS, buildPackage bool) {
	t.Helper()

	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return fs.SkipDir
			}

			return nil
		}

		if strings.HasSuffix(path, "_test.go") || !strings.HasSuffix(path, ".go") {
			return nil
		}

		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}

			// Version 1 release discovery is HTTP metadata, separate from the
			// binary cartridge/KLD resource format. Its sole Go JSON producer
			// is specified in website/content/design/web-delivery-v1.md.
			jsonDiscovery := buildPackage && path == "web_release.go"
			if name == "reflect" || (!jsonDiscovery && (name == "encoding/json" || strings.HasPrefix(name, "encoding/json/"))) {
				t.Errorf("resource producer %s imports %s", path, name)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func checkShippedTemplates(t *testing.T, source fs.FS) {
	t.Helper()

	err := fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(source, path)
		if err != nil {
			return err
		}

		for _, call := range []string{"JSON.parse", "JSON.stringify"} {
			// Only the release worker reads/writes JSON discovery and cache
			// metadata; game resource templates keep their binary contract.
			// See website/content/design/web-delivery-v1.md.
			if path != "web-service-worker.js.tmpl" && strings.Contains(string(data), call) {
				t.Errorf("shipped template %s contains %s", path, call)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

package build

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-ui/compiler"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

func TestGeneratedClientsExcludeUnapprovedDecodersTransitively(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	for _, compiler := range []string{"go", "tinygo"} {
		t.Run(compiler, func(t *testing.T) {
			t.Parallel()

			const module = "example.com/client-policy"

			root := t.TempDir()
			writeClientPolicyFixture(t, root, "go.mod", "module "+module+"\ngo 1.27.0\n")
			writeClientPolicyFixture(
				t,
				root,
				"src/main.go",
				"package main\nimport engine \""+module+"/.karty/engine\"\nfunc main(){ _ = engine.Run }\n",
			)

			bindings, err := sdk.ClientFiles(manifest, compiler)
			if err != nil {
				t.Fatal(err)
			}

			if err := writeGeneratedClientAPI(t.Context(), root, bindings); err != nil {
				t.Fatal(err)
			}

			view, err := uicompiler.Compile("menu.kui", []byte(`<template><panel><label>{title}</label></panel></template>
<script setup lang="go">func setup(title string) {}</script>`))
			if err != nil {
				t.Fatal(err)
			}

			stage, cleanup, err := stageClient(t.Context(), root, module, []uicompiler.Component{view}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			checkProducerImports(t, os.DirFS(filepath.Join(root, ".karty")))

			if err := CheckClientDependencies(t.Context(), root, "go", goTool, stage); err != nil {
				t.Fatal(err)
			}

			// A wrapper imports JSON indirectly, so direct imports in generated
			// source alone cannot satisfy this guard.
			writeClientPolicyFixture(
				t,
				root,
				"src/main.go",
				"package main\nimport \""+module+"/internal/wrapper\"\nfunc main(){ _ = wrapper.Encode }\n",
			)

			for _, forbidden := range []struct{ dependency, function string }{
				{"encoding/json", "json.Marshal"},
				{"encoding/csv", "csv.NewReader"},
				{"encoding/xml", "xml.NewDecoder"},
				{"encoding/gob", "gob.NewDecoder"},
				{"log/slog", "slog.Info"},
			} {
				writeClientPolicyFixture(
					t,
					root,
					"internal/wrapper/wrapper.go",
					"package wrapper\nimport \""+forbidden.dependency+"\"\nvar Encode = "+forbidden.function+"\n",
				)

				if err := CheckClientDependencies(t.Context(), root, "go", goTool, "./src"); !errors.Is(err, errClientDecoderDependency) {
					t.Fatalf("indirect %s dependency was accepted: %v", forbidden.dependency, err)
				}
			}
		})
	}
}

func writeClientPolicyFixture(t *testing.T, root, name, source string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestClientDecoderPolicyNamesExactPackagesAndKnownCodecTrees(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"encoding/json", "encoding/json/v2", "encoding/csv", "encoding/xml", "encoding/gob", "log/slog", "go.yaml.in/yaml/v3", "gopkg.in/yaml.v3", "github.com/pelletier/go-toml/v2/unstable", "github.com/BurntSushi/toml", "github.com/goccy/go-json", "github.com/json-iterator/go"} {
		if !forbiddenClientDecoder(name) {
			t.Errorf("decoder allowed: %s", name)
		}
	}

	for _, name := range []string{"encoding/binary", "encoding/hex", "fmt", "github.com/karty-game/tiled", "example.com/json-model", "encoding/jsonish"} {
		if forbiddenClientDecoder(name) {
			t.Errorf("unrelated package forbidden: %s", name)
		}
	}
}

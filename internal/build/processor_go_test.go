package build

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessorGeneratedGoIsOwnedAndTransactional(t *testing.T) {
	t.Parallel()

	project, set := processorFixture(t)
	defer set.Close()

	run := func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
		return encodeProcessorFiles([]processorFile{
			{Path: "go/helper/helper.go", Data: []byte("package helper\nconst Answer = 42\n")},
			{Path: "notes.go", Data: []byte("resource bytes, not Go source")},
		})
	}
	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal(err)
	}

	generated := filepath.Join(project, ".karty/processed/code/yarn.compile/helper/helper.go")

	before, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(before, []byte("DO NOT EDIT.")) || !bytes.Contains(before, []byte("const Answer = 42")) {
		t.Fatal("processor did not publish generated source with its ownership header")
	}

	resources, err := os.ReadFile(filepath.Join(project, ".karty/processed/processed.go"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(resources), "go/helper/helper.go") || !strings.Contains(string(resources), "notes.go") {
		t.Fatal("generated source must be compiled while ordinary outputs remain resources")
	}

	for _, files := range [][]processorFile{
		{{Path: "go/helper/helper.go", Data: []byte("invalid Go")}},
		{{Path: "go/go.mod", Data: []byte("module replacement")}},
		{{Path: "go/helper/helper_test.go", Data: []byte("package helper")}},
		{{Path: "go/helper/a.go", Data: []byte("package first")}, {Path: "go/helper/b.go", Data: []byte("package second")}},
		{{Path: "go/../../escaped.go", Data: []byte("package escaped")}},
	} {
		bad := func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			return encodeProcessorFiles(files)
		}
		if err := runProcessorsWith(t.Context(), project, set, bad); err == nil {
			t.Fatal("accepted invalid generated sources")
		}

		after, err := os.ReadFile(generated)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("failed processor changed existing generated source: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(project, "karty.toml"), []byte("[project]\nname='test'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(generated); !os.IsNotExist(err) {
		t.Fatalf("removed processor left stale source: %v", err)
	}
}

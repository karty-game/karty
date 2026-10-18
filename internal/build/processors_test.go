package build

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/packages"
)

func processorFixture(t *testing.T) (string, *packages.Set) {
	t.Helper()

	project := t.TempDir()

	tool := t.TempDir()

	if err := os.MkdirAll(filepath.Join(project, "dialog"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(project, "dialog", "story.yarn"), []byte("title: Start\n---\nHello.\n===\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tool, "processor.txt"), []byte("authored tool"), 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(tool)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = root.Close() })

	set := &packages.Set{
		Items: map[string]*packages.Resolved{
			"yarn": {
				Root:      root,
				Directory: tool,
				Manifest:  packages.Manifest{Processors: []packages.Processor{{Name: "compile", Command: []string{"test-tool"}}}},
			},
		},
	}

	processorManifest(t, project, "dialog/*.yarn")

	return project, set
}

func processorManifest(t *testing.T, project, input string) {
	t.Helper()

	data := "[project]\nname='test'\n[sdk]\nversion='0.0.13'\n[[build.processors]]\nuse='yarn.compile'\ninputs=['" + input + "']\n"
	if err := os.WriteFile(filepath.Join(project, "karty.toml"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestProcessorInputsAndTransactionalPublication(t *testing.T) {
	t.Parallel()

	project, set := processorFixture(t)
	calls := 0
	run := func(_ context.Context, directory string, argv []string, input []byte) ([]byte, error) {
		calls++

		if directory == set.Items["yarn"].Directory {
			t.Fatal("ran in authored package")
		}

		data, err := os.ReadFile(filepath.Join(directory, "processor.txt"))
		if err != nil || string(data) != "authored tool" {
			t.Fatal("missing confined tool snapshot", err)
		}

		request, err := decodeProcessorResponse(input)
		if err != nil {
			t.Fatal(err)
		}

		if len(request) != 1 || request[0].Path != "dialog/story.yarn" || !strings.Contains(string(request[0].Data), "Hello.") {
			t.Fatal("lost authored input", request)
		}

		return encodeProcessorFiles([]processorFile{{Path: "story.yarnc", Data: []byte("compiled")}})
	}

	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal(err)
	}

	generated := filepath.Join(project, ".karty", "processed", "processed.go")

	before, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(before), `"yarn.compile/story.yarnc"`) {
		t.Fatal("resource not exposed")
	}

	failure := func(context.Context, string, []string, []byte) ([]byte, error) {
		return nil, fmt.Errorf("broken dialog: %w", os.ErrInvalid)
	}

	if err := runProcessorsWith(t.Context(), project, set, failure); !errors.Is(err, os.ErrInvalid) {
		t.Fatal("accepted failing processor")
	}

	after, err := os.ReadFile(generated)
	if err != nil || string(after) != string(before) {
		t.Fatal("failed build replaced last generated resources", err)
	}

	if calls != 1 {
		t.Fatal(calls)
	}

	if err := runProcessorsWith(t.Context(), project, set, run); err != nil {
		t.Fatal("owned resources did not replace", err)
	}
}

func TestProcessorRejectsInputEscapesAndInvalidResponses(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"../outside.yarn", "/outside.yarn", ".karty/generated.yarn", "dist/stale.yarn", "dialog/missing*.yarn"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			project, set := processorFixture(t)
			processorManifest(t, project, input)

			called := false

			err := runProcessorsWith(
				t.Context(),
				project,
				set,
				func(context.Context, string, []string, []byte) ([]byte, error) {
					called = true

					return nil, nil
				},
			)
			if !errors.Is(err, errProcessor) || called {
				t.Fatal("invalid inputs reached tool", err, called)
			}
		})
	}

	valid, err := encodeProcessorFiles([]processorFile{{Path: "a", Data: []byte("hello")}})
	if err != nil {
		t.Fatal(err)
	}

	duplicate := append(append([]byte(nil), valid...), valid[8:]...)
	binary.LittleEndian.PutUint32(duplicate[4:], 2)

	malformed := [][]byte{
		[]byte("KPR2\x00\x00\x00\x00"),
		valid[:7],
		valid[:len(valid)-1],
		append(append([]byte(nil), valid...), 0),
		duplicate,
	}

	for _, response := range malformed {
		if _, err := decodeProcessorResponse(response); !errors.Is(err, errProcessorResponse) {
			t.Fatal("accepted malformed response")
		}
	}

	for _, name := range []string{"../escape", "/absolute", "a:b", "a\\b", strings.Repeat("x", 257)} {
		if _, err := encodeProcessorFiles([]processorFile{{Path: name}}); err == nil {
			t.Fatal("accepted output path", name)
		}
	}
}

func TestProcessorPreservesUnownedOutput(t *testing.T) {
	t.Parallel()

	project, set := processorFixture(t)

	target := filepath.Join(project, ".karty", "processed")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}

	keep := filepath.Join(target, "keep")
	if err := os.WriteFile(keep, []byte("authored"), 0600); err != nil {
		t.Fatal(err)
	}

	err := runProcessorsWith(t.Context(), project, set, func(context.Context, string, []string, []byte) ([]byte, error) {
		t.Fatal("unowned output reached tool")

		return nil, nil
	})
	if !errors.Is(err, errProcessorOwnership) {
		t.Fatal("overwrote unowned output")
	}

	data, err := os.ReadFile(keep)
	if err != nil || string(data) != "authored" {
		t.Fatal("lost authored file", err)
	}
}

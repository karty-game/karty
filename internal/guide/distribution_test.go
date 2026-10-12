package guide

import (
	"strings"
	"testing"

	"github.com/karty-game/karty/internal/toolchain"
	"go.yaml.in/yaml/v3"
)

func TestGuideWorkflowBuildsAndUploadsMatchingDistributions(t *testing.T) {
	t.Parallel()

	chapter, err := source.ReadFile("chapters/09-share.md")
	if err != nil {
		t.Fatal(err)
	}

	examples := codeBlocks(string(chapter), "yaml")
	if len(examples) != 1 {
		t.Fatal("distribution guide must contain one workflow")
	}

	var workflow struct {
		Name        string            `yaml:"name"`
		On          []string          `yaml:"on"`
		Permissions map[string]string `yaml:"permissions"`
		Jobs        map[string]struct {
			RunsOn string            `yaml:"runs-on"` //nolint:tagliatelle // GitHub Actions uses this exact key.
			Env    map[string]string `yaml:"env"`
			Steps  []struct {
				Name string            `yaml:"name"`
				Uses string            `yaml:"uses"`
				Run  string            `yaml:"run"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}

	decoder := yaml.NewDecoder(strings.NewReader(examples[0]))
	decoder.KnownFields(true)

	if err := decoder.Decode(&workflow); err != nil {
		t.Fatalf("invalid workflow example: %v", err)
	}

	if workflow.Permissions["contents"] != "read" || len(workflow.On) != 2 {
		t.Fatal("workflow must use read-only checkout on push and pull requests")
	}

	job, exists := workflow.Jobs["build"]
	if !exists || job.RunsOn != "ubuntu-24.04" || job.Env["GOEXPERIMENT"] != "simd" || job.Env["GOWORK"] != "off" {
		t.Fatal("workflow does not prepare the public CLI environment")
	}

	var commands strings.Builder

	artifacts := map[string]string{}

	for _, step := range job.Steps {
		commands.WriteString(step.Run)
		commands.WriteByte('\n')

		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			artifacts[step.With["name"]] = step.With["path"]
			if step.With["if-no-files-found"] != "error" {
				t.Fatal("missing build output must fail artifact upload")
			}
		}
	}

	for _, required := range []string{
		"go install github.com/karty-game/karty/cmd/karty@", "karty sdk install 0.0.11",
		"karty build --target web", "karty build --target native --platform windows-arm64", "RUNTIME_LICENSE.md", "SDK_LICENSE.md",
	} {
		if !strings.Contains(commands.String(), required) {
			t.Fatalf("workflow omits %q", required)
		}
	}

	if artifacts["game-web"] != "dist/web/" || artifacts["game-windows-arm64"] != "dist/native/windows-arm64/" {
		t.Fatalf("artifact paths differ from the build targets: %+v", artifacts)
	}

	for _, platform := range []string{"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64"} {
		if _, err := toolchain.NativePlatform(platform); err != nil || !strings.Contains(string(chapter), "`"+platform+"`") {
			t.Fatalf("distribution guide includes an unsupported or missing platform %q: %v", platform, err)
		}
	}
}

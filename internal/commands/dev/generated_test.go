package dev

import (
	"regexp"
	"testing"
)

func TestGeneratedSourceExclusions(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(generatedSourcePattern)
	for _, path := range []string{"api_codegen.go", "src/api_codegen.go", `src\api_codegen.go`} {
		if !pattern.MatchString(path) {
			t.Errorf("generated source is watched: %s", path)
		}
	}

	for _, path := range []string{"src/main.go", "src/my_api_codegen.go", "src/karty_runtime.gen.go", "karty.toml"} {
		if pattern.MatchString(path) {
			t.Errorf("user source is excluded: %s", path)
		}
	}
}

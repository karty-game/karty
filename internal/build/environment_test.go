package build

import (
	"strings"
	"testing"
)

func TestCompilerEnvironmentDisablesVCSStamping(t *testing.T) {
	t.Setenv("GOFLAGS", "-trimpath")
	t.Setenv("GOWORK", "/unexpected/workspace")

	environment := compilerEnvironment("tinygo")

	if value := environmentValue(environment, "GOWORK"); value != "off" {
		t.Errorf("GOWORK = %q, want off", value)
	}

	goFlags := strings.Fields(environmentValue(environment, "GOFLAGS"))
	if len(goFlags) != 2 || goFlags[0] != "-trimpath" || goFlags[1] != "-buildvcs=false" {
		t.Errorf("GOFLAGS = %q, want existing flags followed by -buildvcs=false", goFlags)
	}
}

func TestGoCompilerEnvironmentSelectsWASI(t *testing.T) {
	t.Setenv("GOROOT", "/unexpected/outer-go")

	environment := compilerEnvironment("go")
	for _, entry := range environment {
		if strings.HasPrefix(entry, "GOROOT=") {
			t.Errorf("environment unexpectedly retains %q", entry)
		}
	}

	if value := environmentValue(environment, "GOOS"); value != "wasip1" {
		t.Errorf("GOOS = %q, want wasip1", value)
	}

	if value := environmentValue(environment, "GOARCH"); value != "wasm" {
		t.Errorf("GOARCH = %q, want wasm", value)
	}
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="

	for _, entry := range environment {
		if value, found := strings.CutPrefix(entry, prefix); found {
			return value
		}
	}

	return ""
}

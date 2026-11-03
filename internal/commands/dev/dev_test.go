package dev

import (
	"testing"
)

func TestManagedDevelopmentKeepsSDKHostAndOwnsBaking(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KARTY_HOST_WEB", "")

	host, err := devHost(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}

	options := developmentOptions(host, "go", "", "", "")
	if options.Host != "" || options.Target != "web" || options.Compiler != "go" || !options.Development || !options.DevelopmentBakes {
		t.Fatalf("unexpected development options: %+v", options)
	}
}

func TestDevelopmentPreservesToolAndHostOverrides(t *testing.T) {
	t.Parallel()

	path := developmentBundleFixture(t)

	host, err := devHost(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}

	options := developmentOptions(host, "tinygo", "go-custom", "tinygo-custom", "wasm-tools-custom")
	if options.Host != path || options.Go != "go-custom" || options.TinyGo != "tinygo-custom" || options.WasmTools != "wasm-tools-custom" ||
		options.Compiler != "tinygo" {
		t.Fatalf("lost overrides: %+v", options)
	}

	if _, err := devHost(t.Context(), path+".missing"); err == nil {
		t.Fatal("missing explicit host accepted")
	}
}

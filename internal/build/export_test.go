package build

import (
	"runtime"
	"strings"
	"testing"
)

// WithVerifiedHostFixture supplies an explicit local SDK host to public build
// tests while preserving their requested raw or web output selection.
func WithVerifiedHostFixture(t *testing.T, options Options) Options {
	t.Helper()

	directory, _ := webCodecBundleFixture(t)

	bundle, err := readHostBundle(t.Context(), directory, "web", "")
	if err != nil {
		t.Fatal(err)
	}

	options.hostBundle = bundle
	if options.Target == "web" {
		options.Host = directory
	}

	return options
}

// NativeHostBundleFixture supplies the complete native inventory expected by
// current SDKs. It tests staging and architecture admission, not execution.
func NativeHostBundleFixture(t *testing.T) string {
	t.Helper()

	return nativeBundleFixture(t, runtime.GOOS+"-"+runtime.GOARCH).Directory
}

func nativeBundleFixture(t *testing.T, platform string) *hostBundle {
	t.Helper()
	directory := t.TempDir()
	entrypoint := "karty-host-webgpu"

	var libraries []string

	switch strings.Split(platform, "-")[0] {
	case "linux":
		libraries = []string{"libkarty_webgpu.so", "libwgpu_native.so", "libSDL3.so.0"}
	case "darwin":
		libraries = []string{"libkarty_webgpu.dylib", "libwgpu_native.dylib", "libSDL3.dylib"}
	case "windows":
		libraries = []string{"karty_webgpu.dll", "wgpu_native.dll", "SDL3.dll"}
	default:
		t.Fatal("unsupported native fixture platform", platform)
	}

	if strings.HasPrefix(platform, "windows-") {
		entrypoint += ".exe"
	}

	bundle := &hostBundle{Schema: 1, Renderer: "webgpu", Target: platform, Entrypoint: entrypoint, Files: map[string]hostBundleFile{}}
	addBundleFixtureFile(t, directory, bundle, entrypoint, nativeHeader(t, platform))

	for _, name := range libraries {
		addBundleFixtureFile(t, directory, bundle, name, []byte("native fixture library"))
	}

	writeHostBundleFixture(t, directory, bundle)
	bundle.Directory = directory

	return bundle
}

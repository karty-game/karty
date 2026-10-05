package build_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
)

func TestRunBuildsSelfDescribingClient(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	createErr := scaffold.CreateGame(directory, "pong", manifest)
	if createErr != nil {
		t.Fatalf("CreateGame() error = %v", createErr)
	}

	addUnusedTexture(t, directory)

	if err := os.WriteFile(filepath.Join(directory, "assets", "textures", "legacy-ignored.jpg"), []byte("not a JPEG"), 0o600); err != nil {
		t.Fatal(err)
	}

	writeLegacyRootOutput(t, directory)

	tinyGo, wasmTools := buildTools(t)

	runErr := build.RunWithOptions(context.Background(), directory, build.Options{TinyGo: tinyGo, WasmTools: wasmTools})
	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}

	for _, path := range []string{".karty/engine/game.go", ".karty/engine/components.go", ".karty/assets/textures.go", "dist/raw/game.kart", "dist/raw/asset-report.json"} {
		if _, statErr := os.Stat(filepath.Join(directory, path)); statErr != nil {
			t.Errorf("generated %s: %v", path, statErr)
		}
	}

	assertAssetMetadata(t, filepath.Join(directory, "dist", "raw"))
	assertUnusedTextureStripped(t, filepath.Join(directory, "dist", "raw"), 2)
	assertDistributionRootContainsOnlyDirectories(t, filepath.Join(directory, "dist"))
	assertNoCatalog(t, filepath.Join(directory, "dist", "raw"))
}

func TestRunBuildsAndStagesLevelCartridges(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateGame(directory, "pong", manifest); err != nil {
		t.Fatal(err)
	}

	levelDirectory := filepath.Join(directory, "levels", "one")
	if err := os.MkdirAll(filepath.Join(levelDirectory, "data"), 0o750); err != nil {
		t.Fatal(err)
	}

	definition := "[level]\nname='levels.one'\nkind='demo'\n[[data]]\nname='world'\nsource='data/world.bin'\n"
	if err := os.WriteFile(filepath.Join(levelDirectory, "level.toml"), []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(levelDirectory, "data", "world.bin"), []byte("level one"), 0o600); err != nil {
		t.Fatal(err)
	}

	tinyGo, wasmTools := buildTools(t)

	host := nativeHostFixture(t)

	if err := build.RunWithOptions(t.Context(), directory, build.Options{
		TinyGo: tinyGo, WasmTools: wasmTools, Host: host, Target: "native",
	}); err != nil {
		t.Fatal(err)
	}

	game, err := os.ReadFile(filepath.Join(directory, "dist", "native", "game.kart"))
	if err != nil {
		t.Fatal(err)
	}

	manifestSection, err := cartridge.ExtractSection(game, cartridge.ManifestSectionName)
	if err != nil {
		t.Fatal(err)
	}

	gameManifest, err := cartridge.DecodeManifest(manifestSection)
	if err != nil || len(gameManifest.Levels) != 1 || gameManifest.Levels[0].Name != "levels.one" {
		t.Fatalf("game manifest = %+v, %v", gameManifest, err)
	}

	artifact := filepath.Join("content", gameManifest.Levels[0].ContentSHA256+".kld")

	for _, distribution := range []string{"dist/raw", "dist/native"} {
		contents, err := os.ReadFile(filepath.Join(directory, distribution, artifact))
		if err != nil {
			t.Fatal(err)
		}

		if len(contents) < 4 || string(contents[:4]) != "\x00asm" {
			t.Fatalf("%s level is not core Wasm", distribution)
		}
	}
}

func addUnusedTexture(t *testing.T, directory string) {
	t.Helper()

	player, err := os.ReadFile(filepath.Join(directory, "assets", "textures", "player.png"))
	if err != nil {
		t.Fatal(err)
	}

	player = append(player, 0)

	//nolint:gosec // directory is a test-owned temporary scaffold.
	if err := os.WriteFile(filepath.Join(directory, "assets", "textures", "unused.png"), player, 0o600); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(directory, "karty.toml")

	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	contents = append(contents, []byte(`
[[assets.texture]]
name = "sprites.unused"
source = "assets/textures/unused.png"
profile = "sprite"
`)...)
	//nolint:gosec // manifestPath is inside a test-owned temporary scaffold.
	if err := os.WriteFile(manifestPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertUnusedTextureStripped(t *testing.T, distribution string, count int) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(distribution, "asset-report.json"))
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		fmt.Sprintf(`"strippedTextureCount": %d`, count),
		`"name": "sprites.unused"`,
		`"status": "stripped"`,
	} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("asset report missing %q: %s", expected, contents)
		}
	}

	assertTextureFileCount(t, distribution, 1)
}

func assertTextureFileCount(t *testing.T, distribution string, expected int) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(distribution, "game.kart"))
	if err != nil {
		t.Fatal(err)
	}

	assets, err := cartridge.ExtractAssets(contents)
	if err != nil {
		t.Fatal(err)
	}

	if len(assets) != expected {
		t.Errorf("embedded texture count = %d, want %d", len(assets), expected)
	}

	if _, err := os.Stat(filepath.Join(distribution, "assets")); !os.IsNotExist(err) {
		t.Errorf("separately staged assets remain: %v", err)
	}
}

func TestRunStagesNativeTarget(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateGame(directory, "pong", manifest); err != nil {
		t.Fatal(err)
	}

	tinyGo, wasmTools := buildTools(t)

	host := nativeHostFixture(t)

	err = build.RunWithOptions(context.Background(), directory, build.Options{
		TinyGo:    tinyGo,
		WasmTools: wasmTools,
		Host:      host,
		Target:    "native",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"dist/native/karty-host", "dist/native/game.kart"} {
		if _, statErr := os.Stat(filepath.Join(directory, path)); statErr != nil {
			t.Errorf("staged %s: %v", path, statErr)
		}
	}

	assertTextureFileCount(t, filepath.Join(directory, "dist", "native"), 1)
	assertNoCatalog(t, filepath.Join(directory, "dist", "native"))
}

func TestRunStagesWebTarget(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateGame(directory, "pong", manifest); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(directory, "dist", "web"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "dist", "client.wasm"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "dist", "web", "client.wasm"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	tinyGo, wasmTools := buildTools(t)

	host := filepath.Join(t.TempDir(), "karty-host.wasm")
	if err := os.WriteFile(host, []byte("host"), 0o600); err != nil {
		t.Fatal(err)
	}

	options := build.Options{TinyGo: tinyGo, WasmTools: wasmTools, Host: host, Target: "web"}
	if err := build.RunWithOptions(
		context.Background(),
		directory,
		options,
	); err == nil ||
		!strings.Contains(err.Error(), "matching wasm_exec.js") {
		t.Fatalf("missing companion runtime was not rejected: %v", err)
	}

	webRuntime := []byte("// runtime shipped with this exact host\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(host), "wasm_exec.js"), webRuntime, 0o600); err != nil {
		t.Fatal(err)
	}

	err = build.RunWithOptions(context.Background(), directory, build.Options{
		TinyGo:    tinyGo,
		WasmTools: wasmTools,
		Host:      host,
		Target:    "web",
	})
	if err != nil {
		t.Fatal(err)
	}

	stagedRuntime, err := os.ReadFile(filepath.Join(directory, "dist/web/wasm_exec.js"))
	if err != nil || string(stagedRuntime) != string(webRuntime) {
		t.Fatalf("staged runtime differs from host companion: %q, %v", stagedRuntime, err)
	}

	for _, path := range []string{
		"dist/web/game.kart",
		"dist/web/index.html",
		"dist/web/karty.js",
	} {
		if _, statErr := os.Stat(filepath.Join(directory, path)); statErr != nil {
			t.Errorf("staged %s: %v", path, statErr)
		}
	}

	indexPath := filepath.Join(directory, "dist/web/index.html")

	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(index), "KARTY_AIR_BASE_PATH_PATCHED") ||
		strings.Contains(string(index), "@@AIR_PROXY_BOOTSTRAP@@") {
		t.Fatal("production web shell contains the Air proxy bootstrap or an unreplaced placeholder")
	}

	err = build.RunWithOptions(context.Background(), directory, build.Options{
		TinyGo: tinyGo, WasmTools: wasmTools, Host: host, Target: "web", AirProxy: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	index, err = os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(index), "KARTY_AIR_BASE_PATH_PATCHED") ||
		!strings.Contains(string(index), "Object.defineProperty(window, \"SharedWorker\"") ||
		!strings.Contains(string(index), "new Proxy(window.EventSource") ||
		!strings.Contains(string(index), "new URL(\".\" + value, window.location.href)") ||
		strings.Contains(string(index), "@@AIR_PROXY_BOOTSTRAP@@") {
		t.Fatal("development web shell does not preserve Air URLs under a path-based proxy")
	}

	for _, path := range []string{"dist/client.wasm", "dist/web/client.wasm"} {
		if _, statErr := os.Stat(filepath.Join(directory, path)); !os.IsNotExist(statErr) {
			t.Errorf("legacy artifact %s remains: %v", path, statErr)
		}
	}

	assertTextureFileCount(t, filepath.Join(directory, "dist", "web"), 1)
	assertNoCatalog(t, filepath.Join(directory, "dist", "web"))
}

func TestRunRemovesStaleStrippedTextureOutput(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateGame(directory, "pong", manifest); err != nil {
		t.Fatal(err)
	}

	addUnusedTexture(t, directory)

	dynamicSource := `package main
import "example.com/pong/.karty/engine"
var retainedTexture = engine.DynamicTexture("sprites.unused")
`
	dynamicPath := filepath.Join(directory, "src", "dynamic.go")

	if err := os.WriteFile(dynamicPath, []byte(dynamicSource), 0o600); err != nil {
		t.Fatal(err)
	}

	tinyGo, wasmTools := buildTools(t)

	options := build.Options{TinyGo: tinyGo, WasmTools: wasmTools}
	if err := build.RunWithOptions(context.Background(), directory, options); err != nil {
		t.Fatal(err)
	}

	assertTextureFileCount(t, filepath.Join(directory, "dist", "raw"), 2)

	if err := os.Remove(dynamicPath); err != nil {
		t.Fatal(err)
	}

	if err := build.RunWithOptions(context.Background(), directory, options); err != nil {
		t.Fatal(err)
	}

	assertUnusedTextureStripped(t, filepath.Join(directory, "dist", "raw"), 1)
}

func TestRunRetainsExplicitlyKeptTexture(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "pong")

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	if err := scaffold.CreateGame(directory, "pong", manifest); err != nil {
		t.Fatal(err)
	}

	addUnusedTexture(t, directory)

	manifestPath := filepath.Join(directory, "karty.toml")

	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	contents = append(contents, []byte("keep = true\n")...)
	//nolint:gosec // manifestPath is inside a test-owned temporary scaffold.
	if err := os.WriteFile(manifestPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	tinyGo, wasmTools := buildTools(t)
	if err := build.RunWithOptions(context.Background(), directory, build.Options{TinyGo: tinyGo, WasmTools: wasmTools}); err != nil {
		t.Fatal(err)
	}

	assertTextureFileCount(t, filepath.Join(directory, "dist", "raw"), 2)

	report, err := os.ReadFile(filepath.Join(directory, "dist", "raw", "asset-report.json"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(report), `"reason": "explicit keep declaration"`) {
		t.Errorf("asset report does not explain explicit keep: %s", report)
	}
}

func assertAssetMetadata(t *testing.T, distribution string) {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(distribution, "asset-report.json"))
	if err != nil {
		t.Fatal(err)
	}

	assertGeneratedJSONNotice(t, "asset report", contents)

	for _, expected := range []string{
		`"processor": "qoi@1"`,
		`"estimatedDecodedBytes"`,
	} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("asset report missing %q: %s", expected, contents)
		}
	}
}

func assertNoCatalog(t *testing.T, distribution string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(distribution, "catalog.json")); !os.IsNotExist(err) {
		t.Errorf("external catalog remains: %v", err)
	}
}

func assertGeneratedJSONNotice(t *testing.T, name string, contents []byte) {
	t.Helper()

	for _, expected := range []string{
		`"warning": "GENERATED FILE. DO NOT EDIT."`,
		`"doNotEdit": "MANUAL CHANGES WILL BE OVERWRITTEN."`,
		`"sourceOfTruth":`,
		`"regenerate": "karty build"`,
	} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("generated %s missing guard %q: %s", name, expected, contents)
		}
	}
}

func writeLegacyRootOutput(t *testing.T, directory string) {
	t.Helper()

	distribution := filepath.Join(directory, "dist")
	if err := os.MkdirAll(filepath.Join(distribution, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(distribution, "raw"), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"game.kart", "catalog.json", "asset-report.json", filepath.Join("assets", "stale.png")} {
		if err := os.WriteFile(filepath.Join(distribution, path), []byte("legacy"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(distribution, "raw", "catalog.json"), []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertDistributionRootContainsOnlyDirectories(t *testing.T, directory string) {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf("distribution root contains file %s", entry.Name())
		}
	}
}

func buildTools(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	extension := ""
	tinyGoScript := "#!/bin/sh\nif [ \"$1\" != \"build\" ]; then exit 2; fi\nshift\nwhile [ \"$#\" -gt 0 ]; do if [ \"$1\" = \"-o\" ]; then printf '\\000asm\\001\\000\\000\\000' > \"$2\"; exit 0; fi; shift; done\nexit 1\n"
	wasmToolsScript := "#!/bin/sh\nexit 0\n"

	if runtime.GOOS == "windows" {
		extension = ".bat"
		tinyGoScript = "@echo off\r\nif not \"%~1\"==\"build\" exit /b 2\r\nshift\r\n:next\r\nif \"%~1\"==\"-o\" (echo asm> %~2 & exit /b 0)\r\nshift\r\nif not \"%~1\"==\"\" goto next\r\nexit /b 1\r\n"
		wasmToolsScript = "@echo off\r\nexit /b 0\r\n"
	}

	tinyGo := filepath.Join(directory, "tinygo"+extension)
	wasmTools := filepath.Join(directory, "wasm-tools"+extension)

	if err := os.WriteFile(tinyGo, []byte(tinyGoScript), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(tinyGo, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(wasmTools, []byte(wasmToolsScript), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(wasmTools, 0o700); err != nil {
		t.Fatal(err)
	}

	return tinyGo, wasmTools
}

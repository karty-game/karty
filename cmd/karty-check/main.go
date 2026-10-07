// karty-check runs reproducible integration checks with isolated projects.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/karty-game/karty/internal/build"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/scaffold"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/toolchain"
)

var errCheckMode = errors.New("select exactly one of --web, --browser, --watcher, --allocations, --ui, --world-camera, or --client-hooks")
var errWatcherPlatform = errors.New("watcher integration check currently requires macOS or Linux")

func main() {
	web := flag.Bool("web", false, "execute staged browser launcher and actual TinyGo client under Node")
	browser := flag.Bool("browser", false, "execute the Go/Ebiten host and TinyGo client in Chromium")
	watcher := flag.Bool("watcher", false, "check managed Air rebuild and generated-file exclusion behavior")
	allocations := flag.Bool("allocations", false, "check actual TinyGo guest allocation counters")
	checkUI := flag.Bool("ui", false, "scaffold and execute the UI template in native WASM and Chromium")
	worldCamera := flag.Bool("world-camera", false, "build the packaged world sample twice and verify native/browser camera switching")
	clientHooks := flag.Bool("client-hooks", false, "execute a crafted SDK 0.0.9 hook/action fixture through native WASM and Chromium")
	nativeOnly := flag.Bool("native-only", false, "with --client-hooks, run deterministic native builds and the WASM lifecycle check")

	flag.Parse()

	if *nativeOnly && !*clientHooks {
		fmt.Fprintln(os.Stderr, "--native-only requires --client-hooks")
		os.Exit(1)
	}

	if *clientHooks {
		if *web || *browser || *watcher || *allocations || *checkUI || *worldCamera {
			fmt.Fprintln(os.Stderr, errCheckMode)
			os.Exit(1)
		}

		root, err := os.Getwd()
		if err == nil {
			err = runClientHooksCheck(context.Background(), root, *nativeOnly)
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, "client hooks check:", err)
			os.Exit(1)
		}

		return
	}

	if *checkUI {
		if *web || *browser || *watcher || *allocations || *worldCamera {
			fmt.Fprintln(os.Stderr, errCheckMode)
			os.Exit(1)
		}

		if err := runUI(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "karty UI check:", err)
			os.Exit(1)
		}

		return
	}

	if err := run(context.Background(), *web, *browser, *watcher, *allocations, *worldCamera); err != nil {
		fmt.Fprintln(os.Stderr, "karty-check:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, web, browser, watcher, allocations, worldCamera bool) error {
	selected := 0

	for _, enabled := range []bool{web, browser, watcher, allocations, worldCamera} {
		if enabled {
			selected++
		}
	}

	if selected != 1 {
		return errCheckMode
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	if worldCamera {
		return runWorldCamera(ctx, root)
	}

	manifest, err := sdk.Resolve(testSDKVersion())
	if err != nil {
		return err
	}

	if watcher {
		if runtime.GOOS == "windows" {
			return errWatcherPlatform
		}

		air, resolveErr := toolchain.Air(toolchain.AirOptions{Version: manifest.Tools.Air})
		if resolveErr != nil {
			return fmt.Errorf("resolve managed Air; run karty toolchain install: %w", resolveErr)
		}

		return command(ctx, root, []string{"KARTY_TEST_AIR=" + air}, "go", "test",
			"./internal/commands/dev", "-timeout=9s", "-run", "TestAirWatcherLifecycle", "-count=1")
	}

	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("node.js is required: %w", err)
	}

	temporary, err := os.MkdirTemp("", "karty-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)

	project := filepath.Join(temporary, "runtime-probe")
	if err := scaffold.CreateGame(project, "runtime-probe", manifest); err != nil {
		return err
	}

	fixtures := []struct {
		source, destination string
	}{
		{source: "cmd/karty-check/testdata/runtime-probe.go", destination: "main.go"},
		{source: "cmd/karty-check/testdata/alloccheck.go", destination: "alloccheck.go"},
	}

	for _, fixture := range fixtures {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixture.source)))
		if err != nil {
			return err
		}

		//nolint:gosec // Fixed destinations inside a newly created temporary fixture.
		if err := os.WriteFile(filepath.Join(project, "src", fixture.destination), contents, 0o600); err != nil {
			return err
		}
	}

	// The integration fixture uses an explicit texture alias and level set.
	configPath := filepath.Join(project, "karty.toml")

	config, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config = append(config, []byte("\n[[assets.texture]]\nname = \"sprites.player\"\nsource = \"assets/textures/player.png\"\n")...)
	//nolint:gosec // Fixed filename in a newly allocated private fixture directory.
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		return err
	}

	levels := os.DirFS(filepath.Join(root, "cmd/karty-check/testdata/runtime-levels"))
	if err := os.CopyFS(filepath.Join(project, "levels"), levels); err != nil {
		return err
	}

	options := build.Options{}
	if web || browser {
		options.Target = "web"
		options.Host = os.Getenv("KARTY_HOST_WEB")
	}

	if err := build.RunWithOptions(ctx, project, options); err != nil {
		return err
	}

	if web {
		return command(ctx, root, nil, "node", "internal/build/testdata/launcher.test.mjs", filepath.Join(project, "dist/web"))
	}

	if browser {
		return command(ctx, root, nil, "node", "internal/build/testdata/browser.test.mjs", filepath.Join(project, "dist/web"))
	}

	tinygo, err := toolchain.TinyGo(toolchain.TinyGoOptions{Version: manifest.Tools.TinyGo})
	if err != nil {
		return err
	}

	artifact := filepath.Join(temporary, "alloccheck.kart")
	if err := command(ctx, project, []string{"GOWORK=off", "GOFLAGS=-buildvcs=false"}, tinygo,
		"build", "-target=wasi", "-buildmode=c-shared", "-scheduler=none", "-tags=karty_alloccheck", "-o", artifact, "./src"); err != nil {
		return err
	}

	return command(ctx, root, nil, "node", "cmd/karty-check/testdata/check-allocations.mjs", artifact,
		filepath.Join(project, ".karty", "engine", "protocol.go"))
}

func command(ctx context.Context, directory string, environment []string, executable string, args ...string) error {
	//nolint:gosec // This contributor check intentionally executes an explicit tool or pinned resolved executable.
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = directory

	cmd.Env = append(os.Environ(), environment...)
	cmd.Stdout = os.Stdout

	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", executable, args, err)
	}

	return nil
}

func testSDKVersion() string {
	if version := os.Getenv("KARTY_TEST_SDK"); version != "" {
		return version
	}

	return release.SDKVersion()
}

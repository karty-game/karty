//go:build darwin || linux

package dev

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	airHelperEnvironment = "KARTY_AIR_WATCHER_HELPER"
	airCountEnvironment  = "KARTY_AIR_WATCHER_COUNT"
	watchTimeout         = 10 * time.Second
)

// TestAirWatcherLifecycle is opt-in because ordinary unit tests must not depend
// on an installed external executable. mise run check-dev supplies managed Air.
func TestAirWatcherLifecycle(t *testing.T) {
	t.Parallel()

	air := os.Getenv("KARTY_TEST_AIR")
	if air == "" {
		t.Skip("set KARTY_TEST_AIR; mise run check-dev supplies managed Air")
	}

	root := t.TempDir()

	sourceDirectory := filepath.Join(root, "src")
	if err := os.Mkdir(sourceDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	mainSource := filepath.Join(sourceDirectory, "main.go")

	assetDirectory := filepath.Join(root, "assets")
	if err := os.Mkdir(assetDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	uiSource := filepath.Join(assetDirectory, "menu.ui")
	if err := os.WriteFile(uiSource, []byte("initial UI"), 0o600); err != nil {
		t.Fatal(err)
	}

	projectUIDirectory := filepath.Join(root, "ui", "views")
	if err := os.MkdirAll(projectUIDirectory, 0o750); err != nil {
		t.Fatal(err)
	}

	projectUISource := filepath.Join(projectUIDirectory, "menu.ui")
	if err := os.WriteFile(projectUISource, []byte("initial project UI"), 0o600); err != nil {
		t.Fatal(err)
	}

	generatedSource := filepath.Join(sourceDirectory, "api_codegen.go")

	if err := os.WriteFile(mainSource, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	countPath := filepath.Join(root, "build-count")
	helper := shellQuote(os.Args[0])
	args := []string{
		"-root", root,
		"-build.cmd", helper + " -test.run=^TestAirBuildHelper$",
		"-build.full_bin", helper + " -test.run=^TestAirRunHelper$",
		"-build.include_dir", "src,assets,levels,ui",
		"-build.include_ext", "go,ui,json,toml,png",
		"-build.exclude_dir", "dist",
		"-build.exclude_regex", generatedSourcePattern,
		"-tmp_dir", airRuntimeDirectory,
		"-build.log", airBuildLog,
		"-build.delay", "100",
		"-build.stop_on_error", "true",
		"-log.silent", "true",
		"-misc.clean_on_exit", "true",
	}
	ctx, cancel := context.WithCancel(t.Context())
	//nolint:gosec // The executable is the explicitly resolved managed Air path in an opt-in integration test.
	process := exec.CommandContext(context.WithoutCancel(ctx), air, args...)

	process.Env = append(os.Environ(), airHelperEnvironment+"=1", airCountEnvironment+"="+countPath)
	prepareProcess(process)

	done := make(chan error, 1)
	go func() { done <- runProcess(ctx, process) }()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop Air: %v", err)
			}
		case <-time.After(watchTimeout):
			t.Error("Air did not stop")
		}
	})

	waitForBuildCount(t, countPath, 1)

	if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
		t.Fatalf("Air created a visible project tmp directory: %v", err)
	}

	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(airRuntimeDirectory))); err != nil || !info.IsDir() {
		t.Fatalf("Air runtime directory was not created under .karty: %v", err)
	}

	if err := os.WriteFile(generatedSource, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	time.Sleep(750 * time.Millisecond)

	if count := readBuildCount(t, countPath); count != 1 {
		t.Fatalf("generated output triggered a rebuild: got %d builds, want 1", count)
	}

	if err := os.WriteFile(mainSource, []byte("package main\n// user edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	waitForBuildCount(t, countPath, 2)
	time.Sleep(750 * time.Millisecond)

	if count := readBuildCount(t, countPath); count != 2 {
		t.Fatalf("one source edit produced extra rebuilds: got %d builds, want 2", count)
	}

	if err := os.WriteFile(uiSource, []byte("edited UI"), 0o600); err != nil {
		t.Fatal(err)
	}

	waitForBuildCount(t, countPath, 3)

	if err := os.WriteFile(projectUISource, []byte("edited project UI"), 0o600); err != nil {
		t.Fatal(err)
	}

	waitForBuildCount(t, countPath, 4)
}

func TestAirBuildHelper(t *testing.T) {
	t.Parallel()

	if os.Getenv(airHelperEnvironment) != "1" {
		t.Skip("Air subprocess helper")
	}

	path := os.Getenv(airCountEnvironment)

	count := 0
	//nolint:gosec // The parent test provides its own temporary counter path.
	if contents, err := os.ReadFile(path); err == nil {
		count, err = strconv.Atoi(strings.TrimSpace(string(contents)))
		if err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}

	//nolint:gosec // The parent test provides its own temporary counter path.
	if err := os.WriteFile(path, []byte(strconv.Itoa(count+1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAirRunHelper(t *testing.T) {
	t.Parallel()

	if os.Getenv(airHelperEnvironment) != "1" {
		t.Skip("Air subprocess helper")
	}

	select {}
}

func waitForBuildCount(t *testing.T, path string, expected int) {
	t.Helper()

	deadline := time.Now().Add(watchTimeout)
	for time.Now().Before(deadline) {
		if contents, err := os.ReadFile(path); err == nil {
			count, parseErr := strconv.Atoi(strings.TrimSpace(string(contents)))
			if parseErr != nil {
				t.Fatal(parseErr)
			}

			if count >= expected {
				return
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for build %d", expected)
}

func readBuildCount(t *testing.T, path string) int {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		t.Fatal(err)
	}

	return count
}

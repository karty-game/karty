package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWebGPUHostShellLaunchOrderAndVisibleErrors(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node unavailable; required by mise run check-web")
	}

	directory, _ := webCodecBundleFixture(t)

	game := filepath.Join(t.TempDir(), "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	distribution := t.TempDir()
	if err := stageRequestedTarget(t.Context(), preparedMaterialSDKFixture(), "A test game", distribution,
		t.TempDir(), game, nil, Options{Host: directory, Target: "web"}); err != nil {
		t.Fatal(err)
	}

	// #nosec G204 -- Run the maintained Node checker on its staged index beneath t.TempDir.
	command := exec.CommandContext(
		t.Context(),
		"node",
		"testdata/host-bundle-shell.test.mjs",
		filepath.Join(distribution, "web", "index.html"),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("WebGPU shell launch: %v\n%s", err, output)
	}
}

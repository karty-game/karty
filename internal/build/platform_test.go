package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStageForeignPlatformKeepsOtherDistributions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	host := filepath.Join(root, "download")

	game := filepath.Join(root, "game.kart")
	if err := os.WriteFile(game, []byte("\x00asm\x01\x00\x00\x00"), 0600); err != nil {
		t.Fatal(err)
	}

	dist := filepath.Join(root, "dist")

	for _, platform := range []string{"linux-arm64", "windows-arm64", "darwin-arm64"} {
		if err := os.WriteFile(host, nativeHeader(t, platform), 0600); err != nil {
			t.Fatal(err)
		}

		if err := stageTarget(dist, root, game, nil, host, "native", "", platform, false, "platform-test", false); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"linux-arm64/karty-host", "windows-arm64/karty-host.exe", "windows-arm64/game.kart", "linux-arm64/game.kart", "darwin-arm64/karty-host", "darwin-arm64/game.kart"} {
		if _, err := os.Stat(filepath.Join(dist, "native", name)); err != nil {
			t.Fatal(err)
		}
	}

	info, err := os.Stat(filepath.Join(dist, "native", "darwin-arm64", "karty-host"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("native host must be executable: %v, %v", info, err)
	}
}

func TestPlatformOptionsFailBeforeBuilding(t *testing.T) {
	t.Parallel()

	for _, options := range []Options{
		{Target: "web", Platform: "windows-arm64"},
		{Target: "native", Platform: "../../escape"},
	} {
		if err := RunWithOptions(context.Background(), t.TempDir(), options); err == nil {
			t.Fatal("invalid platform options accepted")
		}
	}
}

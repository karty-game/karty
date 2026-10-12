package levelbuild

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBakeSummaryCountsOnlyPublishedPair(t *testing.T) {
	t.Parallel()

	root := lightingLevelFixture(t, "version: 4\n"+lightingRoomYAML+directLightmapYAML)
	enableFixtureLightmap(t, root, "enabled = true\noffline = true\nlights = [\"red\"]\npage_size = 512\ndensity = 1\nshadow_size = 32\n")

	cache := filepath.Join(root, ".karty", "bakes", "lighting")
	if err := os.MkdirAll(cache, 0o750); err != nil {
		t.Fatal(err)
	}

	old := filepath.Join(cache, "old-lightmap.qoi")
	if err := os.WriteFile(old, make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}

	samples, bounces := 1, 0

	reports, err := BakeAll(t.Context(), root, BakeOptions{Samples: &samples, Bounces: &bounces, Workers: 1}, nil)
	if err != nil || len(reports) != 1 {
		t.Fatalf("bake reports = %v, %v", reports, err)
	}

	manifest, err := os.Stat(reports[0].Manifest)
	if err != nil {
		t.Fatal(err)
	}

	image, err := os.Stat(reports[0].Image)
	if err != nil {
		t.Fatal(err)
	}

	if reports[0].OutputBytes != manifest.Size()+image.Size() {
		t.Fatalf("bake bytes = %d, manifest and atlas = %d", reports[0].OutputBytes, manifest.Size()+image.Size())
	}

	if info, err := os.Stat(old); err != nil || info.Size() != 4096 {
		t.Fatalf("bake changed an older cache entry: %v, %v", info, err)
	}
}

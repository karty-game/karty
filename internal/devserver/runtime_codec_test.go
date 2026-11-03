package devserver

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/cartridge"
)

func TestWebGPUCodecSnapshotRequiresCompleteFamilyAndRestartsOnWorkerChange(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	fixtureBuild(t, directory, []byte("level"), []byte("unchanged-code"), nil)

	if err := os.Remove(filepath.Join(directory, "karty.js")); err != nil {
		t.Fatal(err)
	}

	for name, data := range map[string]string{
		"index.html":    "<html><head></head><body><script type=module src=bootstrap.mjs></script></body></html>",
		"bootstrap.mjs": "import './runtime.mjs';", "runtime.mjs": "export const runtime=1;",
		"client.mjs": "export const client=1;", "runtime.bin": "host",
		"texture-codec.mjs": "codec bridge", "texture-worker.mjs": "worker first", "transcoder.wasm": "codec wasm",
		"licenses/Basis-LICENSE.txt": "Basis notice fixture", "licenses/Zstd-LICENSE.txt": "Zstd notice fixture",
	} {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filename, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	first := captureFixture(t, directory, 1)
	if err := os.WriteFile(filepath.Join(directory, "texture-worker.mjs"), []byte("worker changed"), 0o600); err != nil {
		t.Fatal(err)
	}

	second := captureFixture(t, directory, 2)
	if first.Generation.RuntimeIdentity == second.Generation.RuntimeIdentity {
		t.Fatal("codec change reused previous development runtime identity")
	}

	if err := os.Remove(filepath.Join(directory, "licenses", "Zstd-LICENSE.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := Capture(t.Context(), directory, 3); !errors.Is(err, ErrSnapshot) {
		t.Fatal("unlicensed/incomplete codec runtime published", err)
	}
}

func TestWebGPUCodecSnapshotRejectsEntireMissingFamilyForPreparedMaterial(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	fixtureBuild(t, directory, []byte("level"), []byte("unchanged-code"), []string{
		cartridge.FeatureWorldMaterialAtlasV1, cartridge.FeatureWorldMaterialGPUV1, cartridge.FeatureWorldSectorsV1,
	})

	for name, data := range map[string]string{
		"bootstrap.mjs": "import './runtime.mjs';", "runtime.mjs": "export const runtime=1;",
		"client.mjs": "export const client=1;", "runtime.bin": "host",
		"texture-codec.mjs": "codec bridge", "texture-worker.mjs": "worker", "transcoder.wasm": "codec wasm",
		"licenses/Basis-LICENSE.txt": "Basis notice fixture", "licenses/Zstd-LICENSE.txt": "Zstd notice fixture",
	} {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filename, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	captureFixture(t, directory, 1)

	for _, name := range textureCodecRuntimeFiles()[:3] {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Capture(t.Context(), directory, 2); !errors.Is(err, ErrSnapshot) {
		t.Fatal("prepared-material cartridge accepted without any texture codec files", err)
	}
}

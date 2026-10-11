package levelbuild

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

const levelParticleYAML = `particles:
  version: 1
  presets:
    - name: sparkle
      capacity: 8
      emission: {rate: 4, duration_seconds: 1}
      lifetime_seconds: {min: 0.3, max: 0.5}
      spawn: {shape: point}
      velocity_min: {z: 0.2}
      velocity_max: {z: 0.4}
      acceleration: {z: -0.1}
      sprite: {frames: [wall], playback: cycle, facing: camera-facing, blend: additive}
      size_over_life: [{age: 0, value: 0.05}, {age: 1, value: 0}]
      color_over_life: [{age: 0, rgba: [0, 1, 0, 1]}, {age: 1, rgba: [0, 1, 0, 0]}]
  emitters:
    - {id: pool, preset: sparkle, position: {x: 1, y: 1, z: 1}, seed: 42, playback: {mode: cycle, period_seconds: 1}}
`

func TestParticleCapabilityAndPackagedWASM(t *testing.T) {
	t.Parallel()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	manifest = withoutRuntimeCapabilities(geometrySDK(manifest), asset.CapabilityWorldParticlesV1)
	root := lightingLevelFixture(t, "version: 10\n"+lightingRoomYAML+levelParticleYAML)

	if artifacts, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest); !errors.Is(err, ErrManifest) || artifacts != nil {
		t.Fatalf("SDK gate lost: %v", err)
	}

	manifest.Assets.Capabilities.Runtime = append(manifest.Assets.Capabilities.Runtime, asset.CapabilityWorldParticlesV1)

	first, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	second, err := BuildAllWithAssets(context.Background(), root, 4, "", manifest)
	if err != nil {
		t.Fatal(err)
	}

	if len(first) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) || !slices.Contains(first[0].Features, worldparticles.Feature) {
		t.Fatal("particle staging is not deterministic or loses feature")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "particles.kld")

	if err := os.WriteFile(path, first[0].Bytes, 0o600); err != nil {
		t.Fatal(err)
	}

	const script = `import fs from 'node:fs';const {instance}=await WebAssembly.instantiate(fs.readFileSync(process.argv[1]));const e=instance.exports;if(e.karty_level_abi_version()!==2)throw new Error('ABI');process.stdout.write(Buffer.from(e.memory.buffer,e.karty_level_data_pointer(),e.karty_level_data_length()));`

	encoded, err := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", script, path).Output()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := level.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	data, _, found := envelope.Read(worldparticles.EntryName, 0, worldparticles.MaxEncodedSize)

	if !found {
		t.Fatal("particle entry absent in executed WASM")
	}

	particles, err := worldparticles.Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	if particles.Emitters[0].Seed != 42 || particles.Presets[0].Acceleration.Z != -.1 {
		t.Fatalf("particle payload changed: %+v", particles)
	}
}

func TestParticleFrameGeometryAndOwnedEntry(t *testing.T) {
	t.Parallel()

	particles := worldparticles.Document{
		Presets: []worldparticles.Preset{{Name: "fire", Sprite: worldparticles.Sprite{Frames: []uint32{1, 2}}}},
	}
	good := []metadataTexture{{ID: 1, Width: 64, Height: 64}, {ID: 2, Width: 64, Height: 64}}

	if err := validateParticleFrames(&particles, good); err != nil {
		t.Fatal(err)
	}

	for _, bad := range [][]metadataTexture{{{ID: 1, Width: 32, Height: 64}, {ID: 2, Width: 32, Height: 64}}, {{ID: 1, Width: 64, Height: 64}, {ID: 2, Width: 32, Height: 32}}, {{ID: 1, Width: 64, Height: 64}}} {
		if err := validateParticleFrames(&particles, bad); err == nil {
			t.Fatal("invalid particle frames accepted")
		}
	}

	if _, err := loadData(t.TempDir(), []dataEntry{{Name: worldparticles.EntryName, Source: "manual.bin"}}); !errors.Is(err, ErrSource) {
		t.Fatalf("manual generated entry accepted: %v", err)
	}
}

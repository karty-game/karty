package build

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/codec/qoa"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/project"
)

func TestSoundAssetFileUsesNameSortedCatalogIDs(t *testing.T) {
	t.Parallel()

	generated, err := soundAssetFile([]project.Sound{{Name: "ui.open"}, {Name: "ball.hit"}}, "example.com/game/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(generated, []byte("SoundBallHit engine.SoundID = 1")) ||
		!bytes.Contains(generated, []byte("SoundUiOpen")) || !bytes.Contains(generated, []byte("engine.SoundID = 2")) {
		t.Fatalf("generated constants:\n%s", generated)
	}

	empty, err := soundAssetFile(nil, "example.com/game/.karty/engine")
	if err != nil || !bytes.Contains(empty, []byte("package assets")) {
		t.Fatalf("empty generated sound file = %q, error = %v", empty, err)
	}

	if bytes.Contains(empty, []byte("import engine")) {
		t.Fatalf("empty generated sound file has unused import: %q", empty)
	}
}

func TestEmbedProjectAssetsIncludesTypedSoundCatalog(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	encoded, metadata, err := qoa.Encode([]int16{100, -100, 200, -200}, 1, 48_000)
	if err != nil {
		t.Fatal(err)
	}

	soundPath := filepath.Join(directory, "payload")
	if err := os.WriteFile(soundPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}

	artifact := filepath.Join(directory, "game.kart")
	if err := os.WriteFile(artifact, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	sounds := []assetpipeline.Sound{{
		ID: 1, Name: "ball.hit", SourcePath: soundPath,
		OutputSHA256: fmt.Sprintf("%x", sha256.Sum256(encoded)), OutputBytes: int64(len(encoded)),
		Metadata: assetpipeline.AudioMetadata{Channels: metadata.Channels, SampleRate: metadata.SampleRate, Frames: metadata.Frames},
	}}
	if err := embedProjectAssets(directory, artifact, nil, sounds, nil, nil); err != nil {
		t.Fatal(err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := cartridge.ExtractSounds(wasm)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog) != 1 || catalog[0].Name != "ball.hit" || catalog[0].ID != 1 || !bytes.Equal(catalog[0].Bytes, encoded) {
		t.Fatalf("sound catalog = %+v", catalog)
	}
}

func TestEmbedProjectAssetsRejectsChangedCachedPayload(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	artifact := filepath.Join(directory, "game.kart")
	payload := filepath.Join(directory, "payload")

	if err := os.WriteFile(artifact, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(payload, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}

	texture := assetpipeline.Texture{
		Name: "player", SourcePath: payload, OutputBytes: 7,
		OutputSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("original"))),
	}
	if err := embedProjectAssets(directory, artifact, []assetpipeline.Texture{texture}, nil, nil, nil); err == nil {
		t.Fatal("embedProjectAssets accepted a cache payload changed after validation")
	}
}

func TestEmbedProjectManifestIncludesAssetFeatures(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	artifact := filepath.Join(directory, "game.kart")

	if err := os.WriteFile(artifact, []byte("\x00asm\x01\x00\x00\x00"), 0o600); err != nil {
		t.Fatal(err)
	}

	var config project.Config

	config.Project.Name = "features"
	config.Project.Resolution.Width = 320
	config.Project.Resolution.Height = 180

	features := []string{cartridge.FeatureSoundQOAv1, cartridge.FeatureTextureQOIv1}
	if err := embedProjectManifest(artifact, config, "tinygo", nil, features); err != nil {
		t.Fatal(err)
	}

	wasm, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}

	section, err := cartridge.ExtractSection(wasm, cartridge.ManifestSectionName)
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := cartridge.DecodeManifest(section)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal([]byte(manifest.Features[0]+"\x00"+manifest.Features[1]), []byte(features[0]+"\x00"+features[1])) {
		t.Fatalf("manifest features = %v", manifest.Features)
	}
}

func TestIncludeLevelAssetsUnionsFeaturesAndReportsProcessedTextures(t *testing.T) {
	t.Parallel()

	report := assetpipeline.Report{
		Features: []string{cartridge.FeatureSoundQOAv1},
		Summary: assetpipeline.Summary{
			EstimatedDecodedBytes: 4,
			AssetSourceBytes:      10,
			AssetOutputBytes:      8,
		},
	}

	levels := []levelbuild.Artifact{{
		Name: "levels.first", Features: []string{cartridge.FeatureTextureQOIv1},
		Textures: []assetpipeline.Texture{{
			Name: "background", SourceBytes: 20, OutputBytes: 12, EstimatedDecodedBytes: 16,
		}},
	}}
	if err := includeLevelAssets(&report, levels); err != nil {
		t.Fatal(err)
	}

	if len(report.Features) != 2 || report.Features[0] != cartridge.FeatureSoundQOAv1 ||
		report.Features[1] != cartridge.FeatureTextureQOIv1 {
		t.Fatalf("features = %v", report.Features)
	}

	if len(report.Levels) != 1 || report.Levels[0].Level != "levels.first" ||
		report.Summary.LevelTextureCount != 1 || report.Summary.LevelSourceBytes != 20 ||
		report.Summary.LevelOutputBytes != 12 || report.Summary.EstimatedDecodedLevelBytes != 16 ||
		report.Summary.AssetSourceBytes != 30 || report.Summary.AssetOutputBytes != 20 {
		t.Fatalf("level report = %+v, summary = %+v", report.Levels, report.Summary)
	}
}

func TestIncludeLevelAssetsRejectsCombinedDecodedBudget(t *testing.T) {
	t.Parallel()

	report := assetpipeline.Report{Summary: assetpipeline.Summary{EstimatedDecodedBytes: asset.MaxDecodedTextures}}

	levels := []levelbuild.Artifact{{
		Name:     "levels.too-large",
		Textures: []assetpipeline.Texture{{EstimatedDecodedBytes: 4}},
	}}
	if err := includeLevelAssets(&report, levels); err == nil {
		t.Fatal("includeLevelAssets accepted a level exceeding the combined texture budget")
	}
}

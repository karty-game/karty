package assetpipeline_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty/internal/assetpipeline"
	"github.com/karty-game/karty/internal/project"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
)

//nolint:gocyclo // One integration scenario intentionally checks cold, warm, and selectively invalidated builds.
func TestProcessGameAssetsCachesQOIAndQOAWithResolvedTransforms(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	writePNG(t, filepath.Join(directory, "first.png"), color.RGBA{R: 255, A: 255})
	writePNG(t, filepath.Join(directory, "second.png"), color.RGBA{G: 255, A: 255})

	wave := buildWAV(t, wavOptions{
		encoding: testWavePCM, bits: 16, channels: 2, sampleRate: 48_000,
		data: encodeIntegers(16, 100, -100, 200, -200, 300, -300, 400, -400),
	})
	for _, name := range []string{"open.wav", "hit.wav"} {
		if err := os.WriteFile(filepath.Join(directory, name), wave, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := processingManifest()
	textures := []project.Texture{
		{Name: "sprites.first", Source: "first.png", Profile: "sprite", Transform: project.TextureTransform{MaxWidth: 1}},
		{Name: "sprites.second", Source: "second.png", Profile: "sprite"},
	}
	sounds := []project.Sound{
		{Name: "ui.open", Source: "open.wav", Profile: "effect"},
		{Name: "ball.hit", Source: "hit.wav", Profile: "effect", Transform: project.SoundTransform{Channels: "mono"}},
	}

	first, err := assetpipeline.ProcessGameAssets(context.Background(), directory, manifest, textures, sounds)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(first.Features, []string{cartridge.FeatureSoundQOAv1, cartridge.FeatureTextureQOIv1}) {
		t.Fatalf("features = %v", first.Features)
	}

	if first.Textures[0].Width != 1 || first.Textures[0].Transform.MaxHeight != 2 || first.Textures[0].Encoding != "qoi" {
		t.Fatalf("resolved transformed texture = %+v", first.Textures[0])
	}

	if first.Sounds[0].Name != "ball.hit" || first.Sounds[0].ID != 1 || first.Sounds[0].Metadata.Channels != 1 ||
		first.Sounds[1].Name != "ui.open" || first.Sounds[1].ID != 2 {
		t.Fatalf("deterministic sound catalog = %+v", first.Sounds)
	}

	for _, texture := range first.Textures {
		if texture.CacheHit || texture.SourceSHA256 == "" || texture.OutputSHA256 == "" ||
			texture.SourcePath == filepath.Join(directory, texture.Source) {
			t.Fatalf("cold texture = %+v", texture)
		}
	}

	for _, sound := range first.Sounds {
		if sound.CacheHit || sound.SourceSHA256 == "" || sound.OutputSHA256 == "" {
			t.Fatalf("cold sound = %+v", sound)
		}
	}

	warm, err := assetpipeline.ProcessGameAssets(context.Background(), directory, manifest, textures, sounds)
	if err != nil {
		t.Fatal(err)
	}

	for _, texture := range warm.Textures {
		if !texture.CacheHit {
			t.Fatalf("warm texture miss = %+v", texture)
		}
	}

	for _, sound := range warm.Sounds {
		if !sound.CacheHit {
			t.Fatalf("warm sound miss = %+v", sound)
		}
	}

	writePNG(t, filepath.Join(directory, "second.png"), color.RGBA{B: 255, A: 255})

	selective, err := assetpipeline.ProcessGameAssets(context.Background(), directory, manifest, textures, sounds)
	if err != nil {
		t.Fatal(err)
	}

	if !selective.Textures[0].CacheHit || selective.Textures[1].CacheHit ||
		!selective.Sounds[0].CacheHit || !selective.Sounds[1].CacheHit {
		t.Fatalf("selective invalidation = textures %+v sounds %+v", selective.Textures, selective.Sounds)
	}
}

//nolint:wsl_v5 // One scenario verifies duration and both catalog namespaces together.
func TestProcessAudioStreamsUsesIndependentKindIDsAndLongDuration(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	// Thirty-one seconds proves the stream path does not inherit the one-shot limit.
	samples := make([]int64, 31*22_050)
	for index := range samples {
		samples[index] = int64((index % 200) - 100)
	}
	wave := buildWAV(t, wavOptions{encoding: testWavePCM, bits: 16, channels: 1, sampleRate: 22_050, data: encodeIntegers(16, samples...)})
	for _, name := range []string{"z.wav", "a.wav", "rain.wav"} {
		if err := os.WriteFile(filepath.Join(directory, name), wave, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := processingManifest()
	streams, err := assetpipeline.ProcessAudioStreams(t.Context(), directory, manifest,
		[]project.AudioStream{{Name: "z", Source: "z.wav", Profile: "effect"}, {Name: "a", Source: "a.wav", Profile: "effect"}},
		[]project.AudioStream{{Name: "rain", Source: "rain.wav", Profile: "effect"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 3 || streams[0].Name != "a" || streams[0].ID != 1 || streams[1].Name != "z" || streams[1].ID != 2 ||
		streams[2].Kind != "environment" ||
		streams[2].ID != 1 {
		t.Fatalf("streams = %+v", streams)
	}
	if streams[0].Metadata.Frames <= 30*streams[0].Metadata.SampleRate {
		t.Fatalf("duration was not preserved: %+v", streams[0].Metadata)
	}
}

func processingManifest() sdk.Manifest {
	return sdk.Manifest{Version: "test", Assets: struct {
		Capabilities    asset.Capabilities          `toml:"capabilities"`
		TextureProfiles map[string]sdk.AssetProfile `toml:"texture-profiles"`
		SoundProfiles   map[string]sdk.SoundProfile `toml:"sound-profiles"`
	}{
		Capabilities: asset.Capabilities{
			Processors: []asset.Processor{asset.ProcessorQOAv1, asset.ProcessorQOIv1},
			Runtime:    []asset.Capability{asset.CapabilitySoundQOAv1, asset.CapabilityTextureQOIv1},
		},
		TextureProfiles: map[string]sdk.AssetProfile{
			"sprite": {
				Processor: asset.ProcessorQOIv1,
				Transform: asset.ImageRecipe{
					MaxWidth: 2, MaxHeight: 2, Filter: asset.ImageFilterNearest, BitDepth: 8,
				},
			},
		},
		SoundProfiles: map[string]sdk.SoundProfile{
			"effect": {
				Processor: asset.ProcessorQOAv1,
				Transform: asset.AudioRecipe{SampleRate: 24_000, ChannelMode: asset.ChannelPreserve},
			},
		},
	}}
}

func TestProcessTexturesIsDeterministicAndProfileSensitive(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{R: 255, A: 255})

	declaration := project.Texture{Name: "sprites.player", Source: "player.png", Profile: "sprite"}
	manifest := sdkManifest(t)

	first, err := assetpipeline.ProcessGameAssets(t.Context(), directory, manifest, []project.Texture{declaration}, nil)
	if err != nil {
		t.Fatal(err)
	}

	second, err := assetpipeline.ProcessGameAssets(t.Context(), directory, manifest, []project.Texture{declaration}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if first.Textures[0].CacheKey != second.Textures[0].CacheKey || first.Textures[0].Output != second.Textures[0].Output {
		t.Fatalf("identical inputs produced different identities: %+v != %+v", first.Textures[0], second.Textures[0])
	}

	if first.Summary.EstimatedDecodedBytes != 16 || first.Textures[0].Width != 2 || first.Textures[0].Height != 2 {
		t.Fatalf("texture measurements = %+v", first)
	}

	if !strings.Contains(first.Textures[0].Output, first.Textures[0].OutputSHA256) {
		t.Fatalf("output %q is not content-addressed", first.Textures[0].Output)
	}

	declaration.Profile = "interface"

	profiled, err := assetpipeline.ProcessGameAssets(t.Context(), directory, manifest, []project.Texture{declaration}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if profiled.Textures[0].CacheKey == first.Textures[0].CacheKey {
		t.Fatal("texture profile did not affect the pipeline cache key")
	}

	if profiled.Textures[0].Output != first.Textures[0].Output {
		t.Fatal("unchanged output bytes should retain the same content-addressed path")
	}
}

func TestProcessTexturesRejectsInvalidPNG(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "player.png"), []byte("not png"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := assetpipeline.ProcessGameAssets(t.Context(), directory, sdkManifest(t), []project.Texture{{
		Name: "sprites.player", Source: "player.png", Profile: "sprite",
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), `texture "sprites.player"`) {
		t.Fatalf("ProcessGameAssets() error = %v", err)
	}
}

func TestProcessTexturesRejectsUnknownSDKProfile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writePNG(t, filepath.Join(directory, "player.png"), color.RGBA{A: 255})

	_, err := assetpipeline.ProcessGameAssets(t.Context(), directory, sdkManifest(t), []project.Texture{{
		Name: "sprites.player", Source: "player.png", Profile: "lossy",
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "not defined by the selected SDK") {
		t.Fatalf("ProcessGameAssets() profile error = %v", err)
	}
}

func sdkManifest(t *testing.T) sdk.Manifest {
	t.Helper()

	manifest, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	return manifest
}

func writePNG(t *testing.T, path string, fill color.Color) {
	t.Helper()

	var output bytes.Buffer

	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)

	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, output.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

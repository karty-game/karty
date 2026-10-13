package geometry_test

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldparticles"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/schema"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

const particleLabMaxLayers = 10

type particleLabTextures struct {
	IDs       map[string]uint32
	SpriteIDs map[uint32]bool
	Names     []string
}

type particleLabScene struct {
	Source    worldsource.Document
	World     sdkworld.Document
	Particles *worldparticles.Document
	Textures  particleLabTextures
}

type particleLabLayer struct {
	Preset  worldparticles.Preset
	Emitter worldparticles.Emitter
}

func TestParticleLabLibraryCompilesInsideEmptyRoom(t *testing.T) {
	t.Parallel()

	root := particleLabRoot()
	scene := readCompiledParticleLab(t, root, "lab")
	library := decodeParticleLabDocument(t, readParticleLabFile(t, filepath.Join(root, "effects", "library.yaml")))
	checkParticleLabSameSource(t, "presets", scene.Source.Particles, library.Particles)
	checkParticleLabSameSource(t, "prefabs", scene.Source.Prefabs, library.Prefabs)
	checkParticleLabStaticLibrary(t, scene.Source)

	if len(scene.Source.Rooms) != 1 || len(scene.Source.Instances) != 6 || len(scene.Source.Solids) != 0 ||
		len(scene.Source.Contents) != 0 || len(scene.World.Sectors) != 1 {
		t.Fatal("collection must contain six effects in one empty room")
	}

	capacity := checkParticleLabBoundsAndBudget(t, scene)
	encodedBytes := checkParticleLabRoundTrip(t, scene)
	t.Logf("%d presets, %d emitter layers, %d reserved slots, %d particle bytes",
		len(scene.Particles.Presets), len(scene.Particles.Emitters), capacity, encodedBytes)
}

func TestParticleLabReferenceFire(t *testing.T) {
	t.Parallel()

	root := particleLabRoot()
	scene := readCompiledParticleLab(t, root, "brackeys-fire")
	library := decodeParticleLabDocument(t, readParticleLabFile(t, filepath.Join(root, "effects", "library.yaml")))
	checkParticleLabStaticLibrary(t, scene.Source)

	if len(scene.Source.Rooms) != 1 || len(scene.Source.Instances) != 2 || len(scene.Particles.Emitters) != 6 {
		t.Fatal("reference fire needs two three-layer effects in the empty room")
	}

	for _, preset := range scene.Source.Particles.Presets {
		if !strings.HasPrefix(preset.Name, "brackeys-fire-combined-") && preset.Name != "lab-fire-shaped-tongues" {
			continue
		}

		found := false

		for _, expected := range library.Particles.Presets {
			if expected.Name == preset.Name {
				checkParticleLabSameSource(t, preset.Name, preset, expected)

				found = true

				break
			}
		}

		if !found {
			t.Fatalf("reviewed fire preset %s missing from reusable library", preset.Name)
		}
	}

	checkParticleLabBoundsAndBudget(t, scene)
	checkParticleLabRoundTrip(t, scene)
}

func TestParticleLabStandaloneSmoke(t *testing.T) {
	t.Parallel()

	root := particleLabRoot()
	scene := readCompiledParticleLab(t, root, "smoke")
	library := decodeParticleLabDocument(t, readParticleLabFile(t, filepath.Join(root, "effects", "library.yaml")))
	checkParticleLabEffectSource(t, scene.Source, library)
	checkParticleLabStaticLibrary(t, scene.Source)

	if len(scene.Source.Prefabs) != 2 || len(scene.Source.Instances) != 2 || len(scene.Source.Rooms) != 1 {
		t.Fatal("smoke review must contain the thin and rolling particle effects in one room")
	}

	checkParticleLabSmokeBackdrop(t, root, scene.Source.Rooms[0])

	for _, name := range []string{"industrial-smoke-thin", "industrial-smoke-rolls"} {
		instance := particleLabInstance(t, scene.Source, name)
		layers := particleLabLayers(t, scene, instance.ID)

		for _, layer := range layers {
			preset := layer.Preset
			if preset.Sprite.Blend != "alpha" || preset.VelocityMax.Z <= 0 {
				t.Fatalf("%s must use live, rising source-over particles", layer.Emitter.ID)
			}

			if preset.ColorOverLife[0].RGBA[3] != 0 || preset.ColorOverLife[len(preset.ColorOverLife)-1].RGBA[3] != 0 {
				t.Fatalf("%s smoke must fade in and out over particle lifetime", preset.Name)
			}
		}
	}

	capacity := checkParticleLabBoundsAndBudget(t, scene)
	encodedBytes := checkParticleLabRoundTrip(t, scene)
	t.Logf("smoke: %d live emitter layers, %d static textures, %d slots, %d particle bytes",
		len(scene.Particles.Emitters), len(scene.Textures.SpriteIDs), capacity, encodedBytes)
}

func particleLabRoot() string {
	return filepath.Join("..", "..", "..", "samples", "particle-lab")
}

func readCompiledParticleLab(t *testing.T, root, level string) particleLabScene {
	t.Helper()

	directory := filepath.Join(root, "levels", level)
	textures := readParticleLabTextures(t, root, directory)
	worldBytes := readParticleLabFile(t, filepath.Join(directory, "world.yaml"))

	value, err := schema.Parse(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := schema.Validate(value, schema.Level(value, textures.Names)); err != nil {
		t.Fatal(err)
	}

	document := decodeParticleLabDocument(t, worldBytes)

	expanded, err := source.Decode(worldBytes)
	if err != nil {
		t.Fatal(err)
	}

	world, err := geometry.Compile(expanded, textures.IDs)
	if err != nil {
		t.Fatal(err)
	}

	particles, err := geometry.CompileParticles(expanded, &world, textures.IDs)
	if err != nil {
		t.Fatal(err)
	}

	if particles == nil || particles.Version != 2 || document.Version != 11 {
		t.Fatal("live particle effects require source v11 and world/particles@2")
	}

	if world.Animations != nil {
		t.Fatal("particle lab must not package baked animation resources")
	}

	for _, content := range world.Contents {
		if content.Actor != nil {
			t.Fatalf("%s must not replace particle motion with an ordinary actor", content.ID)
		}
	}

	checkParticleLabTextureResources(t, particles, textures)

	return particleLabScene{Source: document, World: world, Particles: particles, Textures: textures}
}

func checkParticleLabStaticLibrary(t *testing.T, document worldsource.Document) {
	t.Helper()

	if document.Animations != nil || len(document.Particles.Emitters) != 0 {
		t.Fatal("recipes must own their emitter layers without baked animation resources or root emitters")
	}

	for _, preset := range document.Particles.Presets {
		if len(preset.Sprite.Frames) != 1 || preset.Sprite.FPS != 0 {
			t.Fatalf("%s needs exactly one static sprite per layer", preset.Name)
		}
	}

	all := document
	all.Instances = nil

	for _, prefab := range document.Prefabs {
		if len(prefab.Rooms) != 0 || len(prefab.Solids) != 0 || len(prefab.Contents) != 0 {
			t.Fatalf("%s must remain a particle-only effect", prefab.ID)
		}

		all.Instances = append(all.Instances, worldsource.Instance{
			ID: "check-" + prefab.ID, Prefab: prefab.ID, Transform: worldsource.Transform{Scale: 1},
		})
	}

	encoded, err := yaml.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}

	expanded, err := source.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	for _, instance := range all.Instances {
		count := 0

		for _, emitter := range expanded.ParticleEmitters {
			if strings.HasPrefix(emitter.ID, instance.ID+"/") {
				count++
			}
		}

		if count == 0 || count > particleLabMaxLayers {
			t.Fatalf("%s expands to %d layers; all particle roles must fit the ten-layer budget", instance.Prefab, count)
		}
	}
}

func checkParticleLabEffectSource(t *testing.T, document, library worldsource.Document) {
	t.Helper()

	presets := make(map[string]worldsource.ParticlePreset, len(library.Particles.Presets))
	for _, preset := range library.Particles.Presets {
		presets[preset.Name] = preset
	}

	for _, preset := range document.Particles.Presets {
		checkParticleLabSameSource(t, "preset "+preset.Name, preset, presets[preset.Name])
	}

	prefabs := make(map[string]worldsource.Prefab, len(library.Prefabs))
	for _, prefab := range library.Prefabs {
		prefabs[prefab.ID] = prefab
	}

	for _, prefab := range document.Prefabs {
		checkParticleLabSameSource(t, "prefab "+prefab.ID, prefab, prefabs[prefab.ID])
	}
}

func checkParticleLabSameSource(t *testing.T, label string, actual, expected any) {
	t.Helper()

	actualYAML, err := yaml.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}

	expectedYAML, err := yaml.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actualYAML, expectedYAML) {
		t.Fatalf("assembled scene changed library %s", label)
	}
}

func particleLabInstance(t *testing.T, document worldsource.Document, prefab string) worldsource.Instance {
	t.Helper()

	var matches []worldsource.Instance

	for _, instance := range document.Instances {
		if instance.Prefab == prefab {
			matches = append(matches, instance)
		}
	}

	if len(matches) != 1 {
		t.Fatalf("expected one instance of %s, got %d", prefab, len(matches))
	}

	return matches[0]
}

func particleLabLayers(t *testing.T, scene particleLabScene, instance string) []particleLabLayer {
	t.Helper()

	presets := make(map[string]worldparticles.Preset, len(scene.Particles.Presets))
	for _, preset := range scene.Particles.Presets {
		presets[preset.Name] = preset
	}

	var layers []particleLabLayer

	for _, emitter := range scene.Particles.Emitters {
		if strings.HasPrefix(emitter.ID, instance+"/") {
			layers = append(layers, particleLabLayer{Preset: presets[emitter.Preset], Emitter: emitter})
		}
	}

	if len(layers) == 0 || len(layers) > particleLabMaxLayers {
		t.Fatalf("%s has %d layers; require 1 to 10", instance, len(layers))
	}

	return layers
}

func readParticleLabFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func readParticleLabTextures(t *testing.T, root, directory string) particleLabTextures {
	t.Helper()

	var manifest struct {
		Textures []struct{ Name, Source, Profile string }
	}

	if err := toml.Unmarshal(readParticleLabFile(t, filepath.Join(directory, "level.toml")), &manifest); err != nil {
		t.Fatal(err)
	}

	textures := particleLabTextures{
		IDs: make(map[string]uint32, len(manifest.Textures)), SpriteIDs: make(map[uint32]bool),
		Names: make([]string, 0, len(manifest.Textures)),
	}
	for index, entry := range manifest.Textures {
		if textures.IDs[entry.Name] != 0 {
			t.Fatalf("duplicate texture %s", entry.Name)
		}

		textureID := uint32(index + 1)
		textures.IDs[entry.Name] = textureID
		textures.Names = append(textures.Names, entry.Name)

		encoded := readParticleLabFile(t, filepath.Join(directory, entry.Source))

		config, err := png.DecodeConfig(bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}

		if entry.Profile == "sprite" {
			textures.SpriteIDs[textureID] = true

			original := readParticleLabOriginalTexture(t, root, entry.Name)
			if !bytes.Equal(original, encoded) {
				t.Fatalf("copied static mask %s differs from its owning source", entry.Name)
			}

			if config.Width != config.Height || config.Width > 512 {
				t.Fatalf("%s must retain a square static mask no larger than the 512-pixel references", entry.Name)
			}
		}
	}

	return textures
}

func readParticleLabOriginalTexture(t *testing.T, root, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "effects", "textures", name+".png"))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func decodeParticleLabDocument(t *testing.T, data []byte) worldsource.Document {
	t.Helper()

	var document worldsource.Document
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}

	return document
}

func checkParticleLabTextureResources(t *testing.T, particles *worldparticles.Document, textures particleLabTextures) {
	t.Helper()

	used := map[uint32]bool{}

	for _, preset := range particles.Presets {
		if len(preset.Sprite.Frames) != 1 || preset.Sprite.FPS != 0 {
			t.Fatalf("%s packaged baked image animation", preset.Name)
		}

		used[preset.Sprite.Frames[0]] = true
	}

	if !reflect.DeepEqual(used, textures.SpriteIDs) {
		t.Fatal("scene must package exactly the static masks used by particle layers")
	}
}

func checkParticleLabBoundsAndBudget(t *testing.T, scene particleLabScene) uint32 {
	t.Helper()

	if len(scene.World.Sectors) != 1 || len(scene.Particles.Emitters) == 0 {
		t.Fatal("live effects must be mounted inside one convex test sector")
	}

	presets := make(map[string]worldparticles.Preset, len(scene.Particles.Presets))
	for _, preset := range scene.Particles.Presets {
		presets[preset.Name] = preset
	}

	var capacity uint32

	for _, emitter := range scene.Particles.Emitters {
		if emitter.Sector != 0 || !worldparticles.BoundsInSector(emitter.Bounds, &scene.World.Sectors[0]) {
			t.Fatalf("%s escapes the room with its complete motion envelope", emitter.ID)
		}

		preset, exists := presets[emitter.Preset]
		if !exists {
			t.Fatalf("%s references an absent preset", emitter.ID)
		}

		capacity += preset.Capacity
	}

	if capacity > 4096 {
		t.Fatalf("starter room reserves %d slots; keep the scene compact", capacity)
	}

	return capacity
}

func checkParticleLabRoundTrip(t *testing.T, scene particleLabScene) int {
	t.Helper()

	encoded, err := worldparticles.Encode(*scene.Particles)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := worldparticles.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if err := worldparticles.ValidateForWorld(&decoded, &scene.World); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != 2 || len(decoded.Emitters) != len(scene.Particles.Emitters) {
		t.Fatal("wire round-trip changed live particle resources")
	}

	return len(encoded)
}

func checkParticleLabSmokeBackdrop(t *testing.T, root string, room worldsource.Room) {
	t.Helper()

	if room.FloorMaterial != "smoke-room-floor" {
		t.Fatal("smoke-only room lost its neutral review floor")
	}

	for _, edge := range room.Boundary {
		if edge.Material != "smoke-room-wall" {
			t.Fatal("smoke-only room lost its neutral review backdrop")
		}
	}

	for _, name := range []string{"smoke-room-floor", "smoke-room-wall"} {
		original := readParticleLabFile(t, filepath.Join(root, "effects", "textures", name+".png"))
		copied := readParticleLabFile(t, filepath.Join(root, "levels", "smoke", "textures", name+".png"))

		if !bytes.Equal(original, copied) {
			t.Fatalf("smoke backdrop %s changed from its generator-owned source", name)
		}
	}
}

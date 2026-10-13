//go:build ignore

// Assemble the particle lab from its handwritten room and reusable effects.
// Run from the CLI repository root with mise run generate-particle-lab.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/sampleformat"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"go.yaml.in/yaml/v3"
)

const labRoot = "samples/particle-lab"

func main() {
	check := flag.Bool("check", false, "check assembled files without writing")
	flag.Parse()
	if err := assemble(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func assemble(check bool) error {
	room, err := readDocument("levels/lab/room.yaml")
	if err != nil {
		return err
	}
	library, err := readDocument("effects/library.yaml")
	if err != nil {
		return err
	}
	roomVersion, libraryVersion := field(room, "version"), field(library, "version")
	if roomVersion == nil || libraryVersion == nil {
		return fmt.Errorf("room and effect library require source versions")
	}
	roomNumber, err := strconv.Atoi(roomVersion.Value)
	if err != nil {
		return err
	}
	libraryNumber, err := strconv.Atoi(libraryVersion.Value)
	if err != nil {
		return err
	}
	if libraryNumber > roomNumber {
		*roomVersion = *libraryVersion
	}
	for _, key := range []string{"particles", "prefabs", "animations"} {
		value := field(library, key)
		if field(room, key) != nil {
			return fmt.Errorf("expected library %s and no room %s", key, key)
		}
		if value == nil && key == "animations" {
			continue
		}
		if value == nil {
			return fmt.Errorf("expected library %s", key)
		}
		room.Content = append(room.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	}
	if field(room, "instances") != nil {
		return fmt.Errorf("room must not contain gallery instances")
	}
	var placements yaml.Node
	err = placements.Encode([]worldsource.Instance{
		fireStation("fire", "brackeys-fire-combined", -6, -3),
		station("weld", "industrial-weld", 0, -3),
		station("acid", "industrial-acid", 6, -3),
		station("reactor", "industrial-reactor", -6, 4),
		station("discharge", "industrial-discharge", 0, 4),
		station("discharge-once", "industrial-discharge-once", 6, 4),
	})
	if err != nil {
		return err
	}
	room.Content = append(room.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "instances"}, &placements)
	var output bytes.Buffer
	output.WriteString("# Generated from room.yaml and ../../effects/library.yaml by generate-lab.go.\n")
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(room); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	var document worldsource.Document
	decoder := yaml.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := worldsource.Validate(&document); err != nil {
		return fmt.Errorf("assembled world: %w", err)
	}
	if err := validateStaticLayers(document); err != nil {
		return err
	}
	if err := publish("levels/lab/world.yaml", output.Bytes(), check); err != nil {
		return err
	}
	base, err := os.ReadFile(filepath.Join(labRoot, "levels/lab/level.base.toml"))
	if err != nil {
		return err
	}
	frames, err := effectTextureNames(document)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(frames))
	for name := range frames {
		names = append(names, name)
	}
	sort.Strings(names)
	manifest := bytes.NewBufferString("# Generated from level.base.toml and the effect frame references.\n")
	manifest.Write(bytes.TrimSpace(base))
	manifest.WriteByte('\n')
	for _, name := range names {
		fmt.Fprintf(
			manifest,
			"\n[[textures]]\nname = %s\nsource = %s\nprofile = \"sprite\"\n",
			strconv.Quote(name),
			strconv.Quote("textures/"+name+".png"),
		)
	}
	if err := publish("levels/lab/level.toml", manifest.Bytes(), check); err != nil {
		return err
	}
	names = append(names, "room-floor", "room-wall", "room-ceiling")
	for _, name := range names {
		asset, err := readEffectTexture(name)
		if err != nil {
			return err
		}
		if err := publish("levels/lab/textures/"+name+".png", asset, check); err != nil {
			return err
		}
	}
	if err := pruneTextures("lab", names, check); err != nil {
		return err
	}
	if err := assembleEffectRoom(document, base, "smoke", []worldsource.Instance{
		smokeStation("thin", "industrial-smoke-thin", -3),
		smokeStation("rolls", "industrial-smoke-rolls", 3),
	}, check); err != nil {
		return err
	}
	if err := assembleBrackeysFire(document, base, check); err != nil {
		return err
	}
	fmt.Printf(
		"Particle lab: %d presets, %d composite effects, %d effect textures (%s).\n",
		len(document.Particles.Presets),
		len(document.Prefabs),
		len(frames),
		map[bool]string{true: "checked", false: "assembled"}[check],
	)
	return nil
}

func assembleBrackeysFire(library worldsource.Document, base []byte, check bool) error {
	room, err := readDocument("levels/lab/room.yaml")
	if err != nil {
		return err
	}
	var reference worldsource.Document
	if err := room.Decode(&reference); err != nil {
		return err
	}
	reference.Version = 11
	reference.Particles = &worldsource.Particles{Version: 2}
	reference.Particles.Presets = append(reference.Particles.Presets, library.Particles.Presets...)
	reference.Prefabs = append(reference.Prefabs, library.Prefabs...)
	var placements []worldsource.Instance
	for _, recipe := range []struct {
		name string
		x    float64
	}{{"combined", -1.25}, {"separate", 1.25}} {
		instance := station(recipe.name, "brackeys-fire-"+recipe.name, recipe.x, 0)
		instance.Transform.Translation.Z = .35
		placements = append(placements, instance)
		if recipe.name == "combined" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(labRoot, "effects", "brackeys-fire-"+recipe.name+".yaml"))
		if err != nil {
			return err
		}
		var imported worldsource.Document
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&imported); err != nil {
			return err
		}
		// Portable templates contain effects only; the assembled room supplies
		// geometry for complete source and particle-envelope validation.
		if imported.Version != 11 || imported.Particles == nil || imported.Particles.Version != 2 {
			return fmt.Errorf("imported %s fire requires source 11 / particle 2", recipe.name)
		}
		refineSeparateSmoke(&imported)
		reference.Particles.Presets = append(reference.Particles.Presets, imported.Particles.Presets...)
		reference.Prefabs = append(reference.Prefabs, imported.Prefabs...)
	}
	return assembleEffectRoom(reference, base, "brackeys-fire", placements, check)
}

func refineSeparateSmoke(imported *worldsource.Document) {
	for index := range imported.Particles.Presets {
		preset := &imported.Particles.Presets[index]
		if !strings.Contains(preset.Name, "-smoke-") {
			continue
		}
		for key := range preset.ColorOverLife {
			preset.ColorOverLife[key].RGBA[3] *= .25
		}
	}
}

// Keep a separate mounted level for motion review, so rear stations cannot add
// flashes or colored filaments to the fire's silhouette or measured workload.
func assembleEffectRoom(document worldsource.Document, base []byte, levelName string, placements []worldsource.Instance, check bool) error {
	solo := document
	solo.Instances = placements
	solo.Prefabs = nil
	used := map[string]bool{}
	usedAnimations := map[string]bool{}
	selected := map[string]bool{}
	for _, placement := range placements {
		selected[placement.Prefab] = true
	}
	for _, prefab := range document.Prefabs {
		if selected[prefab.ID] {
			solo.Prefabs = append(solo.Prefabs, prefab)
			for _, emitter := range prefab.ParticleEmitters {
				used[emitter.Preset] = true
			}
			for _, content := range prefab.Contents {
				if content.Actor != nil && content.Actor.Animation != nil {
					usedAnimations[content.Actor.Animation.Preset] = true
				}
			}
		}
	}
	solo.Particles = nil
	if document.Particles != nil {
		particles := *document.Particles
		particles.Presets = nil
		particles.Emitters = nil
		for _, preset := range document.Particles.Presets {
			if used[preset.Name] {
				particles.Presets = append(particles.Presets, preset)
			}
		}
		if len(particles.Presets) != 0 {
			solo.Particles = &particles
		}
	}
	solo.Animations = nil
	if document.Animations != nil {
		animations := *document.Animations
		animations.Presets = nil
		animations.Materials = nil
		for _, preset := range document.Animations.Presets {
			if usedAnimations[preset.Name] {
				animations.Presets = append(animations.Presets, preset)
			}
		}
		if len(animations.Presets) != 0 {
			solo.Animations = &animations
		}
	}
	roomTextures := map[string]string{
		"room-floor": "room-floor", "room-wall": "room-wall", "room-ceiling": "room-ceiling",
	}
	if levelName == "smoke" {
		// Smoke is evaluated against a neutral mid-grey backdrop. Retain
		// approved fire-room materials and authored room geometry unchanged.
		solo.Rooms = append([]worldsource.Room(nil), document.Rooms...)
		for index := range solo.Rooms {
			room := &solo.Rooms[index]
			room.FloorMaterial = "smoke-room-floor"
			room.Boundary = append([]worldsource.Edge(nil), room.Boundary...)
			for edge := range room.Boundary {
				room.Boundary[edge].Material = "smoke-room-wall"
			}
		}
		base = bytes.ReplaceAll(base, []byte("room-wall"), []byte("smoke-room-wall"))
		base = bytes.ReplaceAll(base, []byte("room-floor"), []byte("smoke-room-floor"))
		roomTextures = map[string]string{
			"smoke-room-floor": "smoke-room-floor", "smoke-room-wall": "smoke-room-wall", "room-ceiling": "room-ceiling",
		}
	}
	frames, err := effectTextureNames(solo)
	if err != nil {
		return err
	}
	if err := worldsource.Validate(&solo); err != nil {
		return fmt.Errorf("%s room: %w", levelName, err)
	}
	if err := validateStaticLayers(solo); err != nil {
		return fmt.Errorf("%s room: %w", levelName, err)
	}
	var output bytes.Buffer
	fmt.Fprintf(&output, "# Generated %s scene from the empty room and reusable effects by generate-lab.go.\n", levelName)
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(solo); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	if err := publish("levels/"+levelName+"/world.yaml", output.Bytes(), check); err != nil {
		return err
	}
	manifest := bytes.NewBufferString("# Generated effect-only level; edit its owning room and effect sources.\n")
	manifest.Write(bytes.ReplaceAll(bytes.TrimSpace(base), []byte("levels.lab"), []byte("levels."+levelName)))
	manifest.WriteByte('\n')
	names := make([]string, 0, len(frames))
	for name := range frames {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(
			manifest,
			"\n[[textures]]\nname = %s\nsource = %s\nprofile = \"sprite\"\n",
			strconv.Quote(name),
			strconv.Quote("textures/"+name+".png"),
		)
	}
	if err := publish("levels/"+levelName+"/level.toml", manifest.Bytes(), check); err != nil {
		return err
	}
	for _, name := range names {
		asset, err := readEffectTexture(name)
		if err != nil {
			return err
		}
		if err := publish("levels/"+levelName+"/textures/"+name+".png", asset, check); err != nil {
			return err
		}
	}
	for destination, source := range roomTextures {
		asset, err := os.ReadFile(filepath.Join(labRoot, "effects/textures", source+".png"))
		if err != nil {
			return err
		}
		if err := publish("levels/"+levelName+"/textures/"+destination+".png", asset, check); err != nil {
			return err
		}
	}
	owned := append([]string(nil), names...)
	for name := range roomTextures {
		owned = append(owned, name)
	}
	return pruneTextures(levelName, owned, check)
}

// These texture directories are generated by this assembler. Keep them equal
// to their manifests when a recipe changes, rather than accumulating old masks.
func pruneTextures(level string, names []string, check bool) error {
	directory := filepath.Join(labRoot, "levels", level, "textures")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, name := range names {
		used[name+".png"] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" || used[entry.Name()] {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if check {
			return fmt.Errorf("unused generated texture %s; regenerate particle lab", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func readEffectTexture(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(labRoot, "effects/textures", name+".png"))
}

// Every particle layer references one static mask; motion belongs to particles.
func effectTextureNames(document worldsource.Document) (map[string]bool, error) {
	frames := map[string]bool{}
	add := func(name string) error {
		if name == "" || filepath.Base(name) != name {
			return fmt.Errorf("invalid library texture name %q", name)
		}
		frames[name] = true
		return nil
	}
	if document.Particles != nil {
		for _, preset := range document.Particles.Presets {
			for _, name := range preset.Sprite.Frames {
				if err := add(name); err != nil {
					return nil, err
				}
			}
		}
	}
	if document.Animations != nil {
		for _, preset := range document.Animations.Presets {
			for _, name := range preset.Frames {
				if err := add(name); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, prefab := range document.Prefabs {
		for _, content := range prefab.Contents {
			if content.Actor != nil && content.Actor.Sprite != nil {
				if err := add(content.Actor.Sprite.Texture); err != nil {
					return nil, err
				}
			}
		}
	}
	return frames, nil
}

func validateStaticLayers(document worldsource.Document) error {
	if document.Particles == nil || document.Particles.Version != 2 || document.Version < 11 {
		return fmt.Errorf("particle lab requires source v11 and world/particles@2")
	}
	if document.Animations != nil || len(document.Particles.Emitters) != 0 {
		return fmt.Errorf("particle effects must own live emitter layers without animation resources or root emitters")
	}
	for _, preset := range document.Particles.Presets {
		if len(preset.Sprite.Frames) != 1 || preset.Sprite.FPS != 0 {
			return fmt.Errorf("preset %s must use one static sprite without image animation", preset.Name)
		}
	}
	check := document
	check.Instances = nil
	for _, prefab := range document.Prefabs {
		if len(prefab.Rooms) != 0 || len(prefab.Solids) != 0 || len(prefab.Contents) != 0 {
			return fmt.Errorf("effect %s must contain only GPU particle layers", prefab.ID)
		}
		check.Instances = append(check.Instances, station("check-"+prefab.ID, prefab.ID, 0, 0))
	}
	encoded, err := yaml.Marshal(check)
	if err != nil {
		return err
	}
	expanded, err := source.Decode(encoded)
	if err != nil {
		return err
	}
	for _, instance := range check.Instances {
		layers := 0
		for _, emitter := range expanded.ParticleEmitters {
			if strings.HasPrefix(emitter.ID, instance.ID+"/") {
				layers++
			}
		}
		if layers == 0 || layers > 10 {
			return fmt.Errorf("effect %s expands to %d particle layers; require 1 to 10", instance.Prefab, layers)
		}
	}
	return nil
}

func station(id, prefab string, x, y float64) worldsource.Instance {
	return worldsource.Instance{
		ID:        id,
		Prefab:    prefab,
		Transform: worldsource.Transform{Translation: worldsource.Vec3{X: x, Y: y, Z: 1}, Scale: 1},
	}
}

func smokeStation(id, prefab string, x float64) worldsource.Instance {
	instance := station(id, prefab, x, 0)
	instance.Transform.Translation.Z = .9
	return instance
}

// Effect origins describe particle sources, with no baked-image center offset.
func fireStation(id, prefab string, x, y float64) worldsource.Instance {
	instance := station(id, prefab, x, y)
	instance.Transform.Translation.Z = .9
	return instance
}

func readDocument(relative string) (*yaml.Node, error) {
	data, err := os.ReadFile(filepath.Join(labRoot, relative))
	if err != nil {
		return nil, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must contain one mapping", relative)
	}
	return document.Content[0], nil
}

func field(mapping *yaml.Node, name string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == name {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func publish(relative string, data []byte, check bool) error {
	if strings.HasSuffix(relative, ".yaml") {
		formatted, err := sampleformat.YAML(context.Background(), data)
		if err != nil {
			return err
		}
		data = formatted
	}

	path := filepath.Join(labRoot, relative)
	if check {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("%s is stale; run mise run generate-particle-lab", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

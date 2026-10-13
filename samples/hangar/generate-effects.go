//go:build ignore

// Import selected Particle Lab recipes into narrow generated Hangar blocks.
// Geometry, lighting and handwritten records remain outside these blocks.
// Run from the CLI root with mise run generate-hangar-effects; -check is offline.
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
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

const (
	hangarRoot  = "samples/hangar/levels/hangar"
	libraryRoot = "samples/particle-lab/effects"
)

func main() {
	check := flag.Bool("check", false, "check imported blocks and confined textures without writing")
	flag.Parse()
	if err := generateEffects(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateEffects(check bool) error {
	var library worldsource.Document
	data, err := os.ReadFile(filepath.Join(libraryRoot, "library.yaml"))
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&library); err != nil {
		return err
	}
	// The reusable library is a fragment without rooms. Validate the complete
	// resolved Hangar, including all selected references and bounds, below.
	selected, frames, err := selectEffects(library, []string{"brackeys-fire-combined", "industrial-discharge", "industrial-acid"})
	if err != nil {
		return err
	}
	if err := spreadBarrelFire(&selected); err != nil {
		return err
	}
	selected.Instances = []worldsource.Instance{
		effectInstance("barrel-effects", "brackeys-fire-combined", 14.8, -1.55, 1.1, .75),
		effectInstance("reactor-discharge", "industrial-discharge", 6.3, 23.05, 3.7, .7),
	}
	// Cover the actual wet-solid polygons with wider, independently phased
	// patches. Full envelopes were checked against the convex room sectors,
	// liquid footprints and nearby trench ducts/columns; the narrow west-entry
	// pocket is omitted because it cannot fit a whole recipe at these scales.
	for _, patch := range []struct {
		id, prefab         string
		x, y, scale, delay float64
		seed               uint32
	}{
		{"acid-south-effects", "industrial-acid", 24.4, 17, 1.1, 0.000, 0},
		{"acid-south-middle-effects", "hangar-acid-south-middle", 29.2, 19.4, 0.8, 0.071, 137},
		{"acid-south-west-bank-effects", "hangar-acid-south-west-bank", 23, 18, 1.05, 0.142, 274},
		{"acid-south-east-effects", "hangar-acid-south-east", 35.6, 19, 1.1, 0.213, 411},
		{"acid-south-east-bank-effects", "hangar-acid-south-east-bank", 39, 17.6, 1.05, 0.284, 548},
		{"acid-south-east-mid-effects", "hangar-acid-south-east-mid", 33.8, 19.4, 0.8, 0.355, 685},
		{"acid-south-seam-effects", "hangar-acid-south-seam", 30.8, 18.8, 0.8, 0.426, 822},
		{"acid-west-bend-effects", "hangar-acid-west-bend", 26.4, 26, 1.1, 0.497, 959},
		{"acid-west-bend-east-effects", "hangar-acid-west-bend-east", 29.4, 27.4, 0.8, 0.568, 1096},
		{"acid-west-effects", "hangar-acid-west", 26.4, 30, 1.1, 0.639, 1233},
		{"acid-west-upper-east-effects", "hangar-acid-west-upper-east", 28.2, 29, 1.05, 0.710, 1370},
		{"acid-entry-effects", "hangar-acid-entry", 35, 22, 1.1, 0.781, 1507},
		{"acid-entry-west-effects", "hangar-acid-entry-west", 31, 23.2, 1.05, 0.852, 1644},
		{"acid-entry-east-effects", "hangar-acid-entry-east", 39, 23.2, 1.05, 0.923, 1781},
		{"acid-entry-south-effects", "hangar-acid-entry-south", 37.6, 20.8, 1.05, 0.994, 1918},
		{"acid-east-bend-effects", "hangar-acid-east-bend", 36, 26, 1.05, 1.065, 2055},
		{"acid-east-lower-west-effects", "hangar-acid-east-lower-west", 33.4, 24.8, 1.05, 1.136, 2192},
		{"acid-east-lower-bank-effects", "hangar-acid-east-lower-bank", 38, 25.2, 1.1, 1.207, 2329},
		{"acid-east-effects", "hangar-acid-east", 36.4, 30, 1.1, 1.278, 2466},
		{"acid-east-upper-west-effects", "hangar-acid-east-upper-west", 32.6, 31.4, 0.8, 1.349, 2603},
		{"acid-north-centre-effects", "hangar-acid-north-centre", 35, 33, 1.1, 1.420, 2740},
		{"acid-north-east-effects", "hangar-acid-north-east", 39, 33.2, 1.05, 1.491, 2877},
		{"acid-north-west-effects", "hangar-acid-north-west", 31, 33.2, 1.05, 1.562, 3014},
		{"acid-north-middle-effects", "hangar-acid-north-middle", 37, 33.2, 1.05, 1.633, 3151},
	} {
		selected.Instances = append(selected.Instances, effectInstance(patch.id, patch.prefab, patch.x, patch.y, -.93, patch.scale))
		if patch.prefab == "industrial-acid" {
			continue
		}
		for _, prefab := range library.Prefabs {
			if prefab.ID != "industrial-acid" {
				continue
			}
			if len(prefab.ParticleEmitters) != 5 || len(prefab.Instances) != 0 {
				return fmt.Errorf("acid patches require the five-layer reusable recipe")
			}
			prefab.ID = patch.prefab
			prefab.ParticleEmitters = append([]worldsource.ParticleEmitter(nil), prefab.ParticleEmitters...)
			for index := range prefab.ParticleEmitters {
				prefab.ParticleEmitters[index].Playback.DelaySeconds += patch.delay
				prefab.ParticleEmitters[index].Seed += patch.seed
			}
			selected.Prefabs = append(selected.Prefabs, prefab)
		}
	}
	world, err := os.ReadFile(filepath.Join(hangarRoot, "world.yaml"))
	if err != nil {
		return err
	}
	// Alpha particles require source 11; preserve the rest of the handwritten
	// world rather than serializing unrelated geometry and lighting.
	world = bytes.Replace(world, []byte("version: 10\n"), []byte("version: 11\n"), 1)
	world, err = replaceBlock(
		world,
		"particles",
		marshalYAML(map[string]any{"particles": selected.Particles}),
		"particles:\n",
		"animations:\n",
	)
	if err != nil {
		return err
	}
	world, err = replaceBlock(world, "prefabs", indent(marshalYAML(selected.Prefabs), 2), "prefabs:\n", "")
	if err != nil {
		return err
	}
	var animationBlock []byte
	if selected.Animations != nil && len(selected.Animations.Presets) > 0 {
		animationBlock = indent(marshalYAML(selected.Animations.Presets), 4)
	}
	world, err = replaceBlock(world, "animations", animationBlock, "  materials:\n", "")
	if err != nil {
		return err
	}
	world, err = replaceBlock(world, "instances", indent(marshalYAML(selected.Instances), 2), "instances:\n", "")
	if err != nil {
		return err
	}
	manifest, err := os.ReadFile(filepath.Join(hangarRoot, "level.toml"))
	if err != nil {
		return err
	}
	// Remove only the obsolete original particle texture declarations. The
	// opaque coals texture and its live emission-linked lamp stay authored.
	for _, name := range []string{"acid-sparkle", "barrel-flame-1", "barrel-flame-2", "barrel-flame-3", "barrel-ember"} {
		manifest = removeTexture(manifest, name)
	}
	manifest = bytes.ReplaceAll(
		manifest,
		[]byte(
			"# Original procedural white masks; GPU color curves provide their final hues.\n# Regenerate with samples/hangar/generate-particle-assets.go from the CLI root.\n",
		),
		nil,
	)
	var declarations bytes.Buffer
	for _, name := range frames {
		fmt.Fprintf(
			&declarations,
			"\n[[textures]]\nname = %s\nsource = %s\nprofile = \"sprite\"\n",
			strconv.Quote(name),
			strconv.Quote("textures/effects/"+name+".png"),
		)
	}
	manifest, err = replaceBlock(manifest, "textures", declarations.Bytes(), "", "")
	if err != nil {
		return err
	}
	if err := validateScene(world, manifest); err != nil {
		return err
	}
	if err := publishEffectFile("world.yaml", world, check); err != nil {
		return err
	}
	if err := publishEffectFile("level.toml", manifest, check); err != nil {
		return err
	}
	for _, name := range frames {
		if filepath.Base(name) != name || name == "" {
			return fmt.Errorf("unconfined recipe texture %q", name)
		}
		data, err := readEffectTexture(name)
		if err != nil {
			return err
		}
		if err := publishEffectFile("textures/effects/"+name+".png", data, check); err != nil {
			return err
		}
	}
	if err := pruneEffectTextures(frames, check); err != nil {
		return err
	}
	fmt.Printf(
		"Hangar effects: %d presets, %d reused/phase prefabs, %d instances, %d confined frame textures (%s).\n",
		len(selected.Particles.Presets),
		len(selected.Prefabs),
		len(selected.Instances),
		len(frames),
		map[bool]string{true: "checked", false: "assembled"}[check],
	)
	return nil
}

// A barrel burns across its coal bed. Keep the shared reference recipe intact;
// adapt only the imported copy's horizontal spawn and tongue size. Its original
// vertical lifetime/velocity envelope still ends below the 2.60-metre ceiling.
func spreadBarrelFire(selected *worldsource.Document) error {
	adapted := map[string]string{}
	for index := range selected.Particles.Presets {
		preset := &selected.Particles.Presets[index]
		switch preset.Name {
		case "brackeys-fire-combined-flamesmoke-01":
			adapted[preset.Name] = "hangar-barrel-flamesmoke"
			preset.Name = adapted[preset.Name]
			preset.Spawn = worldsource.ParticleSpawn{Shape: "box", HalfExtent: worldsource.Vec3{X: .24, Y: .18}}
		case "lab-fire-shaped-tongues":
			adapted[preset.Name] = "hangar-barrel-tongues"
			preset.Name = adapted[preset.Name]
			preset.Spawn.HalfExtent.X = .24
			preset.Spawn.HalfExtent.Y = .18
			for key := range preset.SizeOverLife {
				preset.SizeOverLife[key].Value *= 1.8
			}
		}
	}
	if len(adapted) != 2 {
		return fmt.Errorf("barrel adaptation requires the shared flame/smoke and tongue layers")
	}
	for index := range selected.Prefabs {
		prefab := &selected.Prefabs[index]
		if prefab.ID != "brackeys-fire-combined" {
			continue
		}
		for emitter := range prefab.ParticleEmitters {
			if name := adapted[prefab.ParticleEmitters[emitter].Preset]; name != "" {
				prefab.ParticleEmitters[emitter].Preset = name
			}
		}
	}
	return nil
}

func pruneEffectTextures(frames []string, check bool) error {
	directory := filepath.Join(hangarRoot, "textures/effects")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, name := range frames {
		used[name+".png"] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" || used[entry.Name()] {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if check {
			return fmt.Errorf("unused generated effect texture %s; regenerate Hangar effects", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

func readEffectTexture(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(libraryRoot, "textures", name+".png"))
}

func selectEffects(library worldsource.Document, names []string) (worldsource.Document, []string, error) {
	if library.Particles == nil {
		return worldsource.Document{}, nil, fmt.Errorf("Particle Lab library must declare particles")
	}
	prefabs := map[string]worldsource.Prefab{}
	for _, prefab := range library.Prefabs {
		prefabs[prefab.ID] = prefab
	}
	selectedPrefabs, particleNames, animationNames, textureNames := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if selectedPrefabs[name] {
			return nil
		}
		prefab, ok := prefabs[name]
		if !ok {
			return fmt.Errorf("missing library prefab %q", name)
		}
		if len(prefab.Rooms) > 0 || len(prefab.Solids) > 0 {
			return fmt.Errorf("effect %q unexpectedly imports geometry", name)
		}
		selectedPrefabs[name] = true
		for _, child := range prefab.Instances {
			if err := visit(child.Prefab); err != nil {
				return err
			}
		}
		for _, emitter := range prefab.ParticleEmitters {
			particleNames[emitter.Preset] = true
		}
		for _, content := range prefab.Contents {
			if content.Actor == nil {
				continue
			}
			if content.Actor.Sprite != nil {
				textureNames[content.Actor.Sprite.Texture] = true
			}
			if content.Actor.Animation != nil {
				animationNames[content.Actor.Animation.Preset] = true
			}
		}
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return worldsource.Document{}, nil, err
		}
	}
	result := worldsource.Document{Version: library.Version, Particles: &worldsource.Particles{Version: library.Particles.Version}}
	if library.Animations != nil {
		result.Animations = &worldsource.Animations{Version: library.Animations.Version}
	}
	for _, prefab := range library.Prefabs {
		if selectedPrefabs[prefab.ID] {
			result.Prefabs = append(result.Prefabs, prefab)
		}
	}
	for _, preset := range library.Particles.Presets {
		if particleNames[preset.Name] {
			result.Particles.Presets = append(result.Particles.Presets, preset)
			for _, frame := range preset.Sprite.Frames {
				textureNames[frame] = true
			}
		}
	}
	if library.Animations != nil {
		for _, preset := range library.Animations.Presets {
			if animationNames[preset.Name] {
				result.Animations.Presets = append(result.Animations.Presets, preset)
				for _, frame := range preset.Frames {
					textureNames[frame] = true
				}
			}
		}
	}
	frames := make([]string, 0, len(textureNames))
	for name := range textureNames {
		frames = append(frames, name)
	}
	sort.Strings(frames)
	return result, frames, nil
}

func effectInstance(id, prefab string, x, y, z, scale float64) worldsource.Instance {
	return worldsource.Instance{
		ID:        id,
		Prefab:    prefab,
		Transform: worldsource.Transform{Translation: worldsource.Vec3{X: x, Y: y, Z: z}, Scale: scale},
	}
}

func marshalYAML(value any) []byte {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(value); err != nil {
		panic(err)
	}
	if err := encoder.Close(); err != nil {
		panic(err)
	}
	return output.Bytes()
}

func indent(data []byte, spaces int) []byte {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	for index := range lines {
		lines[index] = prefix + lines[index]
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func replaceBlock(data []byte, name string, content []byte, anchor, legacyEnd string) ([]byte, error) {
	padding := ""
	if name == "prefabs" || name == "instances" {
		padding = "  "
	}
	if name == "animations" {
		padding = "    "
	}
	begin := padding + "# BEGIN PARTICLE LAB EFFECTS: " + name + "\n"
	end := padding + "# END PARTICLE LAB EFFECTS: " + name + "\n"
	block := []byte(begin + string(content) + end)
	start, finish := bytes.Index(data, []byte(begin)), bytes.Index(data, []byte(end))
	if start >= 0 || finish >= 0 {
		if start < 0 || finish < start || bytes.Count(data, []byte(begin)) != 1 || bytes.Count(data, []byte(end)) != 1 {
			return nil, fmt.Errorf("invalid generated %s markers", name)
		}
		return append(append(append([]byte(nil), data[:start]...), block...), data[finish+len(end):]...), nil
	}
	if name == "textures" {
		return append(append(append([]byte(nil), data...), []byte("\n")...), block...), nil
	}
	// Match the complete line indentation. A nested prefab may also contain
	// an instances list; it must never become the world insertion point.
	start = bytes.Index(data, []byte("\n"+anchor))
	if start >= 0 {
		start++
	} else if bytes.HasPrefix(data, []byte(anchor)) {
		start = 0
	}
	if start < 0 {
		return nil, fmt.Errorf("missing %s insertion anchor", name)
	}
	if legacyEnd != "" {
		finish = bytes.Index(data[start:], []byte(legacyEnd))
		if finish < 0 {
			return nil, fmt.Errorf("missing legacy %s boundary", name)
		}
		return append(append(append([]byte(nil), data[:start]...), block...), data[start+finish:]...), nil
	}
	if name != "animations" {
		start += len(anchor)
	}
	return append(append(append([]byte(nil), data[:start]...), block...), data[start:]...), nil
}

func removeTexture(data []byte, name string) []byte {
	blocks := strings.Split(string(data), "[[textures]]")
	var output strings.Builder
	output.WriteString(blocks[0])
	for _, block := range blocks[1:] {
		if strings.Contains(block, "\nname = "+strconv.Quote(name)+"\n") {
			continue
		}
		output.WriteString("[[textures]]")
		output.WriteString(block)
	}
	return []byte(output.String())
}

func validateScene(world, manifest []byte) error {
	var level struct {
		Textures []struct{ Name, Source string }
	}
	if err := toml.Unmarshal(manifest, &level); err != nil {
		return err
	}
	textures := map[string]uint32{}
	for index, texture := range level.Textures {
		if textures[texture.Name] != 0 {
			return fmt.Errorf("duplicate texture %q", texture.Name)
		}
		textures[texture.Name] = uint32(index + 1)
	}
	expanded, err := source.Decode(world)
	if err != nil {
		return err
	}
	compiled, err := geometry.Compile(expanded, textures)
	if err != nil {
		return err
	}
	particles, err := geometry.CompileParticles(expanded, &compiled, textures)
	if err != nil {
		return err
	}
	capacities := map[string]uint32{}
	for _, preset := range particles.Presets {
		capacities[preset.Name] = preset.Capacity
	}
	var slots uint32
	for _, emitter := range particles.Emitters {
		slots += capacities[emitter.Preset]
	}
	fmt.Printf("Compiled effect confinement: %d emitters, %d slots.\n", len(particles.Emitters), slots)
	return nil
}

func publishEffectFile(relative string, data []byte, check bool) error {
	if strings.HasSuffix(relative, ".yaml") {
		formatted, err := sampleformat.YAML(context.Background(), data)
		if err != nil {
			return err
		}
		data = formatted
	}

	path := filepath.Join(hangarRoot, relative)
	if check {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("%s is stale; run mise run generate-hangar-effects", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

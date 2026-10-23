package packages

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func TestFPSKitDecalsArchive(t *testing.T) {
	t.Parallel()

	archive := fpsKitArchive(t)
	game := t.TempDir()
	write(t, filepath.Join(game, "karty.toml"), []byte("[project]\nname = 'fps-decals-check'\n"))

	if _, err := Import(t.Context(), game, archive, false); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}

	set := restore(t, game)
	names := fpsDecalNames()

	merged, textures, err := ResolveWorld(t.Context(), set, fpsDecalWorld(names), []string{"surface"})
	if err != nil {
		t.Fatal(err)
	}

	if len(set.Items["fps-kit"].Manifest.Textures) != 85 || len(textures) != len(names) {
		t.Fatalf("archive must expose 85 textures and select exactly 36 decals, got %d / %d",
			len(set.Items["fps-kit"].Manifest.Textures), len(textures))
	}

	materials := assertFPSDecalTextures(t, textures, names)

	expanded, err := source.Decode(merged)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := world.Encode(compiled)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := world.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if len(decoded.Contents) != len(names) || len(decoded.Sectors) != 1 {
		t.Fatal("decal consumer lost its room or markers")
	}

	for _, content := range decoded.Contents {
		actor := content.Actor
		if !slices.Contains(names, content.SourceID) || actor == nil || actor.Sprite == nil {
			t.Fatalf("invalid decal content: %+v", content)
		}

		sprite := actor.Sprite

		if sprite.AssetID != materials["fps-kit."+content.SourceID] || sprite.Facing != world.SpriteFixed ||
			sprite.Alpha != world.SpriteCutout || sprite.Width != 1 || sprite.Height != 1 {
			t.Fatalf("decal %s lost its selected texture or sprite settings", content.SourceID)
		}
	}

	if strings.Contains(string(merged), "prefabs:") || strings.Contains(string(merged), "fps-kit.wall-") {
		t.Fatal("decal consumer imported unused kit prefabs or surfaces")
	}

	assertFPSDecalSelection(t, set, names[0])
	assertFPSDecalGallery(t, set)
}

func fpsDecalNames() []string {
	names := make([]string, 0, 36)
	names = append(names,
		"decal-shape-cross", "decal-shape-triangle", "decal-shape-circle", "decal-shape-square",
		"decal-feedback-nice", "decal-feedback-great", "decal-feedback-ok", "decal-feedback-yes",
		"decal-feedback-no", "decal-feedback-wrong",
		"decal-icon-question", "decal-icon-exclamation", "decal-icon-eye", "decal-icon-bomb",
		"decal-icon-explosion", "decal-icon-rocket", "decal-icon-star", "decal-icon-heart",
		"decal-icon-flag", "decal-icon-target", "decal-icon-arrow", "decal-icon-key",
		"decal-icon-lock", "decal-icon-skull", "decal-icon-waypoint", "decal-icon-check",
	)

	for number := range 10 {
		names = append(names, fmt.Sprintf("decal-number-%d", number))
	}

	return names
}

func fpsDecalWorld(names []string) []byte {
	var contents strings.Builder

	contents.WriteString(`version: 11
rooms:
  - id: room
    floor: {c: 0}
    ceiling: {c: 4}
    floor_material: surface
    ceiling_material: surface
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 8, y: 0}, material: surface}
      - {id: east, start: {x: 8, y: 0}, end: {x: 8, y: 8}, material: surface}
      - {id: north, start: {x: 8, y: 8}, end: {x: 0, y: 8}, material: surface}
      - {id: west, start: {x: 0, y: 8}, end: {x: 0, y: 0}, material: surface}
    contents:
`)

	for index, name := range names {
		fmt.Fprintf(&contents, `      - id: %s
        kind: prop
        position: {x: %d, y: %d, z: 1}
        actor:
          sprite: {texture: fps-kit.%s, facing: fixed, alpha: cutout, width: 1, height: 1}
`, name, index%6+1, index/6+1, name)
	}

	return []byte(contents.String())
}

func assertFPSDecalTextures(t *testing.T, textures []WorldTexture, names []string) map[string]uint32 {
	t.Helper()

	materials := map[string]uint32{"surface": 1}

	for index, texture := range textures {
		if texture.Profile != "sprite" || !slices.Contains(names, strings.TrimPrefix(texture.Name, "fps-kit.")) {
			t.Fatalf("unexpected decal export %s (%s)", texture.Name, texture.Profile)
		}

		img, err := png.Decode(bytes.NewReader(texture.Data))
		if err != nil {
			t.Fatal(err)
		}

		if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
			t.Fatalf("decal %s is not 256x256", texture.Name)
		}

		transparent, visible := false, false

		for y := range 256 {
			for x := range 256 {
				_, _, _, alpha := img.At(x, y).RGBA()
				transparent = transparent || alpha == 0
				visible = visible || alpha > 0
			}
		}

		if !transparent || !visible {
			t.Fatalf("decal %s must contain both transparent background and visible artwork", texture.Name)
		}

		materials[texture.Name] = uint32(index + 2)
	}

	return materials
}

func assertFPSDecalSelection(t *testing.T, set *Set, name string) {
	t.Helper()

	input := fpsDecalWorld([]string{name})

	_, textures, err := ResolveWorld(t.Context(), set, input, []string{"surface"})
	if err != nil {
		t.Fatal(err)
	}

	if len(textures) != 1 || textures[0].Name != "fps-kit."+name {
		t.Fatal("selecting one decal must prune every other package export")
	}

	if _, _, err := ResolveWorld(t.Context(), set, input, []string{"surface", "fps-kit." + name}); err == nil {
		t.Fatal("decal selection accepted a local/package texture collision")
	}

	for _, invalid := range []string{"missing-kit." + name, "fps-kit.decal-missing"} {
		broken := bytes.ReplaceAll(input, []byte("fps-kit."+name), []byte(invalid))
		if _, _, err := ResolveWorld(t.Context(), set, broken, []string{"surface"}); err == nil {
			t.Fatalf("decal selection accepted unknown texture %s", invalid)
		}
	}
}

func assertFPSDecalGallery(t *testing.T, set *Set) {
	t.Helper()

	contents, err := Read(t.Context(), set.Items["fps-kit"].Root, "examples/decals/world.yaml", world.MaxEncodedSize)
	if err != nil {
		t.Fatal(err)
	}

	merged, textures, err := ResolveWorld(t.Context(), set, contents, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 36 decals, three room surfaces and three runtime crate materials.
	if len(textures) != 42 {
		t.Fatalf("gallery selects %d textures, want 42", len(textures))
	}

	materials := make(map[string]uint32, len(textures))
	for index, texture := range textures {
		materials[texture.Name] = uint32(index + 1)
	}

	expanded, err := source.Decode(merged)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.Contents) != 38 || compiled.SolidRenderers == nil {
		t.Fatal("gallery lost its wall/floor decals or FPS runtime crate renderer")
	}

	floorDecals, goals := 0, 0

	for _, content := range compiled.Contents {
		actor := content.Actor
		if actor == nil || actor.Sprite == nil || actor.Sprite.Facing != world.SpriteFixed {
			t.Fatalf("gallery marker %s is not a fixed sprite", content.ID)
		}

		if strings.HasPrefix(content.SourceID, "floor-") {
			floorDecals++

			if math.Abs(actor.Pitch-math.Pi/2) > 1e-9 || content.Position.Z != 0.015 {
				t.Fatalf("gallery floor decal %s lost its orientation or surface offset", content.ID)
			}
		}

		if slices.Contains(actor.Tags, "goal") && slices.Contains(actor.Tags, "placeholder") {
			goals++
		}
	}

	if floorDecals != 2 || goals != 1 {
		t.Fatal("gallery must demonstrate two floor decals and a tagged placeholder goal")
	}
}

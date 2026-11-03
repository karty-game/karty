package levelbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
)

func TestWebGPUConvertedSourcesOmitOriginalsButRetainActorConsumers(t *testing.T) {
	t.Parallel()

	// A converted original is absent from the final material layout; it still
	// must be omitted unless another runtime consumer explicitly needs it.
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "world.yaml"), []byte("version: 5\n"+lightingRoomYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	layout, err := worldmaterial.NewLayout([]uint32{9})
	if err != nil {
		t.Fatal(err)
	}

	layoutBytes, err := worldmaterial.EncodeLayout(layout)
	if err != nil {
		t.Fatal(err)
	}
	// Use a valid fixture world's serialized topology, independently of build
	// cache/SDK installation and source texture processors.
	fixture, _, err := compileMaterialWorld(
		directory,
		manifest{World: worldSettings{Source: "world.yaml"}},
		[]metadataTexture{{ID: 1, Name: "wall"}},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	fixture.Contents = nil

	encoded, err := world.Encode(fixture)
	if err != nil {
		t.Fatal(err)
	}

	entries := []level.SourceEntry{
		{Name: world.EntryName, Kind: level.EntryData, Data: encoded},
		{Name: worldmaterial.LayoutEntry, Kind: level.EntryData, Data: layoutBytes},
		{Name: level.TextureEntryName(1), Kind: level.EntryTexture, Data: []byte{1}},
		{Name: level.TextureEntryName(2), Kind: level.EntryTexture, Data: []byte{2}},
	}

	result, err := omitAtlasSourceTextures(entries, []uint32{1})
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 3 || result[2].Name != level.TextureEntryName(2) {
		t.Fatal("dead original retained or unrelated sprite removed")
	}

	fixture.Contents = []world.Content{
		{ID: "sprite", SourceID: "sprite", Kind: "actor", Sector: 0, Position: world.Vec3{X: 1, Y: 1, Z: 1}, Actor: &world.Actor{
			Scale: world.Vec3{
				X: 1,
				Y: 1,
				Z: 1,
			},
			Sprite: &world.Sprite{AssetID: 1, Facing: world.SpriteFixed, Alpha: world.SpriteCutout, Width: 1, Height: 1},
		}},
	}

	encoded, err = world.Encode(fixture)
	if err != nil {
		t.Fatal(err)
	}

	entries[0].Data = encoded

	result, err = omitAtlasSourceTextures(entries, []uint32{1})
	if err != nil || len(result) != 4 {
		t.Fatal("actor's shared original texture removed", err)
	}
}

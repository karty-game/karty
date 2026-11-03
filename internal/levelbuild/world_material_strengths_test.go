package levelbuild

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldmaterial"
	"github.com/pelletier/go-toml/v2"
)

func TestMaterialStrengthTOMLOmissionBoundsAndCapability(t *testing.T) {
	t.Parallel()

	var definition manifest
	if err := toml.Unmarshal([]byte(`
[world]
source="world.yaml"
[[textures]]
name="wall"
source="wall.png"
[textures.material_strengths]
normal=0
height=4
`), &definition); err != nil {
		t.Fatal(err)
	}

	got, err := definition.Textures[0].MaterialStrengths.resolved()
	if err != nil || got != (worldmaterial.Strengths{Normal: 0, Height: 4, AO: 1, Rim: 1}) {
		t.Fatalf("explicit zero or omitted default lost: %+v %v", got, err)
	}

	assets := &assetBuild{}

	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1}
	if err := validateMaterialStrengths(definition, assets); err != nil {
		t.Fatal(err)
	}

	for _, unsupported := range []*assetBuild{nil, {}} {
		if err := validateMaterialStrengths(definition, unsupported); !errors.Is(err, ErrManifest) {
			t.Fatal("unsupported SDK silently ignored material controls")
		}
	}

	withoutWorld := definition

	withoutWorld.World.Source = ""
	if err := validateMaterialStrengths(withoutWorld, assets); !errors.Is(err, ErrManifest) {
		t.Fatal("worldless strengths silently ignored")
	}

	for _, invalid := range []float64{-0.001, 4.001, math.NaN(), math.Inf(1), math.Inf(-1)} {
		definition.Textures[0].MaterialStrengths.Rim = &invalid
		if err := validateMaterialStrengths(definition, assets); !errors.Is(err, ErrManifest) {
			t.Fatalf("invalid final control accepted: %v", invalid)
		}
	}

	definition.Textures[0].MaterialStrengths = nil
	if err := validateMaterialStrengths(definition, nil); err != nil {
		t.Fatal("omitted controls changed released SDK behavior")
	}
}

func TestMaterialStrengthIDMappingAndOwnership(t *testing.T) {
	t.Parallel()

	zero := float64(0)
	entries := []textureEntry{{Name: "wall", MaterialStrengths: &materialStrengths{Normal: &zero}}}
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 9, CeilingMaterial: 3}}}

	resolved, err := resolveMaterialStrengths(entries, []metadataTexture{{Name: "wall", ID: 9}}, document)
	if err != nil || resolved[9] == nil || *resolved[9] != (worldmaterial.Strengths{Height: 1, AO: 1, Rim: 1}) {
		t.Fatalf("material source-name mapping: %+v %v", resolved, err)
	}

	zero = 4

	if resolved[9].Normal != 0 {
		t.Fatal("compiled controls alias author configuration")
	}

	if _, err := resolveMaterialStrengths(entries, []metadataTexture{{Name: "wall", ID: 7}}, document); !errors.Is(err, ErrManifest) {
		t.Fatal("unused surface controls silently ignored")
	}

	if _, err := resolveMaterialStrengths(entries, nil, document); !errors.Is(err, ErrManifest) {
		t.Fatal("missing source texture controls silently ignored")
	}
}

func TestMaterialStrengthBinaryLayoutPreservesMaximumPrecision(t *testing.T) {
	t.Parallel()

	precision := 1.123456789012345
	entries := make([]textureEntry, worldmaterial.MaxMaterials-1)
	metadata := make([]metadataTexture, len(entries))
	document := world.Document{Sectors: []world.Sector{{}}}

	for index := range entries {
		materialID := uint32(index + 1)
		name := strconv.Itoa(index)
		entries[index] = textureEntry{
			Name:              name,
			MaterialStrengths: &materialStrengths{Normal: &precision, Height: &precision, AO: &precision, Rim: &precision},
		}
		metadata[index] = metadataTexture{ID: materialID, Name: name}
		document.Sectors[0].Walls = append(document.Sectors[0].Walls, world.Wall{Material: materialID})
	}

	resolved, err := resolveMaterialStrengths(entries, metadata, document)
	if err != nil || len(resolved) != len(entries) {
		t.Fatalf("compact binary layout lost valid complete controls: %v", err)
	}

	for _, controls := range resolved {
		if controls.Normal != precision || controls.Height != precision || controls.AO != precision || controls.Rim != precision {
			t.Fatal("binary material strengths lost precision")
		}
	}
}

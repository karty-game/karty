package levelbuild

import (
	"errors"
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/pelletier/go-toml/v2"
)

func TestMaterializeAuthoring(t *testing.T) {
	t.Parallel()

	contents := []byte(`[world]
source="world.yaml"
[[textures]]
name="steel"
source="steel.png"
[textures.materialize]
preset="metal"
normal_strength=0
height_contrast=1.25
`)

	var definition manifest
	if err := toml.Unmarshal(contents, &definition); err != nil {
		t.Fatal(err)
	}

	if err := validateMaterializeKeys(contents); err != nil {
		t.Fatal(err)
	}

	options := definition.Textures[0].Materialize.options()
	if options.NormalStrength == nil || *options.NormalStrength != 0 || options.MetallicScale != nil {
		t.Fatal("omitted settings and explicit zero must remain distinct")
	}

	assets := &assetBuild{}

	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldMaterialAtlasV1}
	if err := validateMaterializeSettings(definition, assets); err != nil {
		t.Fatal(err)
	}

	if err := validateMaterializeSettings(definition, nil); !errors.Is(err, ErrManifest) {
		t.Fatal("unsupported build accepted options")
	}

	textures := []metadataTexture{{Name: "steel", ID: 7}}
	document := world.Document{Sectors: []world.Sector{{FloorMaterial: 7}}}

	resolved, err := resolveMaterializeSettings(definition.Textures, textures, document)
	if err != nil || resolved[7].Preset != "metal" {
		t.Fatalf("resolved=%v err=%v", resolved, err)
	}

	if _, err := resolveMaterializeSettings(definition.Textures, textures, world.Document{}); !errors.Is(err, ErrManifest) {
		t.Fatal("unused texture accepted")
	}

	secondaryOnly := world.Document{Sectors: []world.Sector{{FloorSecondary: &world.SurfaceSecondary{Material: 7}}}}
	if _, err := resolveMaterializeSettings(definition.Textures, textures, secondaryOnly); !errors.Is(err, ErrManifest) {
		t.Fatal("secondary-only material accepted generation controls")
	}

	definition.Textures[0].Materialize.Preset = "invalid"
	if err := validateMaterializeSettings(definition, assets); !errors.Is(err, ErrManifest) {
		t.Fatal("unknown preset accepted")
	}

	definition.Textures[0].Materialize.Preset = "metal"
	for _, value := range []float64{-1, 17, math.Inf(1), math.NaN()} {
		definition.Textures[0].Materialize.NormalStrength = &value
		if err := validateMaterializeSettings(definition, assets); !errors.Is(err, ErrManifest) {
			t.Fatalf("invalid value %v accepted", value)
		}
	}

	for _, key := range []string{"seamless", "normal_strength"} {
		if err := validateMaterializeKeys(append(append([]byte{}, contents...), []byte(key+"=true\n")...)); !errors.Is(err, ErrManifest) {
			t.Fatalf("unknown key %q accepted", key)
		}
	}
}

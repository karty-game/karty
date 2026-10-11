package levelbuild

import (
	"math"
	"testing"

	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/world"
	"github.com/pelletier/go-toml/v2"
)

func TestMaterialEmissionAuthoringAndCapability(t *testing.T) {
	t.Parallel()

	var definition manifest
	if err := toml.Unmarshal([]byte(`[world]
source="world.yaml"
[[textures]]
name="acid"
source="acid.png"
[textures.emission]
intensity=2
lights=["acid-fill"]
[textures.emission.pulse]
depth=0.25
period_seconds=4
phase_seconds=1
`), &definition); err != nil {
		t.Fatal(err)
	}

	if err := validateMaterialEmission(definition, nil); err == nil {
		t.Fatal("missing SDK capability accepted")
	}

	assets := &assetBuild{}

	assets.manifest.Assets.Capabilities.Runtime = []asset.Capability{asset.CapabilityWorldEmissionV1}
	if err := validateMaterialEmission(definition, assets); err != nil {
		t.Fatal(err)
	}

	document := world.Document{
		Version:  world.Version,
		Sectors:  []world.Sector{{FloorMaterial: 9}},
		Lighting: &world.Lighting{Version: 1, Lights: []world.PointLight{{ID: "acid-fill", Radius: 1}}},
	}
	if err := compileMaterialEmission(&document, definition.Textures, []metadataTexture{{Name: "acid", ID: 9}}); err != nil {
		t.Fatal(err)
	}

	got := document.Emission.Materials[0]
	if got.Material != 9 || got.Intensity != 64 || got.Pulse.Depth != .25 || got.Pulse.PeriodSeconds != 4 || got.Pulse.PhaseSeconds != 1 {
		t.Fatalf("emission=%+v", got)
	}

	definition.Textures[0].Emission.Lights[0] = "other"
	if got.Lights[0] != "acid-fill" {
		t.Fatal("compiled light links alias authoring")
	}

	definition.Textures[0].Emission.Pulse.Depth = .5
	if got.Pulse.Depth != .25 {
		t.Fatal("compiled emission aliases authoring")
	}

	definition.Textures[0].Emission.Intensity = math.Inf(1)
	if err := validateMaterialEmission(definition, assets); err == nil {
		t.Fatal("nonfinite intensity accepted")
	}

	definition.Textures[0].Emission.Intensity = 1

	definition.World.Source = ""
	if err := validateMaterialEmission(definition, assets); err == nil {
		t.Fatal("nonworld emission accepted")
	}

	if err := compileMaterialEmission(&document, definition.Textures, []metadataTexture{{Name: "acid", ID: 99}}); err == nil {
		t.Fatal("unused material accepted")
	}
}

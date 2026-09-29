package sdk

import (
	"testing"

	assetcontract "github.com/karty-game/karty-sdk/format/asset"
)

func TestValidateAssetsRequiresDeclaredProcessorAndRuntimeCapabilities(t *testing.T) {
	t.Parallel()

	manifest := Manifest{}
	manifest.Assets.Capabilities = assetcontract.Capabilities{
		Processors: []assetcontract.Processor{assetcontract.ProcessorQOAv1, assetcontract.ProcessorQOIv1},
		Runtime: []assetcontract.Capability{
			assetcontract.CapabilitySoundQOAv1,
			assetcontract.CapabilityTextureQOIv1,
		},
	}
	manifest.Assets.TextureProfiles = map[string]AssetProfile{
		"sprite": {
			Processor: assetcontract.ProcessorQOIv1,
			Transform: assetcontract.ImageRecipe{Filter: assetcontract.ImageFilterNearest, BitDepth: 8},
		},
	}
	manifest.Assets.SoundProfiles = map[string]SoundProfile{
		"effect": {
			Processor: assetcontract.ProcessorQOAv1,
			Transform: assetcontract.AudioRecipe{SampleRate: 48_000, ChannelMode: assetcontract.ChannelPreserve},
		},
	}

	if err := validateAssets(manifest); err != nil {
		t.Fatal(err)
	}

	manifest.Assets.Capabilities.Runtime = []assetcontract.Capability{assetcontract.CapabilityTextureQOIv1}
	if err := validateAssets(manifest); err == nil {
		t.Fatal("validateAssets() accepted a sound profile without host QOA capability")
	}
}

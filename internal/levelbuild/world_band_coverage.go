package levelbuild

import (
	"fmt"
	"image"

	"github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/level"
	"github.com/karty-game/karty-sdk/format/world"
)

const bandOpaqueAlpha16 = 65535

// bandCoverage inspects authored coverage, independently of RGB and generated maps.
func bandCoverage(pixels image.Image) string {
	opaque, empty := true, true

	bounds := pixels.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := pixels.At(x, y).RGBA()
			opaque = opaque && alpha == bandOpaqueAlpha16
			empty = empty && alpha == 0
		}
	}

	if empty {
		return "empty"
	}

	if opaque {
		return "opaque"
	}

	return "masked"
}

// Classify only band IDs, decoding one bounded processed QOI at a time.
// Other level textures (including transparent sprites) keep their existing roles.
func classifyBandSources(document world.Document, textures []metadataTexture, sources []level.SourceEntry) error {
	bandIDs := make(map[uint32]bool)

	for _, sector := range document.Sectors {
		for _, wall := range sector.Walls {
			for _, region := range wall.FrameRegions {
				if region.Coverage != world.FrameCoverageMain {
					bandIDs[region.Material] = true
				}
			}
		}
	}

	classes := make(map[uint32]string)

	for _, entry := range sources {
		materialID, valid := level.TextureAssetID(entry.Name)
		if !valid || !bandIDs[materialID] {
			continue
		}

		_, pixels, decodeErr := qoi.Decode(entry.Data)
		if decodeErr != nil {
			return fmt.Errorf("band texture %d: %w", materialID, decodeErr)
		}

		classes[materialID] = bandCoverage(pixels)
	}

	for i := range textures {
		if value, found := classes[textures[i].ID]; found {
			textures[i].Coverage = value
		}
	}

	return nil
}

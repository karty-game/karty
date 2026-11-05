package levelbuild

const cellRNMLobes = cellRGBMAtlasPixelBytes / cellRGBMPixelBytes

// Compact only an entire spatially constant lobe, including its gutters.
// Chart-local constants do not establish this property. Geometry retains its
// original normalized atlas UVs: every lookup in a 1x1 layer returns its value.
func compactGPUCellLighting(lighting gpuCellLighting) gpuCellLighting {
	if lighting.width == 0 || lighting.height == 0 || lighting.width > 2048 || lighting.height > 2048 ||
		uint64(len(lighting.pixels)) != uint64(lighting.width)*uint64(lighting.height)*cellRGBMAtlasPixelBytes ||
		lighting.width == 1 && lighting.height == 1 {
		return lighting
	}

	planeBytes := len(lighting.pixels) / cellRNMLobes

	var compact [cellRGBMAtlasPixelBytes]byte

	for lobe := range 3 {
		plane := lighting.pixels[lobe*planeBytes : (lobe+1)*planeBytes]
		first := [4]byte(plane[:4])
		constant, black := true, true

		for offset := 0; offset < len(plane); offset += 4 {
			pixel := [4]byte(plane[offset : offset+4])
			constant = constant && pixel == first
			// Alpha zero denotes uncovered atlas padding. A covered black texel
			// uses alpha one or greater; both decode to zero radiance everywhere.
			black = black && (pixel[3] == 0 || pixel[0] == 0 && pixel[1] == 0 && pixel[2] == 0)
			if !constant && !black {
				return lighting
			}
		}

		if black {
			first = [4]byte{0, 0, 0, 1}
		}

		copy(compact[lobe*4:], first[:])
	}

	lighting.width, lighting.height = 1, 1
	lighting.pixels = compact[:]

	return lighting
}

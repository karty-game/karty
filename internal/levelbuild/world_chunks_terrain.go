package levelbuild

import (
	"fmt"
	"math"
	"slices"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldterrain"
)

// A cell belongs to one chunk, selected by its center and stable authored
// sector order. Shared edge vertices retain the original heights and normals.
// Complete terrain remains an offline transport input; only these bounded
// cropped documents are published in the corresponding child modules.
func partitionWorldChunkTerrain(terrain *worldterrain.Document, document *world.Document,
	chunks []worldchunk.PlannedChunk,
) (map[int]*worldterrain.Document, error) {
	const cellEndpoints = 2

	result := make(map[int]*worldterrain.Document)
	if terrain == nil {
		return result, nil
	}

	if err := worldterrain.ValidateForWorld(terrain, document); err != nil {
		return nil, err
	}

	sectorChunk := make(map[int]int)

	for _, chunk := range chunks {
		for _, sector := range chunk.Sectors {
			sectorChunk[sector] = chunk.ID
		}
	}

	owners := make([]int, len(terrain.Holes))
	for i := range owners {
		owners[i] = -1
	}

	cols := int(terrain.Columns) - 1
	for index, hole := range terrain.Holes {
		if hole != 0 {
			continue
		}

		column, row := index%cols, index/cols
		worldX := float64(terrain.Origin[0]+float32(column)*terrain.CellSize)/cellEndpoints +
			float64(terrain.Origin[0]+float32(column+1)*terrain.CellSize)/cellEndpoints

		worldY := float64(terrain.Origin[1]+float32(row)*terrain.CellSize)/cellEndpoints +
			float64(terrain.Origin[1]+float32(row+1)*terrain.CellSize)/cellEndpoints
		for _, sector := range terrain.ExteriorSectors {
			inside := true

			for _, wall := range document.Sectors[sector].Walls {
				if (wall.End.X-wall.Start.X)*(worldY-wall.Start.Y)-(wall.End.Y-wall.Start.Y)*(worldX-wall.Start.X) < -1e-5 {
					inside = false

					break
				}
			}

			if inside {
				owners[index] = sectorChunk[int(sector)]

				break
			}
		}

		if owners[index] < 0 {
			return nil, worldterrain.ErrTerrain
		}
	}

	for _, chunk := range chunks {
		cropped, err := cropChunkTerrain(terrain, owners, chunk.ID, chunk.Sectors)
		if err != nil {
			return nil, err
		}

		if cropped == nil {
			continue
		}

		if err := worldterrain.ValidateForWorld(cropped, document); err != nil {
			return nil, fmt.Errorf(
				"terrain chunk %d boundary must align with the terrain grid and contain every owned cell: %w",
				chunk.ID,
				err,
			)
		}

		result[chunk.ID] = cropped
	}

	return result, nil
}

func cropChunkTerrain(source *worldterrain.Document, owners []int, chunk int, sectors []int) (*worldterrain.Document, error) {
	cols := int(source.Columns) - 1
	minX, minY, maxX, maxY := cols, int(source.Rows)-1, 0, 0

	for index, owner := range owners {
		if owner == chunk {
			column, row := index%cols, index/cols
			minX, minY, maxX, maxY = min(minX, column), min(minY, row), max(maxX, column+1), max(maxY, row+1)
		}
	}

	if minX >= maxX || minY >= maxY {
		//nolint:nilnil // A chunk without active terrain cells has no terrain resource.
		return nil, nil
	}

	width, height := maxX-minX+1, maxY-minY+1
	result := &worldterrain.Document{
		Version:  worldterrain.Version,
		Origin:   [2]float32{source.Origin[0] + float32(minX)*source.CellSize, source.Origin[1] + float32(minY)*source.CellSize},
		CellSize: source.CellSize, Columns: uint16(width), Rows: uint16(height),
		Heights: make([]float32, width*height), Normals: make([][3]float32, width*height), Holes: make([]byte, (width-1)*(height-1)),
	}
	// Cropping must preserve the exact float32 lattice uploaded by the source.
	// Reject an unrepresentable origin instead of creating chunk-edge cracks.
	for column := range width {
		if result.Origin[0]+float32(column)*result.CellSize != source.Origin[0]+float32(minX+column)*source.CellSize {
			return nil, fmt.Errorf(
				"terrain chunk %d crop cannot preserve the exact X lattice; use an exactly representable origin/cell size or adjust chunk boundaries: %w",
				chunk,
				worldterrain.ErrTerrain,
			)
		}
	}

	for row := range height {
		if result.Origin[1]+float32(row)*result.CellSize != source.Origin[1]+float32(minY+row)*source.CellSize {
			return nil, fmt.Errorf(
				"terrain chunk %d crop cannot preserve the exact Y lattice; use an exactly representable origin/cell size or adjust chunk boundaries: %w",
				chunk,
				worldterrain.ErrTerrain,
			)
		}
	}

	for _, sector := range source.ExteriorSectors {
		if slices.Contains(sectors, int(sector)) {
			result.ExteriorSectors = append(result.ExteriorSectors, sector)
		}
	}

	for row := range height {
		for column := range width {
			from, to := (minY+row)*int(source.Columns)+minX+column, row*width+column

			result.Heights[to], result.Normals[to] = source.Heights[from], source.Normals[from]
			if column < width-1 && row < height-1 && owners[(minY+row)*cols+minX+column] != chunk {
				result.Holes[row*(width-1)+column] = 1
			}
		}
	}

	cropTerrainPatches(source, result, minX, minY, maxX, maxY)

	result.FarColour = cropTerrainImage(source.FarColour, float64(minX)/float64(cols), float64(minY)/float64(source.Rows-1),
		float64(maxX)/float64(cols), float64(maxY)/float64(source.Rows-1), false)
	if err := worldterrain.Validate(result); err != nil {
		return nil, err
	}

	return result, nil
}

func cropTerrainImage(source worldterrain.Image, minU, minV, maxU, maxV float64, weights bool) worldterrain.Image {
	if minU == 0 && minV == 0 && maxU == 1 && maxV == 1 {
		return source
	}

	width := max(2, int(math.Ceil((maxU-minU)*float64(source.Width-1)))+1)
	height := max(2, int(math.Ceil((maxV-minV)*float64(source.Height-1)))+1)

	result := worldterrain.Image{Width: uint16(width), Height: uint16(height), Pixels: make([]byte, width*height*4)}
	for row := range height {
		for column := range width {
			u := (minU + (maxU-minU)*float64(column)/float64(width-1)) * float64(source.Width-1)
			v := (minV + (maxV-minV)*float64(row)/float64(height-1)) * float64(source.Height-1)
			sampleX, sampleY := min(int(u), int(source.Width)-2), min(int(v), int(source.Height)-2)
			blendX, blendY := u-float64(sampleX), v-float64(sampleY)

			var (
				values [4]float64
				sum    int
			)

			for c := range 4 {
				at := func(column, row int) float64 { return float64(source.Pixels[(row*int(source.Width)+column)*4+c]) }
				values[c] = (at(sampleX, sampleY)*(1-blendX)+at(sampleX+1, sampleY)*blendX)*(1-blendY) + (at(sampleX, sampleY+1)*(1-blendX)+at(sampleX+1, sampleY+1)*blendX)*blendY
				result.Pixels[(row*width+column)*4+c] = byte(math.Floor(values[c]))
				sum += int(result.Pixels[(row*width+column)*4+c])
				values[c] -= math.Floor(values[c])
			}

			if weights {
				for sum < 255 {
					best := 0
					for c := 1; c < 4; c++ {
						if values[c] > values[best] {
							best = c
						}
					}

					result.Pixels[(row*width+column)*4+best]++
					values[best] = -1
					sum++
				}
			}
		}
	}

	return result
}

func cropTerrainPatches(source, result *worldterrain.Document, minX, minY, maxX, maxY int) {
	materialIDs := make(map[uint16]uint16)

	for _, patch := range source.Patches {
		px0, py0 := max(minX, int(patch.X)), max(minY, int(patch.Y))

		px1, py1 := min(maxX, int(patch.X+patch.Columns)), min(maxY, int(patch.Y+patch.Rows))
		if px0 >= px1 || py0 >= py1 {
			continue
		}

		local := worldterrain.Patch{X: uint16(px0 - minX), Y: uint16(py0 - minY), Columns: uint16(px1 - px0), Rows: uint16(py1 - py0)}

		for channel, old := range patch.Palette {
			materialID, found := materialIDs[old]
			if !found {
				materialID = uint16(len(result.Materials))
				result.Materials = append(result.Materials, source.Materials[old])
				materialIDs[old] = materialID
			}

			local.Palette[channel] = materialID
		}

		local.Weights = cropTerrainImage(patch.Weights,
			float64(px0-int(patch.X))/float64(patch.Columns), float64(py0-int(patch.Y))/float64(patch.Rows),
			float64(px1-int(patch.X))/float64(patch.Columns), float64(py1-int(patch.Y))/float64(patch.Rows), true)
		result.Patches = append(result.Patches, local)
	}

	slices.SortFunc(result.Patches, func(a, b worldterrain.Patch) int {
		if a.Y != b.Y {
			return int(a.Y) - int(b.Y)
		}

		return int(a.X) - int(b.X)
	})
}

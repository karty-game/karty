//nolint:gocognit,nestif,varnamelen // Compilation resolves the complete bounded room, edge and piece graph together.
package geometry

import (
	"fmt"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

// Resolve every authored choice before packaging. Runtime receives only the
// complete convex partition, one selected piece and its independent affine UVs.
func compileMaterialLayers(document *world.Document, expanded source.Expanded, materials map[string]uint32,
	roomSectors map[string][]uint32) error {
	for _, room := range expanded.Rooms {
		for _, sectorIndex := range roomSectors[room.ID] {
			sector := &document.Sectors[sectorIndex]

			var err error

			sector.FloorSecondary, err = compileSecondary(room.FloorSecondary, room.FloorUV, materials,
				func(uv *worldsource.UVSettings) *world.SurfaceUV { return bakeHorizontalUV(room.Floor, uv, false) })
			if err != nil {
				return err
			}

			sector.CeilingSecondary, err = compileSecondary(room.CeilingSecondary, room.CeilingUV, materials,
				func(uv *worldsource.UVSettings) *world.SurfaceUV { return bakeHorizontalUV(room.Ceiling, uv, true) })
			if err != nil {
				return err
			}

			if sector.FloorSecondary != nil || sector.CeilingSecondary != nil {
				document.MaterialLayers = &world.MaterialLayers{Version: 1}
			}

			for wi := range sector.Walls {
				wall := &sector.Walls[wi]
				if wall.SourceEdge == "" {
					continue
				}

				perimeter := 0.0

				for ei, edge := range room.Boundary {
					if edge.ID != wall.SourceEdge {
						perimeter += math.Hypot(edge.End.X-edge.Start.X, edge.End.Y-edge.Start.Y)

						continue
					}

					wall.Secondary, err = compileSecondary(
						edge.Secondary,
						edge.UV,
						materials,
						func(uv *worldsource.UVSettings) *world.SurfaceUV {
							copyRoom := room
							copyRoom.Boundary = append([]worldsource.Edge(nil), room.Boundary...)
							copyRoom.Boundary[ei].UV = uv
							baked, _ := bakeWallUV(copyRoom, edge.Start, edge.End, ei)

							return baked
						},
					)
					if err != nil {
						return err
					}

					if wall.Secondary != nil {
						document.MaterialLayers = &world.MaterialLayers{Version: 1}
					}

					if edge.Bands == nil || edge.Bands.Enabled != nil && !*edge.Bands.Enabled {
						break
					}

					config, err := compileFrameConfig(room, ei, materials, perimeter)
					if err != nil {
						return fmt.Errorf("room %q edge %q bands: %w", room.ID, edge.ID, err)
					}

					if config.Top.Height == 0 && config.Bottom.Height == 0 {
						break
					}

					wall.FrameCompiled = true

					profile, err := world.ProfileForWall(document, int(sectorIndex), wi)
					if err != nil {
						return err
					}

					wall.FrameRegions, err = world.CompileWallFrame(*wall, profile, config)
					if err != nil {
						return err
					}

					document.MaterialLayers = &world.MaterialLayers{Version: 1}

					break
				}
			}
		}
	}

	if len(world.MaterialIDs(document)) > MaxMaterials {
		return ErrMaterial
	}

	return nil
}

//nolint:nilnil // Disabled optional layers are deliberately represented by nil without an error.
func compileSecondary(settings *worldsource.SecondarySettings, main *worldsource.UVSettings, materials map[string]uint32,
	bake func(*worldsource.UVSettings) *world.SurfaceUV) (*world.SurfaceSecondary, error) {
	if settings == nil || settings.Enabled != nil && !*settings.Enabled {
		return nil, nil
	}

	if settings.Texture == "" || main == nil {
		return nil, ErrMaterial
	}

	id, err := materialID(materials, settings.Texture)
	if err != nil {
		return nil, err
	}

	strength := 1.0
	if settings.Strength != nil {
		strength = *settings.Strength
	}

	if strength == 0 {
		return nil, nil
	}

	resolved := *main
	zero := worldsource.Vec2{}
	rotation := 0.0
	resolved.Offset, resolved.RotationDegrees = &zero, &rotation

	if settings.UV != nil {
		uv := settings.UV
		if uv.Mode != "" {
			resolved.Mode = uv.Mode
		}

		if uv.Anchor != "" {
			resolved.Anchor = uv.Anchor
		}

		if uv.Scale != nil {
			resolved.Scale = uv.Scale
		}

		if uv.Offset != nil {
			resolved.Offset = uv.Offset
		}

		if uv.RotationDegrees != nil {
			resolved.RotationDegrees = uv.RotationDegrees
		}
	}

	result := &world.SurfaceSecondary{Material: id, Strength: strength, UV: bake(&resolved)}
	if err := world.ValidateSurfaceSecondary(result); err != nil {
		return nil, err
	}

	return result, nil
}

func compileFrameConfig(room source.Room, index int, materials map[string]uint32, perimeter float64) (world.WallFrameConfig, error) {
	bands := room.Boundary[index].Bands
	config := world.WallFrameConfig{PerimeterOffset: perimeter}
	horizontal := func(settings *worldsource.HorizontalBandSettings) (world.FrameHorizontal, error) {
		result := world.FrameHorizontal{}
		if settings == nil || settings.Enabled != nil && !*settings.Enabled {
			return result, nil
		}

		if settings.Height == nil {
			return result, worldsource.ErrBounds
		}

		id, err := materialID(materials, settings.Texture)
		if err != nil {
			return result, err
		}

		result.Height, result.Repeat = *settings.Height, 1
		if settings.RepeatWidth != nil {
			result.Repeat = *settings.RepeatWidth
		}

		if settings.Offset != nil {
			result.Offset = world.Vec2(*settings.Offset)
		}

		result.Piece = world.FramePiece{Material: id, Coverage: world.FrameCoverageMasked}

		return result, nil
	}

	var err error

	config.Top, err = horizontal(bands.Top)
	if err != nil {
		return config, err
	}

	config.Bottom, err = horizontal(bands.Bottom)

	return config, err
}

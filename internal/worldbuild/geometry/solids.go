package geometry

import (
	"fmt"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

func compileSolids(document *world.Document, expanded source.Expanded, materials map[string]uint32, used map[uint32]struct{}) error {
	if expanded.Solids == nil {
		return nil
	}

	if len(expanded.Solids) > world.MaxStaticSolids {
		return world.ErrBounds
	}

	payload := &world.StaticSolids{Version: world.StaticSolidsVersion, Items: make([]world.Solid, len(expanded.Solids))}
	for index, authored := range expanded.Solids {
		solid := world.Solid{
			ID:        authored.ID,
			Bottom:    compilePlane(authored.Bottom),
			Top:       compilePlane(authored.Top),
			Collision: authored.Collision,
		}
		if len(authored.Footprint) > world.MaxSolidVertices {
			return world.ErrBounds
		}

		solid.Footprint = make([]world.Vec2, len(authored.Footprint))
		for vertex, point := range authored.Footprint {
			solid.Footprint[vertex] = compilePoint(point)
		}

		var err error
		for _, surface := range []struct {
			name   string
			target *uint32
		}{{authored.SideMaterial, &solid.SideMaterial}, {authored.TopMaterial, &solid.TopMaterial}, {authored.BottomMaterial, &solid.BottomMaterial}} {
			*surface.target, err = materialID(materials, surface.name)
			if err != nil {
				return fmt.Errorf("solid %q: %w", authored.ID, err)
			}

			used[*surface.target] = struct{}{}
		}

		if err := compileSolidUV(&solid, authored, expanded.UV != nil); err != nil {
			return err
		}

		payload.Items[index] = solid
	}

	if err := world.ValidateStaticSolids(payload); err != nil {
		return err
	}

	document.StaticSolids = payload

	return nil
}

func compileLooseContents(
	document *world.Document,
	contents []source.Content,
	materials map[string]uint32,
	used map[uint32]struct{},
) error {
	if len(document.Contents)+len(contents) > world.MaxContents {
		return world.ErrBounds
	}

	for _, authored := range contents {
		// Stable compiled-sector order chooses the first containing volume, including
		// shared boundaries. Z containment is required as well as the footprint.
		sector := -1

		for index := range document.Sectors {
			candidate := &document.Sectors[index]
			if _, inside := contentSector(document.Sectors, []uint32{uint32(index)}, authored.Position); !inside {
				continue
			}

			floor := candidate.Floor.A*authored.Position.X + candidate.Floor.B*authored.Position.Y + candidate.Floor.C

			ceiling := candidate.Ceiling.A*authored.Position.X + candidate.Ceiling.B*authored.Position.Y + candidate.Ceiling.C
			if authored.Position.Z >= floor && authored.Position.Z <= ceiling {
				sector = index

				break
			}
		}

		if sector < 0 {
			return fmt.Errorf("content %q has no containing sector: %w", authored.ID, world.ErrContent)
		}

		compiled := world.Content{
			ID:       authored.ID,
			SourceID: authored.SourceID,
			Instance: authored.Instance,
			Kind:     authored.Kind,
			Sector:   uint32(sector),
			Position: world.Vec3(authored.Position),
		}
		if actor := authored.Actor; actor != nil {
			compiled.Actor = &world.Actor{
				Yaw:   actor.Yaw,
				Pitch: actor.Pitch,
				Roll:  actor.Roll,
				Scale: world.Vec3(actor.Scale),
				Tags:  append([]string(nil), actor.Tags...),
			}
			if sprite := actor.Sprite; sprite != nil {
				textureID, err := materialID(materials, sprite.Texture)
				if err != nil {
					return err
				}

				compiled.Actor.Sprite = &world.Sprite{
					AssetID: textureID,
					Facing:  world.SpriteFacing(sprite.Facing),
					Alpha:   world.SpriteAlpha(sprite.Alpha),
					Width:   sprite.Width,
					Height:  sprite.Height,
					OriginX: sprite.OriginX,
					OriginY: sprite.OriginY,
				}
				used[textureID] = struct{}{}
			}
		}

		document.Contents = append(document.Contents, compiled)
	}

	return nil
}

func compileSolidUV(solid *world.Solid, authored worldsource.Solid, enabled bool) error {
	if !enabled {
		if authored.TopUV != nil || authored.BottomUV != nil || authored.SideUV != nil {
			return worldsource.ErrUVMapping
		}

		return nil
	}

	if !completeUVSettings(authored.TopUV, true) || !completeUVSettings(authored.BottomUV, true) {
		return worldsource.ErrUVMapping
	}

	solid.TopUV = bakeHorizontalUV(authored.Top, authored.TopUV, false)

	solid.BottomUV = bakeHorizontalUV(authored.Bottom, authored.BottomUV, true)
	if authored.SideUV == nil {
		return nil
	}

	if !completeUVSettings(authored.SideUV, false) || authored.SideUV.Mode != worldsource.UVPlanar ||
		authored.SideUV.Anchor != worldsource.UVWorld {
		return worldsource.ErrUVMapping
	}
	// All sides share one global projection; arbitrary normals do not imply per-edge wrapping.
	solid.SideUV = bakeSurfaceUV(
		authored.SideUV,
		world.UVProjection{U: world.UVPlane{X: 1}, V: world.UVPlane{Z: 1}},
		world.UVPlane{Z: 1},
		world.Vec3{Y: 1},
	)

	return nil
}

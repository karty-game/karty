package source

import "github.com/karty-game/karty-sdk/format/worldsource"

// Resolve all authoring choices after geometric expansion. UV scale remains in
// world units per repeat: prefab transforms never rescale these settings.
func expandMaterialUV(expanded *Expanded, root *worldsource.UVSettings) {
	unit := worldsource.Vec2{X: 1, Y: 1}
	origin := worldsource.Vec2{}
	rotation := float64(0)
	defaults := worldsource.UVSettings{
		Mode: worldsource.UVTriplanar, Anchor: worldsource.UVWorld,
		Scale: &unit, Offset: &origin, RotationDegrees: &rotation,
	}
	expanded.UV = inheritUV(defaults, root)
	planeDefaults := *expanded.UV

	planeDefaults.Anchor = worldsource.UVWorld
	if planeDefaults.Mode == worldsource.UVWrap {
		planeDefaults.Mode = worldsource.UVPlanar
	}

	for index := range expanded.Solids {
		solid := &expanded.Solids[index]
		solid.TopUV = inheritUV(planeDefaults, solid.TopUV)

		solid.BottomUV = inheritUV(planeDefaults, solid.BottomUV)
		if solid.SideUV != nil {
			sideDefaults := planeDefaults
			sideDefaults.Mode = worldsource.UVPlanar
			solid.SideUV = inheritUV(sideDefaults, solid.SideUV)
		}
	}

	for index := range expanded.Rooms {
		room := &expanded.Rooms[index]
		room.FloorUV = inheritUV(planeDefaults, room.FloorUV)
		room.CeilingUV = inheritUV(planeDefaults, room.CeilingUV)

		room.WallUV = inheritUV(*expanded.UV, room.WallUV)
		for edge := range room.Boundary {
			room.Boundary[edge].UV = inheritUV(*room.WallUV, room.Boundary[edge].UV)
		}
	}
}

func inheritUV(parent worldsource.UVSettings, authored *worldsource.UVSettings) *worldsource.UVSettings {
	result := parent
	if authored != nil {
		result = overrideUV(parent, *authored)
	}

	// Each effective record owns every optional value, including inherited ones.
	if result.Scale != nil {
		owned := *result.Scale
		result.Scale = &owned
	}

	if result.Offset != nil {
		owned := *result.Offset
		result.Offset = &owned
	}

	if result.RotationDegrees != nil {
		owned := *result.RotationDegrees
		result.RotationDegrees = &owned
	}

	return &result
}

func overrideUV(result, authored worldsource.UVSettings) worldsource.UVSettings {
	if authored.Mode != "" {
		result.Mode = authored.Mode
	}

	if authored.Anchor != "" {
		result.Anchor = authored.Anchor
	}

	if authored.Scale != nil {
		result.Scale = authored.Scale
	}

	if authored.Offset != nil {
		result.Offset = authored.Offset
	}

	if authored.RotationDegrees != nil {
		result.RotationDegrees = authored.RotationDegrees
	}

	return result
}

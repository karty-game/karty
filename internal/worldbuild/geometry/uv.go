package geometry

import (
	"fmt"
	"math"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

const uvBlendPower = 4

func validateExpandedUV(expanded source.Expanded) error {
	declared := expanded.UV != nil
	if declared && !completeUVSettings(expanded.UV, false) {
		return worldsource.ErrUVMapping
	}

	for _, room := range expanded.Rooms {
		for _, settings := range []*worldsource.UVSettings{room.FloorUV, room.CeilingUV} {
			if !validExpandedUVSettings(settings, declared, true) {
				return fmt.Errorf("room %q floor/ceiling UV: %w", room.ID, worldsource.ErrUVMapping)
			}
		}

		if !validExpandedUVSettings(room.WallUV, declared, false) {
			return fmt.Errorf("room %q wall UV: %w", room.ID, worldsource.ErrUVMapping)
		}

		for _, edge := range room.Boundary {
			if !validExpandedUVSettings(edge.UV, declared, false) {
				return fmt.Errorf("room %q edge %q UV: %w", room.ID, edge.ID, worldsource.ErrUVMapping)
			}
		}
	}

	return nil
}

func validExpandedUVSettings(settings *worldsource.UVSettings, declared, horizontal bool) bool {
	if !declared {
		return settings == nil
	}

	return completeUVSettings(settings, horizontal)
}

func completeUVSettings(settings *worldsource.UVSettings, horizontal bool) bool {
	if worldsource.ValidateUVSettings(settings) != nil || settings.Mode == "" || settings.Anchor == "" ||
		settings.Scale == nil || settings.Offset == nil || settings.RotationDegrees == nil {
		return false
	}

	return !horizontal || (settings.Anchor == worldsource.UVWorld && settings.Mode != worldsource.UVWrap)
}

func bakeHorizontalUV(plane worldsource.Plane, settings *worldsource.UVSettings, ceiling bool) *world.SurfaceUV {
	normal := world.Vec3{X: -plane.A, Y: -plane.B, Z: 1}
	if ceiling {
		normal = world.Vec3{X: plane.A, Y: plane.B, Z: -1}
	}

	return bakeSurfaceUV(settings, world.UVProjection{U: world.UVPlane{X: 1}, V: world.UVPlane{Y: 1}}, world.UVPlane{Z: 1}, normal)
}

func bakeWallUV(room source.Room, start, end worldsource.Vec2, authoredIndex int) (*world.SurfaceUV, error) {
	settings := room.WallUV
	uPlane := world.UVPlane{}

	if authoredIndex >= 0 {
		original := room.Boundary[authoredIndex]
		settings = original.UV
		start, end = original.Start, original.End
	}

	length := math.Hypot(end.X-start.X, end.Y-start.Y)
	if length <= geometryEpsilon {
		return nil, worldsource.ErrUVMapping
	}

	uPlane.X, uPlane.Y = (end.X-start.X)/length, (end.Y-start.Y)/length

	if settings.Mode == worldsource.UVWrap && authoredIndex >= 0 {
		// Retain the authored edge's origin and cumulative boundary distance,
		// independently of decomposition, portal clipping or an emitted segment.
		for _, edge := range room.Boundary[:authoredIndex] {
			uPlane.Offset += math.Hypot(edge.End.X-edge.Start.X, edge.End.Y-edge.Start.Y)
		}

		uPlane.Offset -= uPlane.X*start.X + uPlane.Y*start.Y
	}

	vertical := wallVerticalUV(room, settings.Anchor)

	mapping := bakeSurfaceUV(settings, world.UVProjection{U: uPlane, V: vertical}, vertical,
		world.Vec3{X: -uPlane.Y, Y: uPlane.X})
	if err := world.ValidateSurfaceUV(mapping); err != nil {
		return nil, err
	}

	return mapping, nil
}

func wallVerticalUV(room source.Room, anchor worldsource.UVAnchor) world.UVPlane {
	switch anchor {
	case worldsource.UVWorld:
		return world.UVPlane{Z: 1}
	case worldsource.UVBottom:
		return world.UVPlane{X: -room.Floor.A, Y: -room.Floor.B, Z: 1, Offset: -room.Floor.C}
	case worldsource.UVTop:
		return world.UVPlane{X: room.Ceiling.A, Y: room.Ceiling.B, Z: -1, Offset: room.Ceiling.C}
	default:
		return world.UVPlane{Z: 1}
	}
}

func bakeSurfaceUV(
	settings *worldsource.UVSettings,
	planar world.UVProjection,
	vertical world.UVPlane,
	normal world.Vec3,
) *world.SurfaceUV {
	mapping := &world.SurfaceUV{Projections: []world.UVProjection{planar}, Weights: []float64{1}}
	if settings.Mode == worldsource.UVTriplanar {
		mapping.Projections = []world.UVProjection{
			{U: world.UVPlane{X: 1}, V: world.UVPlane{Y: 1}},
			{U: world.UVPlane{Y: 1}, V: vertical},
			{U: world.UVPlane{X: 1}, V: vertical},
		}
		weightX, weightY, weightZ := math.Pow(
			math.Abs(normal.X),
			uvBlendPower,
		), math.Pow(
			math.Abs(normal.Y),
			uvBlendPower,
		), math.Pow(
			math.Abs(normal.Z),
			uvBlendPower,
		)
		total := weightX + weightY + weightZ
		mapping.Weights = []float64{weightZ / total, weightX / total, weightY / total}
	}

	for index, projection := range mapping.Projections {
		mapping.Projections[index] = transformUVProjection(projection, settings)
	}

	return mapping
}

func transformUVProjection(projection world.UVProjection, settings *worldsource.UVSettings) world.UVProjection {
	sine, cosine := math.Sincos(*settings.RotationDegrees * math.Pi / degreesPerHalfTurnUV)
	u, v := projection.U, projection.V
	result := world.UVProjection{
		U: combineUVPlanes(u, v, cosine/settings.Scale.X, -sine/settings.Scale.Y),
		V: combineUVPlanes(u, v, sine/settings.Scale.X, cosine/settings.Scale.Y),
	}
	result.U.Offset += settings.Offset.X
	result.V.Offset += settings.Offset.Y

	return result
}

const degreesPerHalfTurnUV = 180

func combineUVPlanes(u, v world.UVPlane, uScale, vScale float64) world.UVPlane {
	return world.UVPlane{
		X: u.X*uScale + v.X*vScale, Y: u.Y*uScale + v.Y*vScale, Z: u.Z*uScale + v.Z*vScale,
		Offset: u.Offset*uScale + v.Offset*vScale,
	}
}

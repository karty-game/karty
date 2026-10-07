// Package geometry compiles expanded authored rooms into canonical convex sectors.
//
//nolint:wsl_v5,nlreturn // The staged compiler keeps related operations and loop exits together.
package geometry

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"github.com/karty-game/karty/internal/worldbuild/source"
)

var (
	ErrTriangulation = errors.New("world room cannot be triangulated")
	ErrMaterial      = errors.New("world material is not a packaged level texture")
	ErrPortal        = errors.New("world portal endpoints do not compile to one reciprocal edge pair")
)

// MaxMaterials matches the SDK 0.0.5 host atlas: 14 by 14 material cells.
const (
	MaxMaterials    = 196
	geometryEpsilon = 1e-9
)

type wallRef struct{ sector, wall int }

type edgeKey struct {
	ax, ay, bx, by uint64
}

// Compile triangulates every room and resolves internal and authored portals.
//
//nolint:gocognit,gocyclo,maintidx // The phases share bounded identity and portal maps that are easier to audit together.
func Compile(expanded source.Expanded, materials map[string]uint32) (sdkworld.Document, error) {
	document := sdkworld.Document{Version: sdkworld.Version}
	if err := validateExpandedUV(expanded); err != nil {
		return sdkworld.Document{}, err
	}
	if expanded.UV != nil {
		document.MaterialMapping = &sdkworld.MaterialMapping{Version: sdkworld.MaterialMappingVersion}
	}
	if expanded.Lighting != nil {
		if err := sdkworld.ValidateLighting(expanded.Lighting); err != nil {
			return sdkworld.Document{}, fmt.Errorf("world lighting: %w", err)
		}
		lighting := *expanded.Lighting
		lighting.Lights = slices.Clone(expanded.Lighting.Lights)
		for index := range lighting.Lights {
			if motion := lighting.Lights[index].Motion; motion != nil {
				owned := *motion
				lighting.Lights[index].Motion = &owned
			}
		}
		if expanded.Lighting.AmbientCube != nil {
			cube := *expanded.Lighting.AmbientCube
			lighting.AmbientCube = &cube
		}
		document.Lighting = &lighting
	}
	external := make(map[string]wallRef)
	internal := make(map[edgeKey]wallRef)
	roomSectors := make(map[string][]uint32, len(expanded.Rooms))

	for _, room := range expanded.Rooms {
		polygons, err := decompose(room.Boundary)
		if err != nil {
			return sdkworld.Document{}, fmt.Errorf("room %q: %w", room.ID, err)
		}
		floorMaterial, err := materialID(materials, room.FloorMaterial)
		if err != nil {
			return sdkworld.Document{}, fmt.Errorf("room %q floor: %w", room.ID, err)
		}
		ceilingMaterial, err := materialID(materials, room.CeilingMaterial)
		if err != nil {
			return sdkworld.Document{}, fmt.Errorf("room %q ceiling: %w", room.ID, err)
		}

		for polygonIndex, polygon := range polygons {
			sectorIndex := len(document.Sectors)
			if sectorIndex >= sdkworld.MaxSectors {
				return sdkworld.Document{}, sdkworld.ErrBounds
			}
			sector := sdkworld.Sector{
				ID: room.ID + "#" + strconv.Itoa(polygonIndex), SourceRoom: room.SourceRoom, Instance: room.Instance,
				Floor: compilePlane(room.Floor), Ceiling: compilePlane(room.Ceiling),
				FloorMaterial: floorMaterial, CeilingMaterial: ceilingMaterial,
				Walls: make([]sdkworld.Wall, len(polygon)),
			}
			if expanded.UV != nil {
				sector.FloorUV = bakeHorizontalUV(room.Floor, room.FloorUV, false)
				sector.CeilingUV = bakeHorizontalUV(room.Ceiling, room.CeilingUV, true)
			}
			for wallIndex := range polygon {
				startIndex, endIndex := polygon[wallIndex], polygon[(wallIndex+1)%len(polygon)]
				start, end := room.Boundary[startIndex].Start, room.Boundary[endIndex].Start
				wall := sdkworld.Wall{Start: compilePoint(start), End: compilePoint(end), Portal: -1}
				authoredIndex := -1
				if endIndex == (startIndex+1)%len(room.Boundary) {
					authoredIndex = startIndex
				}
				if expanded.UV != nil {
					wall.UV, err = bakeWallUV(room, start, end, authoredIndex)
					if err != nil {
						return sdkworld.Document{}, fmt.Errorf("room %q wall UV: %w", room.ID, err)
					}
				}
				if endIndex == (startIndex+1)%len(room.Boundary) {
					authored := room.Boundary[startIndex]
					wall.SourceEdge = authored.ID
					wall.Material, err = materialID(materials, authored.Material)
					if err != nil {
						return sdkworld.Document{}, fmt.Errorf("room %q edge %q: %w", room.ID, authored.ID, err)
					}
					external[endpointKey(room.ID, authored.ID)] = wallRef{sector: sectorIndex, wall: wallIndex}
				}
				sector.Walls[wallIndex] = wall
			}
			document.Sectors = append(document.Sectors, sector)
			roomSectors[room.ID] = append(roomSectors[room.ID], uint32(sectorIndex))
		}
	}
	usedMaterials := make(map[uint32]struct{})
	if err := compileSolids(&document, expanded, materials, usedMaterials); err != nil {
		return sdkworld.Document{}, err
	}
	for _, sector := range document.Sectors {
		usedMaterials[sector.FloorMaterial] = struct{}{}
		usedMaterials[sector.CeilingMaterial] = struct{}{}
		for _, wall := range sector.Walls {
			if wall.Material != 0 {
				usedMaterials[wall.Material] = struct{}{}
			}
		}
	}
	for sectorIndex := range document.Sectors {
		for wallIndex := range document.Sectors[sectorIndex].Walls {
			wall := &document.Sectors[sectorIndex].Walls[wallIndex]
			if wall.SourceEdge != "" {
				continue
			}
			key := canonicalEdge(wall.Start, wall.End)
			if other, ok := internal[key]; ok {
				otherWall := &document.Sectors[other.sector].Walls[other.wall]
				if otherWall.Start != wall.End || otherWall.End != wall.Start || otherWall.Portal >= 0 {
					return sdkworld.Document{}, ErrTriangulation
				}
				otherWall.Portal = int32(sectorIndex)
				otherWall.PortalWall = uint16(wallIndex + 1)
				wall.Portal = int32(other.sector)
				wall.PortalWall = uint16(other.wall + 1)
				delete(internal, key)
			} else {
				internal[key] = wallRef{sector: sectorIndex, wall: wallIndex}
			}
		}
	}
	if len(internal) != 0 {
		return sdkworld.Document{}, ErrTriangulation
	}

	for _, connection := range expanded.Connections {
		left, leftOK := external[endpointKey(connection.A.Room, connection.A.Edge)]
		right, rightOK := external[endpointKey(connection.B.Room, connection.B.Edge)]
		if !leftOK || !rightOK {
			return sdkworld.Document{}, fmt.Errorf("connection %q: %w", connection.ID, ErrPortal)
		}
		leftWall := &document.Sectors[left.sector].Walls[left.wall]
		rightWall := &document.Sectors[right.sector].Walls[right.wall]
		if !connection.NonEuclidean && (leftWall.Start != rightWall.End || leftWall.End != rightWall.Start) ||
			!equalEdgeLength(leftWall.Start, leftWall.End, rightWall.Start, rightWall.End) {
			return sdkworld.Document{}, fmt.Errorf("connection %q: %w", connection.ID, ErrPortal)
		}
		outgoingA := connection.Direction == "" || connection.Direction == worldsource.PortalBoth ||
			connection.Direction == worldsource.PortalAToB
		outgoingB := connection.Direction == "" || connection.Direction == worldsource.PortalBoth ||
			connection.Direction == worldsource.PortalBToA
		if outgoingA {
			if leftWall.Portal >= 0 {
				return sdkworld.Document{}, fmt.Errorf("connection %q endpoint a: %w", connection.ID, ErrPortal)
			}
			leftWall.Portal = int32(right.sector)
			leftWall.PortalWall = uint16(right.wall + 1)
		}
		if outgoingB {
			if rightWall.Portal >= 0 {
				return sdkworld.Document{}, fmt.Errorf("connection %q endpoint b: %w", connection.ID, ErrPortal)
			}
			rightWall.Portal = int32(left.sector)
			rightWall.PortalWall = uint16(left.wall + 1)
		}
	}

	for _, room := range expanded.Rooms {
		for _, content := range room.Contents {
			sector, found := contentSector(document.Sectors, roomSectors[room.ID], content.Position)
			if !found {
				return sdkworld.Document{}, fmt.Errorf("content %q is outside compiled room: %w", content.ID, sdkworld.ErrContent)
			}
			compiledContent := sdkworld.Content{
				ID: content.ID, SourceID: content.SourceID, Instance: content.Instance, Kind: content.Kind,
				Sector: sector, Position: sdkworld.Vec3{X: content.Position.X, Y: content.Position.Y, Z: content.Position.Z},
			}
			if content.Actor != nil {
				actor := content.Actor
				compiledActor := &sdkworld.Actor{
					Yaw: actor.Yaw, Pitch: actor.Pitch, Roll: actor.Roll,
					Scale: sdkworld.Vec3{X: actor.Scale.X, Y: actor.Scale.Y, Z: actor.Scale.Z},
					Tags:  append([]string(nil), actor.Tags...),
				}
				if actor.Sprite != nil {
					assetID, assetErr := materialID(materials, actor.Sprite.Texture)
					if assetErr != nil {
						return sdkworld.Document{}, fmt.Errorf("content %q sprite: %w", content.ID, assetErr)
					}
					compiledActor.Sprite = &sdkworld.Sprite{
						AssetID: assetID, Facing: sdkworld.SpriteFacing(actor.Sprite.Facing),
						Alpha: sdkworld.SpriteAlpha(actor.Sprite.Alpha), Width: actor.Sprite.Width,
						Height: actor.Sprite.Height, OriginX: actor.Sprite.OriginX, OriginY: actor.Sprite.OriginY,
					}
					usedMaterials[assetID] = struct{}{}
				}
				compiledContent.Actor = compiledActor
			}
			document.Contents = append(document.Contents, compiledContent)
		}
	}
	if err := compileLooseContents(&document, expanded.Contents, materials, usedMaterials); err != nil {
		return sdkworld.Document{}, err
	}
	if len(usedMaterials) > MaxMaterials {
		return sdkworld.Document{}, fmt.Errorf("%d materials exceeds %d: %w", len(usedMaterials), MaxMaterials, ErrMaterial)
	}

	if err := compileMaterialLayers(&document, expanded, materials, roomSectors); err != nil {
		return sdkworld.Document{}, fmt.Errorf("compile material layers: %w", err)
	}

	if err := sdkworld.Validate(&document); err != nil {
		return sdkworld.Document{}, fmt.Errorf("validate compiled world: %w", err)
	}

	return document, nil
}

func decompose(edges []worldsource.Edge) ([][]int, error) {
	triangles, err := triangulate(edges)
	if err != nil {
		return nil, err
	}

	polygons := make([][]int, len(triangles))
	for index, triangle := range triangles {
		polygons[index] = []int{triangle[0], triangle[1], triangle[2]}
	}

merge:
	for {
		for leftIndex := range polygons {
			for rightIndex := leftIndex + 1; rightIndex < len(polygons); rightIndex++ {
				merged, ok := mergeConvex(polygons[leftIndex], polygons[rightIndex], edges)
				if !ok {
					continue
				}

				polygons[leftIndex] = merged
				polygons = append(polygons[:rightIndex], polygons[rightIndex+1:]...)

				continue merge
			}
		}

		return polygons, nil
	}
}

func mergeConvex(left, right []int, edges []worldsource.Edge) ([]int, bool) {
	for leftEdge := range left {
		leftStart, leftEnd := left[leftEdge], left[(leftEdge+1)%len(left)]
		for rightEdge := range right {
			if leftStart != right[(rightEdge+1)%len(right)] || leftEnd != right[rightEdge] {
				continue
			}

			merged := make([]int, 1, len(left)+len(right)-2)
			merged[0] = leftStart
			for step := 1; step < len(right); step++ {
				merged = append(merged, right[(rightEdge+1+step)%len(right)])
			}
			for step := 2; step < len(left); step++ {
				merged = append(merged, left[(leftEdge+step)%len(left)])
			}
			if strictlyConvex(merged, edges) {
				return merged, true
			}
		}
	}

	return nil, false
}

func strictlyConvex(polygon []int, edges []worldsource.Edge) bool {
	if len(polygon) < 3 || len(polygon) > sdkworld.MaxWallsPerSector {
		return false
	}
	for index := range polygon {
		previous := edges[polygon[(index+len(polygon)-1)%len(polygon)]].Start
		current := edges[polygon[index]].Start
		next := edges[polygon[(index+1)%len(polygon)]].Start
		if cross(previous, current, next) <= geometryEpsilon {
			return false
		}
	}

	return true
}

func triangulate(edges []worldsource.Edge) ([][3]int, error) {
	indices := make([]int, len(edges))
	for index := range indices {
		indices[index] = index
	}
	triangles := make([][3]int, 0, len(edges)-2)
	for len(indices) > 3 {
		found := false
		for position := range indices {
			previous := indices[(position+len(indices)-1)%len(indices)]
			current := indices[position]
			next := indices[(position+1)%len(indices)]
			previousPoint, currentPoint, nextPoint := edges[previous].Start, edges[current].Start, edges[next].Start
			if cross(previousPoint, currentPoint, nextPoint) <= geometryEpsilon {
				continue
			}
			contains := false
			for _, candidate := range indices {
				if candidate == previous || candidate == current || candidate == next {
					continue
				}
				if pointInTriangle(edges[candidate].Start, previousPoint, currentPoint, nextPoint) {
					contains = true
					break
				}
			}
			if contains {
				continue
			}
			triangles = append(triangles, [3]int{previous, current, next})
			indices = append(indices[:position], indices[position+1:]...)
			found = true
			break
		}
		if !found {
			return nil, ErrTriangulation
		}
	}
	if len(indices) != 3 ||
		cross(edges[indices[0]].Start, edges[indices[1]].Start, edges[indices[2]].Start) <= geometryEpsilon {
		return nil, ErrTriangulation
	}
	triangles = append(triangles, [3]int{indices[0], indices[1], indices[2]})

	return triangles, nil
}

func pointInTriangle(point, a, b, c worldsource.Vec2) bool {
	const epsilon = 1e-9

	return cross(a, b, point) >= -epsilon && cross(b, c, point) >= -epsilon && cross(c, a, point) >= -epsilon
}

func cross(a, b, c worldsource.Vec2) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func materialID(materials map[string]uint32, name string) (uint32, error) {
	value, ok := materials[name]
	if !ok || value == 0 {
		return 0, fmt.Errorf("%q: %w", name, ErrMaterial)
	}

	return value, nil
}

func equalEdgeLength(leftStart, leftEnd, rightStart, rightEnd sdkworld.Vec2) bool {
	leftX, leftY := leftEnd.X-leftStart.X, leftEnd.Y-leftStart.Y
	rightX, rightY := rightEnd.X-rightStart.X, rightEnd.Y-rightStart.Y
	leftLength, rightLength := leftX*leftX+leftY*leftY, rightX*rightX+rightY*rightY

	return math.Abs(leftLength-rightLength) <= geometryEpsilon*max(1, leftLength, rightLength)
}

func endpointKey(room, edge string) string { return room + "\x00" + edge }

func canonicalEdge(start, end sdkworld.Vec2) edgeKey {
	left := [2]uint64{math.Float64bits(start.X), math.Float64bits(start.Y)}
	right := [2]uint64{math.Float64bits(end.X), math.Float64bits(end.Y)}
	if left[0] > right[0] || left[0] == right[0] && left[1] > right[1] {
		left, right = right, left
	}

	return edgeKey{ax: left[0], ay: left[1], bx: right[0], by: right[1]}
}

func compilePoint(value worldsource.Vec2) sdkworld.Vec2 {
	return sdkworld.Vec2{X: value.X, Y: value.Y}
}

func compilePlane(value worldsource.Plane) sdkworld.Plane {
	return sdkworld.Plane{A: value.A, B: value.B, C: value.C}
}

func contentSector(sectors []sdkworld.Sector, candidates []uint32, point worldsource.Vec3) (uint32, bool) {
	for _, sectorIndex := range candidates {
		sector := &sectors[sectorIndex]
		inside := true
		for _, wall := range sector.Walls {
			if (wall.End.X-wall.Start.X)*(point.Y-wall.Start.Y)-
				(wall.End.Y-wall.Start.Y)*(point.X-wall.Start.X) < -geometryEpsilon {
				inside = false
				break
			}
		}
		if inside {
			return sectorIndex, true
		}
	}

	return 0, false
}

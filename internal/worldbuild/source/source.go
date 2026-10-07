// Package source loads and expands authored world YAML into a flat room graph.
//
//nolint:wsl_v5 // Expansion keeps validation next to each bounded append and resolved identity.
package source

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldsource"
	"go.yaml.in/yaml/v3"
)

const degreesPerHalfTurn = 180

var (
	ErrPath      = errors.New("world source path is invalid")
	ErrYAML      = errors.New("world source YAML is invalid")
	ErrExpansion = errors.New("world source expansion is invalid")
)

type Content struct {
	ID, SourceID, Instance, Kind string
	Position                     worldsource.Vec3
	Actor                        *Actor
}

type Actor struct {
	Yaw, Pitch, Roll float64
	Scale            worldsource.Vec3
	Sprite           *worldsource.Sprite
	Tags             []string
}

type Room struct {
	ID, SourceRoom, Instance                        string
	Boundary                                        []worldsource.Edge
	Floor, Ceiling                                  worldsource.Plane
	FloorMaterial, CeilingMaterial                  string
	Contents                                        []Content
	FloorUV, CeilingUV, WallUV                      *worldsource.UVSettings
	WallBands                                       *worldsource.BandSettings
	FloorSecondary, CeilingSecondary, WallSecondary *worldsource.SecondarySettings
}

type Endpoint struct{ Room, Edge string }

type Connection struct {
	ID           string
	A, B         Endpoint
	Direction    worldsource.PortalDirection
	NonEuclidean bool
}

type Expanded struct {
	Rooms       []Room
	Connections []Connection
	// Lighting retains authored global coordinates and order independently of
	// prefab expansion. Its payload and light slice belong to this result.
	Lighting *sdkworld.Lighting
	// UV is nil for released source versions; v5 owns complete effective settings.
	UV       *worldsource.UVSettings
	Solids   []worldsource.Solid
	Contents []Content
}

type affine struct {
	x, y, z, yaw, scale float64
}

type scopeResult struct {
	rooms       []Room
	connections []Connection
	ports       map[string]Endpoint
	solids      []worldsource.Solid
	contents    []Content
}

type expander struct {
	prefabs map[string]*worldsource.Prefab
}

// Load reads one confined YAML file and expands all prefab instances.
func Load(root, relative string) (Expanded, error) {
	path, err := confinedPath(root, relative)
	if err != nil {
		return Expanded{}, err
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return Expanded{}, fmt.Errorf("read world source: %w", err)
	}
	if len(contents) > sdkworld.MaxEncodedSize {
		return Expanded{}, fmt.Errorf("world source is larger than %d bytes: %w", sdkworld.MaxEncodedSize, ErrYAML)
	}

	return Decode(contents)
}

// Decode strictly decodes and expands one complete YAML document.
func Decode(contents []byte) (Expanded, error) {
	if err := rejectAliases(contents); err != nil {
		return Expanded{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)

	var document worldsource.Document
	if err := decoder.Decode(&document); err != nil {
		return Expanded{}, fmt.Errorf("decode world source: %w: %w", err, ErrYAML)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Expanded{}, fmt.Errorf("multiple YAML documents: %w", ErrYAML)
		}

		return Expanded{}, fmt.Errorf("decode trailing YAML: %w: %w", err, ErrYAML)
	}

	if err := worldsource.Validate(&document); err != nil {
		return Expanded{}, fmt.Errorf("validate world source: %w", err)
	}

	prefabs := make(map[string]*worldsource.Prefab, len(document.Prefabs))
	for index := range document.Prefabs {
		prefabs[document.Prefabs[index].ID] = &document.Prefabs[index]
	}

	result, err := (&expander{prefabs: prefabs}).expandScope(
		"", document.Rooms, document.Instances, document.Connections, nil, document.Solids, document.Contents,
		affine{scale: 1}, func(value string) string { return value },
		nil,
	)
	if err != nil {
		return Expanded{}, err
	}
	if len(result.rooms) == 0 || len(result.rooms) > sdkworld.MaxSectors ||
		len(
			result.connections,
		) > worldsource.MaxConnections || len(result.solids) > sdkworld.MaxStaticSolids || len(result.contents) > sdkworld.MaxContents {
		return Expanded{}, fmt.Errorf("expanded rooms or connections exceed runtime bounds: %w", ErrExpansion)
	}

	expanded := Expanded{Rooms: result.rooms, Connections: result.connections, Solids: result.solids, Contents: result.contents}
	if document.Lighting != nil {
		expanded.Lighting = &sdkworld.Lighting{
			Version: document.Lighting.Version, Ambient: sdkworld.Vec3(document.Lighting.Ambient),
			Lights: make([]sdkworld.PointLight, len(document.Lighting.Lights)), Actors: document.Lighting.Actors,
		}
		if cube := document.Lighting.AmbientCube; cube != nil {
			expanded.Lighting.AmbientCube = &sdkworld.AmbientCube{
				PositiveX: sdkworld.Vec3(cube.PositiveX),
				NegativeX: sdkworld.Vec3(cube.NegativeX),
				PositiveY: sdkworld.Vec3(cube.PositiveY),
				NegativeY: sdkworld.Vec3(cube.NegativeY),
				PositiveZ: sdkworld.Vec3(cube.PositiveZ),
				NegativeZ: sdkworld.Vec3(cube.NegativeZ),
			}
		}
		for index, light := range document.Lighting.Lights {
			expanded.Lighting.Lights[index] = sdkworld.PointLight{
				ID: light.ID, Position: sdkworld.Vec3(light.Position),
				Color: sdkworld.Vec3(light.Color), Radius: light.Radius,
			}
			if motion := light.Motion; motion != nil {
				expanded.Lighting.Lights[index].Motion = &sdkworld.LightMotion{
					Version:       motion.Version,
					Offset:        sdkworld.Vec3(motion.Offset),
					PeriodSeconds: motion.PeriodSeconds,
				}
			}
		}
	}

	contentCount := len(expanded.Contents)
	for _, room := range expanded.Rooms {
		contentCount += len(room.Contents)
	}
	if contentCount > sdkworld.MaxContents {
		return Expanded{}, ErrExpansion
	}
	if document.Version >= worldsource.MappingVersion {
		expandMaterialUV(&expanded, document.UV)
	}

	return expanded, nil
}

func rejectAliases(contents []byte) error {
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("parse world source: %w: %w", err, ErrYAML)
	}

	var visit func(*yaml.Node) error
	visit = func(node *yaml.Node) error {
		if node.Kind == yaml.AliasNode || node.Anchor != "" {
			return fmt.Errorf("YAML aliases and anchors are not supported: %w", ErrYAML)
		}
		for _, child := range node.Content {
			if err := visit(child); err != nil {
				return err
			}
		}

		return nil
	}

	if err := visit(&document); err != nil {
		return err
	}
	if err := validateMaterialPresence(&document); err != nil {
		return err
	}

	return validateExtrasPresence(&document)
}

func confinedPath(root, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if relative == "" || filepath.IsAbs(relative) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPath
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve world root: %w", ErrPath)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, clean))
	if err != nil {
		return "", fmt.Errorf("resolve world source: %w", ErrPath)
	}
	relativeResolved, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || relativeResolved == ".." || strings.HasPrefix(relativeResolved, ".."+string(filepath.Separator)) {
		return "", ErrPath
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrPath
	}

	return resolved, nil
}

func (builder *expander) expandScope(
	prefix string,
	rooms []worldsource.Room,
	instances []worldsource.Instance,
	connections []worldsource.Connection,
	ports []worldsource.Port,
	solids []worldsource.Solid,
	contents []worldsource.Content,
	transform affine,
	material func(string) string,
	tagOverride []string,
) (scopeResult, error) {
	result := scopeResult{ports: make(map[string]Endpoint, len(ports))}
	direct := make(map[string]map[string]Endpoint, len(rooms))
	nested := make(map[string]map[string]Endpoint, len(instances))

	for _, solid := range solids {
		result.solids = append(result.solids, transformSolid(solid, prefix, transform, material))
	}
	for _, content := range contents {
		result.contents = append(result.contents, transformLooseContent(content, prefix, transform, material, tagOverride))
	}
	for _, authored := range rooms {
		room := transformRoom(authored, prefix, transform, material, tagOverride)
		if len(room.ID) > sdkworld.MaxIdentifierBytes {
			return scopeResult{}, fmt.Errorf("expanded room identity %q is too long: %w", room.ID, ErrExpansion)
		}
		result.rooms = append(result.rooms, room)
		edges := make(map[string]Endpoint, len(room.Boundary))
		for _, edge := range room.Boundary {
			edges[edge.ID] = Endpoint{Room: room.ID, Edge: edge.ID}
		}
		direct[authored.ID] = edges
	}

	for _, instance := range instances {
		prefab := builder.prefabs[instance.Prefab]
		if prefab == nil {
			return scopeResult{}, fmt.Errorf("instance %q prefab %q: %w", instance.ID, instance.Prefab, ErrExpansion)
		}
		instancePrefix := joinIdentity(prefix, instance.ID)
		instanceTransform := compose(transform, transformFrom(instance.Transform))
		instanceMaterial := materialOverride(instance.Materials, material)
		instanceTags := tagOverride
		if len(instance.Tags) > 0 {
			instanceTags = instance.Tags
		}
		expanded, err := builder.expandScope(
			instancePrefix, prefab.Rooms, prefab.Instances, prefab.Connections, prefab.Ports, prefab.Solids, prefab.Contents,
			instanceTransform, instanceMaterial, instanceTags,
		)
		if err != nil {
			return scopeResult{}, err
		}
		if len(result.solids)+len(expanded.solids) > sdkworld.MaxStaticSolids ||
			len(result.contents)+len(expanded.contents) > sdkworld.MaxContents {
			return scopeResult{}, ErrExpansion
		}
		result.solids = append(result.solids, expanded.solids...)
		result.contents = append(result.contents, expanded.contents...)
		result.rooms = append(result.rooms, expanded.rooms...)
		result.connections = append(result.connections, expanded.connections...)
		nested[instance.ID] = expanded.ports
	}

	for _, connection := range connections {
		left, err := resolveScopeEndpoint(connection.A, direct, nested)
		if err != nil {
			return scopeResult{}, fmt.Errorf("connection %q endpoint a: %w", connection.ID, err)
		}
		right, err := resolveScopeEndpoint(connection.B, direct, nested)
		if err != nil {
			return scopeResult{}, fmt.Errorf("connection %q endpoint b: %w", connection.ID, err)
		}
		result.connections = append(result.connections, Connection{
			ID: joinIdentity(prefix, connection.ID), A: left, B: right,
			Direction: connection.Direction, NonEuclidean: connection.NonEuclidean,
		})
	}

	for _, port := range ports {
		resolved, err := resolveScopeEndpoint(port.Endpoint, direct, nested)
		if err != nil {
			return scopeResult{}, fmt.Errorf("port %q: %w", port.ID, err)
		}
		result.ports[port.ID] = resolved
	}

	if len(result.rooms) > sdkworld.MaxSectors || len(result.connections) > worldsource.MaxConnections {
		return scopeResult{}, ErrExpansion
	}

	return result, nil
}

func transformRoom(
	authored worldsource.Room,
	prefix string,
	transform affine,
	material func(string) string,
	tagOverride []string,
) Room {
	roomID := joinIdentity(prefix, authored.ID)
	room := Room{
		ID: roomID, SourceRoom: authored.ID, Instance: prefix,
		Boundary: make([]worldsource.Edge, len(authored.Boundary)),
		Floor:    transformPlane(authored.Floor, transform), Ceiling: transformPlane(authored.Ceiling, transform),
		FloorMaterial: material(authored.FloorMaterial), CeilingMaterial: material(authored.CeilingMaterial),
		Contents: make([]Content, len(authored.Contents)),
		FloorUV:  authored.FloorUV, CeilingUV: authored.CeilingUV, WallUV: authored.WallUV,
		WallBands:        transformBands(authored.WallBands, material),
		FloorSecondary:   transformSecondary(authored.FloorSecondary, material),
		CeilingSecondary: transformSecondary(authored.CeilingSecondary, material),
		WallSecondary:    transformSecondary(authored.WallSecondary, material),
	}
	for index, edge := range authored.Boundary {
		room.Boundary[index] = worldsource.Edge{
			ID: edge.ID, Start: transformPoint(edge.Start, transform), End: transformPoint(edge.End, transform),
			Material: material(edge.Material), UV: edge.UV,
			Bands:     transformBands(worldsource.MergeBandSettings(authored.WallBands, edge.Bands), material),
			Secondary: transformSecondary(mergeSecondary(authored.WallSecondary, edge.Secondary), material),
		}
	}
	for index, content := range authored.Contents {
		room.Contents[index] = Content{
			ID: joinIdentity(roomID, content.ID), SourceID: content.ID, Instance: prefix, Kind: content.Kind,
			Position: transformPoint3(content.Position, transform),
		}
		room.Contents[index].Actor = transformActor(content.Actor, transform, tagOverride)
	}

	return room
}

func transformActor(authored *worldsource.Actor, transform affine, tagOverride []string) *Actor {
	if authored == nil {
		return nil
	}

	scale := authored.Scale
	if scale.X == 0 {
		scale.X = 1
	}
	if scale.Y == 0 {
		scale.Y = 1
	}
	if scale.Z == 0 {
		scale.Z = 1
	}

	tags := authored.Tags
	if len(tagOverride) > 0 {
		tags = tagOverride
	}
	tags = append([]string(nil), tags...)
	slices.Sort(tags)
	tags = slices.Compact(tags)

	var sprite *worldsource.Sprite
	if authored.Sprite != nil {
		copyOfSprite := *authored.Sprite
		sprite = &copyOfSprite
	}

	return &Actor{
		Yaw:   transform.yaw + authored.YawDegrees*math.Pi/degreesPerHalfTurn,
		Pitch: authored.PitchDegrees * math.Pi / degreesPerHalfTurn,
		Roll:  authored.RollDegrees * math.Pi / degreesPerHalfTurn,
		Scale: worldsource.Vec3{
			X: scale.X * transform.scale, Y: scale.Y * transform.scale, Z: scale.Z * transform.scale,
		},
		Sprite: sprite, Tags: tags,
	}
}

func transformFrom(value worldsource.Transform) affine {
	scale := value.Scale
	if scale == 0 {
		scale = 1
	}

	return affine{
		x: value.Translation.X, y: value.Translation.Y, z: value.Translation.Z,
		yaw: value.YawDegrees * math.Pi / degreesPerHalfTurn, scale: scale,
	}
}

func compose(parent, child affine) affine {
	sine, cosine := math.Sincos(parent.yaw)

	return affine{
		x:     parent.x + parent.scale*(child.x*cosine-child.y*sine),
		y:     parent.y + parent.scale*(child.x*sine+child.y*cosine),
		z:     parent.z + parent.scale*child.z,
		yaw:   parent.yaw + child.yaw,
		scale: parent.scale * child.scale,
	}
}

func transformPoint(value worldsource.Vec2, transform affine) worldsource.Vec2 {
	sine, cosine := math.Sincos(transform.yaw)

	return worldsource.Vec2{
		X: snap(transform.x + transform.scale*(value.X*cosine-value.Y*sine)),
		Y: snap(transform.y + transform.scale*(value.X*sine+value.Y*cosine)),
	}
}

func transformPoint3(value worldsource.Vec3, transform affine) worldsource.Vec3 {
	point := transformPoint(worldsource.Vec2{X: value.X, Y: value.Y}, transform)

	return worldsource.Vec3{X: point.X, Y: point.Y, Z: snap(transform.z + transform.scale*value.Z)}
}

func transformPlane(value worldsource.Plane, transform affine) worldsource.Plane {
	sine, cosine := math.Sincos(transform.yaw)
	a := value.A*cosine - value.B*sine
	b := value.A*sine + value.B*cosine

	return worldsource.Plane{
		A: snap(a), B: snap(b),
		C: snap(transform.z + transform.scale*value.C - a*transform.x - b*transform.y),
	}
}

func materialOverride(overrides []worldsource.MaterialOverride, parent func(string) string) func(string) string {
	values := make(map[string]string, len(overrides))
	for _, override := range overrides {
		values[override.From] = override.To
	}

	return func(value string) string {
		if replacement, ok := values[value]; ok {
			value = replacement
		}

		return parent(value)
	}
}

func joinIdentity(prefix, value string) string {
	if prefix == "" {
		return value
	}

	return prefix + "/" + value
}

func snap(value float64) float64 {
	const precision = 1_000_000_000.0

	value = math.Round(value*precision) / precision
	if value == 0 {
		return 0
	}

	return value
}

func resolveScopeEndpoint(endpoint worldsource.Endpoint, direct, nested map[string]map[string]Endpoint) (Endpoint, error) {
	if endpoint.Room != "" {
		resolved, ok := direct[endpoint.Room][endpoint.Edge]
		if !ok {
			return Endpoint{}, ErrExpansion
		}

		return resolved, nil
	}
	resolved, ok := nested[endpoint.Instance][endpoint.Port]
	if !ok {
		return Endpoint{}, ErrExpansion
	}

	return resolved, nil
}

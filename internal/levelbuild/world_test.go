//nolint:wsl_v5 // The build fixtures keep their setup and deterministic-output assertions together.
package levelbuild_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sdkqoi "github.com/karty-game/karty-sdk/codec/qoi"
	"github.com/karty-game/karty-sdk/format/asset"
	"github.com/karty-game/karty-sdk/format/cartridge"
	"github.com/karty-game/karty-sdk/format/level"
	sdkworld "github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty/internal/levelbuild"
	"github.com/karty-game/karty/internal/release"
	"github.com/karty-game/karty/internal/sdk"
	"github.com/karty-game/karty/internal/testfixture"
)

func TestWorldCameraSampleProcessedTextures(t *testing.T) {
	t.Parallel()

	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = geometrySDK(selected)
	root := testfixture.WorldCameraGeometry(t, filepath.Join("..", "..", "samples", "world-camera"))
	first, err := levelbuild.BuildAllWithAssets(context.Background(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}
	second, err := levelbuild.BuildAllWithAssets(context.Background(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Bytes, second[0].Bytes) ||
		!slices.Contains(first[0].Features, cartridge.FeatureTextureQOIv1) || !slices.Contains(first[0].Features, sdkworld.Feature) {
		t.Fatal("processed sample must retain world/QOI features and deterministic packaging")
	}
	envelope, err := level.Decode(unwrapLevelModule(t, first[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	materials := make(map[string]bool)
	for _, materialID := range []uint32{1, 4, 8} { // ceiling, floor, wall: same IDs as the original PNG fixture.
		encoded, _, found := envelope.Read(level.TextureEntryName(materialID), 0, level.MaxEntrySize)
		if !found || materials[string(encoded)] {
			t.Fatalf("processed material %d missing or duplicated", materialID)
		}
		materials[string(encoded)] = true
		metadata, texture, err := sdkqoi.Decode(encoded)
		if err != nil || metadata.Width == 0 || metadata.Height == 0 {
			t.Fatalf("processed material %d QOI metadata=%+v: %v", materialID, metadata, err)
		}
		pixels := texture.Pix
		colors := make(map[[3]byte]bool)
		for offset := 0; offset < len(pixels); offset += 4 {
			if pixels[offset+3] != 255 {
				t.Fatalf("processed material %d is not opaque", materialID)
			}
			colors[[3]byte{pixels[offset], pixels[offset+1], pixels[offset+2]}] = true
		}
		if len(colors) < 3 {
			t.Fatalf("processed material %d lost its authored pattern", materialID)
		}
	}
	for _, texture := range second[0].Textures {
		if !texture.CacheHit || texture.Encoding != "qoi" {
			t.Fatalf("second processed build did not reuse QOI cache: %+v", texture)
		}
	}
}

func TestBuildAllCompilesWorldYAMLIntoCanonicalLevelEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	directory := filepath.Join(root, "levels", "camera")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}

	var texture bytes.Buffer
	if err := png.Encode(&texture, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"floor", "ceiling", "wall"} {
		imageBytes := append([]byte(nil), texture.Bytes()...)
		if name == "wall" {
			texture.Reset()
			colored := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			colored.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
			if err := png.Encode(&texture, colored); err != nil {
				t.Fatal(err)
			}
			imageBytes = texture.Bytes()
		}
		if err := os.WriteFile(filepath.Join(directory, name+".png"), imageBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `[level]
name = "levels.camera"
kind = "world"
[world]
source = "world.yaml"
[[textures]]
name = "floor"
source = "floor.png"
[[textures]]
name = "ceiling"
source = "ceiling.png"
[[textures]]
name = "wall"
source = "wall.png"
`
	if err := os.WriteFile(filepath.Join(directory, "level.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	yaml := `version: 1
rooms:
  - id: hall
    boundary:
      - {id: south, start: {x: 0, y: 0}, end: {x: 4, y: 0}, material: wall}
      - {id: east, start: {x: 4, y: 0}, end: {x: 4, y: 4}, material: wall}
      - {id: north, start: {x: 4, y: 4}, end: {x: 0, y: 4}, material: wall}
      - {id: west, start: {x: 0, y: 4}, end: {x: 0, y: 0}, material: wall}
    floor: {a: 0.05, b: 0, c: 0}
    ceiling: {a: 0, b: -0.05, c: 4}
    floor_material: floor
    ceiling_material: ceiling
    contents:
      - {id: spawn, kind: spawn, position: {x: 2, y: 2, z: 1}}
`
	if err := os.WriteFile(filepath.Join(directory, "world.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := levelbuild.BuildAllWithAssets(t.Context(), root, 2, "", textureSDK(asset.ProcessorQOIv1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := levelbuild.BuildAllWithAssets(t.Context(), root, 2, "", textureSDK(asset.ProcessorQOIv1))
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || !slices.Equal(first[0].Features, []string{cartridge.FeatureTextureQOIv1, sdkworld.Feature}) ||
		first[0].ContentSHA256 != second[0].ContentSHA256 {
		t.Fatalf("world artifacts = %+v", first)
	}

	envelope, err := level.Decode(unwrapLevelModule(t, first[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, found := envelope.Read(sdkworld.EntryName, 0, sdkworld.MaxEncodedSize)
	if !found {
		t.Fatal("compiled world entry is missing")
	}
	document, err := sdkworld.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Sectors) != 1 || len(document.Contents) != 1 || document.Contents[0].SourceID != "spawn" {
		t.Fatalf("compiled world = %+v", document)
	}
}

func TestWorldCameraSampleLevelCompiles(t *testing.T) {
	t.Parallel()

	root := testfixture.WorldCameraGeometry(t, filepath.Join("..", "..", "samples", "world-camera"))
	selected, err := sdk.Resolve(release.SDKVersion())
	if err != nil {
		t.Fatal(err)
	}

	selected = geometrySDK(selected)
	first, err := levelbuild.BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}
	second, err := levelbuild.BuildAllWithAssets(t.Context(), root, 4, "", selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Name != "levels.camera-showcase" ||
		!slices.Equal(first[0].Features, []string{
			cartridge.FeatureTextureQOIv1, sdkworld.FeatureMaterialMapping, sdkworld.Feature, sdkworld.FeatureStaticSolids,
		}) ||
		first[0].ContentSHA256 != second[0].ContentSHA256 {
		t.Fatalf("sample artifacts = %+v", first)
	}

	envelope, err := level.Decode(unwrapLevelModule(t, first[0].Bytes))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, found := envelope.Read(sdkworld.EntryName, 0, sdkworld.MaxEncodedSize)
	if !found {
		t.Fatal("sample compiled world entry is missing")
	}
	document, err := sdkworld.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	// Column detail now belongs to independent solids; the actual migrated fixture
	// has a pinned room/portal graph, independent of shaft facet counts.
	if document.Version != sdkworld.Version || document.Lighting != nil || len(document.Sectors) != 69 || len(document.Contents) != 12 {
		t.Fatalf("sample world v%d = %d sectors, %d contents", document.Version, len(document.Sectors), len(document.Contents))
	}

	checkWorldCameraMaterials(t, document, envelope)
	checkWorldCameraPortals(t, document)
	checkWorldCameraBasin(t, document)
	checkWorldCameraActors(t, document)
}

func checkWorldCameraMaterials(t *testing.T, document sdkworld.Document, envelope level.Envelope) {
	t.Helper()

	// Material IDs follow the level's sorted texture names: ceiling, character,
	// decal, floor, ghost, sign, tree, wall. Decomposition-only edges have no
	// authored material, but every source boundary must retain the wall texture.
	for _, sector := range document.Sectors {
		if sector.Instance == "roman-court" {
			checkWorldCameraCourtMaterials(t, sector)

			continue
		}
		if sector.FloorMaterial != 4 || sector.CeilingMaterial != 1 {
			t.Fatalf("sector %q materials = floor %d, ceiling %d", sector.ID, sector.FloorMaterial, sector.CeilingMaterial)
		}
		for _, wall := range sector.Walls {
			if wall.SourceEdge != "" && wall.Material != 8 {
				t.Fatalf("sector %q edge %q material = %d, want wall texture 8", sector.ID, wall.SourceEdge, wall.Material)
			}
		}
	}
	materialBytes := make(map[string]bool)
	for _, materialID := range []uint32{1, 4, 8, 9, 10, 11, 12, 13, 14, 15} {
		encoded, _, found := envelope.Read(level.TextureEntryName(materialID), 0, level.MaxEntrySize)
		if !found || materialBytes[string(encoded)] {
			t.Fatalf("material texture %d is missing or duplicates another surface", materialID)
		}
		materialBytes[string(encoded)] = true
		_, texture, err := sdkqoi.Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		checkOpaquePattern(t, materialID, texture)
	}
}

func checkOpaquePattern(t *testing.T, materialID uint32, texture image.Image) {
	t.Helper()

	colors := make(map[color.NRGBA]bool)
	for row := range texture.Bounds().Dy() {
		for column := range texture.Bounds().Dx() {
			pixel, ok := color.NRGBAModel.Convert(texture.At(column, row)).(color.NRGBA)
			if !ok {
				t.Fatal("material pixel could not be converted to NRGBA")
			}
			if pixel.A != 255 {
				t.Fatalf("material texture %d has transparent pixel (%d,%d)", materialID, column, row)
			}
			colors[pixel] = true
		}
	}
	if len(colors) < 3 {
		t.Fatalf("material texture %d lacks patterned artwork", materialID)
	}
}

func checkWorldCameraColumns(t *testing.T, document sdkworld.Document) {
	t.Helper()
	if document.StaticSolids == nil || document.StaticSolids.Version != 1 || len(document.StaticSolids.Items) != 8 {
		t.Fatal("court lacks eight independent solid shafts")
	}
	bays := map[string]bool{}
	portals := 0
	for _, sector := range document.Sectors {
		if sector.Instance == "roman-court" && strings.HasPrefix(sector.SourceRoom, "bay-") {
			bays[sector.SourceRoom] = true
		}
		for _, wall := range sector.Walls {
			if strings.HasPrefix(wall.SourceEdge, "column-") {
				t.Fatal("column facet still partitions room geometry")
			}
			if wall.Portal >= 0 {
				portals++
			}
		}
	}
	if len(bays) != 16 || portals != 236 {
		t.Fatalf("court partition changed: bays=%d directed portals=%d", len(bays), portals)
	}
	centers := []sdkworld.Vec2{
		{X: -12, Y: 0},
		{X: -8, Y: 0},
		{X: -4, Y: 0},
		{X: -12, Y: 4},
		{X: -4, Y: 4},
		{X: -12, Y: 8},
		{X: -8, Y: 8},
		{X: -4, Y: 8},
	}
	for index, solid := range document.StaticSolids.Items {
		if solid.ID != fmt.Sprintf("roman-court/column-%d/shaft", index) || !solid.Collision || solid.Bottom != (sdkworld.Plane{}) ||
			solid.Top != (sdkworld.Plane{C: 12}) ||
			solid.SideMaterial != 10 ||
			solid.TopMaterial != 10 ||
			solid.BottomMaterial != 10 ||
			len(solid.Footprint) != 10 {
			t.Fatalf("column%d lost volume/identity/materials: %+v", index, solid)
		}
		perimeter := 0.0
		for vertex, point := range solid.Footprint {
			if math.Abs(math.Hypot(point.X-centers[index].X, point.Y-centers[index].Y)-0.5) > 1e-6 {
				t.Fatalf("column%d radius changed", index)
			}
			next := solid.Footprint[(vertex+1)%len(solid.Footprint)]
			perimeter += math.Hypot(next.X-point.X, next.Y-point.Y)
		}
		if math.Abs(perimeter-10*math.Sin(math.Pi/10)) > 1e-5 {
			t.Fatalf("column%d not regular decagon", index)
		}
	}
}

func checkWorldCameraPortals(t *testing.T, document sdkworld.Document) {
	t.Helper()
	checkWorldCameraColumns(t, document)

	type portalRef struct{ sector, wall int }
	findPortal := func(instance, room, edge string) portalRef {
		for sectorIndex, sector := range document.Sectors {
			if sector.Instance != instance || sector.SourceRoom != room {
				continue
			}
			for wallIndex, wall := range sector.Walls {
				if wall.SourceEdge == edge {
					return portalRef{sector: sectorIndex, wall: wallIndex}
				}
			}
		}
		t.Fatalf("missing portal endpoint %s/%s/%s", instance, room, edge)

		return portalRef{-1, -1}
	}
	ascentExit := findPortal("north-ascent", "room", "exit")
	hallNorth := findPortal("", "hall", "north")
	cornerExit := findPortal("east-corner", "room", "exit")
	ascentEntrance := findPortal("north-ascent", "room", "entrance")
	assertTarget := func(from, to portalRef) {
		wall := document.Sectors[from.sector].Walls[from.wall]
		if int(wall.Portal) != to.sector || int(wall.PortalWall) != to.wall+1 {
			t.Fatalf("portal %+v targets sector=%d wall=%d, want %+v", from, wall.Portal, wall.PortalWall, to)
		}
	}
	assertTarget(ascentExit, hallNorth)
	assertTarget(hallNorth, ascentExit)
	assertTarget(cornerExit, ascentEntrance)
	assertTarget(ascentEntrance, cornerExit)
	courtEntrance := findPortal("roman-court", "bay-3-1", "east")
	hallCourt := findPortal("", "hall", "courtyard")
	assertTarget(courtEntrance, hallCourt)
	assertTarget(hallCourt, courtEntrance)
}

func checkWorldCameraActors(t *testing.T, document sdkworld.Document) {
	t.Helper()

	actors := make(map[string]sdkworld.Content)
	for _, content := range document.Contents {
		if content.Actor != nil {
			actors[content.ID] = content
		}
	}
	if len(actors) != 11 {
		t.Fatalf("sample authored actors = %d, want 11", len(actors))
	}
	for authoredID, facing := range map[string]sdkworld.SpriteFacing{
		"hall/player-marker":            sdkworld.SpriteCameraFacing,
		"hall/camera-facing-adventurer": sdkworld.SpriteUpright,
		"hall/upright-adventurer":       sdkworld.SpriteUpright,
		"hall/fixed-sign":               sdkworld.SpriteFixed,
		"hall/slope-decal":              sdkworld.SpriteFixed,
		"hall/blended-ghost":            sdkworld.SpriteUpright,
		"gallery-a/room/marker":         sdkworld.SpriteCross,
		"gallery-b/room/marker":         sdkworld.SpriteCross,
		"gallery-c/room/marker":         sdkworld.SpriteCross,
		"east-corner/room/corner-sign":  sdkworld.SpriteFixed,
		"north-ascent/room/beacon":      sdkworld.SpriteUpright,
	} {
		actor, ok := actors[authoredID]
		if !ok || actor.Actor.Sprite == nil || actor.Actor.Sprite.Facing != facing || actor.Actor.Sprite.AssetID == 0 {
			t.Fatalf("sample actor %q = %+v, exists %v", authoredID, actor, ok)
		}
	}
	if actors["hall/camera-facing-adventurer"].Actor.Sprite.AssetID != actors["hall/upright-adventurer"].Actor.Sprite.AssetID ||
		actors["hall/blended-ghost"].Actor.Sprite.AssetID != actors["north-ascent/room/beacon"].Actor.Sprite.AssetID ||
		actors["gallery-a/room/marker"].Actor.Sprite.AssetID != actors["gallery-c/room/marker"].Actor.Sprite.AssetID ||
		actors["hall/fixed-sign"].Actor.Sprite.AssetID != actors["east-corner/room/corner-sign"].Actor.Sprite.AssetID {
		t.Fatal("sample actor roles do not share their intended texture assets")
	}
	decal := actors["hall/slope-decal"].Actor
	wantYaw := math.Atan2(-.08, 0)
	wantPitch := math.Atan2(1, .08)
	if math.Abs(decal.Yaw-wantYaw) > 1e-8 || math.Abs(decal.Pitch-wantPitch) > 1e-8 ||
		decal.Sprite.Alpha != sdkworld.SpriteBlend {
		t.Fatalf("slope decal = %+v, want yaw=%g pitch=%g", decal, wantYaw, wantPitch)
	}
	// Despite their authored IDs, fixed-sign is in the starting-room corner,
	// while corner-sign is the sign at the far end of the long hall.
	if got := actors["hall/fixed-sign"].Actor.Yaw; math.Abs(got) > 1e-8 {
		t.Fatalf("starting-corner sign yaw = %g, want 0", got)
	}
	if got := actors["east-corner/room/corner-sign"].Actor.Yaw; math.Abs(got+math.Pi/2) > 1e-8 {
		t.Fatalf("far-hall sign yaw = %g, want %g", got, -math.Pi/2)
	}
	if got := actors["gallery-a/room/marker"].Actor.Tags; !slices.Equal(got, []string{"gallery", "gallery-a", "spinner"}) {
		t.Fatalf("gallery-a tag override = %v", got)
	}
	if got := actors["gallery-b/room/marker"].Actor.Tags; !slices.Equal(got, []string{"gallery", "gallery-b"}) {
		t.Fatalf("gallery-b tag override = %v", got)
	}
	if got := actors["gallery-c/room/marker"].Actor.Tags; !slices.Equal(got, []string{"gallery", "gallery-c"}) {
		t.Fatalf("gallery-c tag override = %v", got)
	}
}

func checkWorldCameraCourtMaterials(t *testing.T, sector sdkworld.Sector) {
	t.Helper()
	floor, ceiling := uint32(11), uint32(9)
	if sector.Ceiling.C == 12 {
		floor, ceiling = 12, 14
	}
	if strings.HasPrefix(sector.SourceRoom, "basin-water-") {
		floor = 15
	} else if strings.HasPrefix(sector.SourceRoom, "basin-") {
		floor = 10
	}
	if sector.FloorMaterial != floor || sector.CeilingMaterial != ceiling {
		t.Fatalf("court sector %q materials = %d/%d, want %d/%d", sector.ID, sector.FloorMaterial, sector.CeilingMaterial, floor, ceiling)
	}
	for _, wall := range sector.Walls {
		if wall.SourceEdge != "" && wall.Material != 10 && wall.Material != 13 {
			t.Fatalf("court sector %q lost marble/plaster", sector.ID)
		}
	}
}

// Verify visible relief on the compiled geometry, including concave bay
// decomposition, rather than depending only on the authored room count.
func checkWorldCameraBasin(t *testing.T, document sdkworld.Document) {
	t.Helper()
	for _, sector := range document.Sectors {
		if sector.Instance != "roman-court" || !strings.HasPrefix(sector.SourceRoom, "basin-") {
			continue
		}
		for _, wall := range sector.Walls {
			if wall.Portal < 0 {
				t.Fatalf("basin edge %s/%s is disconnected", sector.SourceRoom, wall.SourceEdge)
			}
		}
	}
	for _, sample := range []struct{ x, height float64 }{
		{-5, 0}, {-5.35, .12}, {-5.6, .24}, {-5.82, .36}, {-6.12, .64}, {-7.7, .50},
	} {
		matches := 0
		for _, sector := range document.Sectors {
			if sector.Instance != "roman-court" {
				continue
			}
			if worldCameraSectorContains(sector, sample.x, 4.1) {
				matches++
				height := sector.Floor.A*sample.x + sector.Floor.B*4.1 + sector.Floor.C
				if math.Abs(height-sample.height) > 1e-9 {
					t.Fatalf("basin floor at (%g,4.1) = %g, want %g", sample.x, height, sample.height)
				}
			}
		}
		if matches != 1 {
			t.Fatalf("basin point (%g,4.1) belongs to %d sectors", sample.x, matches)
		}
	}
}

func worldCameraSectorContains(sector sdkworld.Sector, x, y float64) bool {
	for _, wall := range sector.Walls {
		cross := (wall.End.X-wall.Start.X)*(y-wall.Start.Y) -
			(wall.End.Y-wall.Start.Y)*(x-wall.Start.X)
		if cross < -1e-9 {
			return false
		}
	}

	return true
}

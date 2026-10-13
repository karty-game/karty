package geometry_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/karty-game/karty-sdk/format/world"
	"github.com/karty-game/karty-sdk/format/worldchunk"
	"github.com/karty-game/karty-sdk/format/worldlightmap"
	"github.com/karty-game/karty/internal/worldbuild/geometry"
	"github.com/karty-game/karty/internal/worldbuild/source"
	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

func TestEnvironmentSampleCompilesAndOwnsSun(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join("..", "..", "..", "samples", "environment", "levels"))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != "daylight" {
		t.Fatal("environment lab must contain only the daylight level", entries)
	}

	materials := environmentMaterials()

	preset := "daylight"

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", preset), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// An explicit authored sun and time selector are mutually exclusive.
	expanded.Environment.TimeOfDay = nil
	expanded.Environment.Sun = &world.EnvironmentSun{Azimuth: 135, Elevation: 40}

	compiled, err := geometry.Compile(expanded, materials)
	if err != nil {
		t.Fatal(err)
	}

	backdrop := world.EnvironmentBackdropMountains
	if compiled.Environment.Preset != world.EnvironmentPreset(preset) || compiled.Environment.Version != world.EnvironmentVersion ||
		compiled.Environment.Backdrop != backdrop {
		t.Fatal(compiled.Environment)
	}

	compiled.Environment.Sun.Azimuth = 90
	if expanded.Environment.Sun.Azimuth != 135 {
		t.Fatal("compiled environment aliases authoring")
	}

	skyWalls, skySectors, portals := 0, 0, 0

	for _, sector := range compiled.Sectors {
		if sector.Sky {
			skySectors++
		}

		for _, wall := range sector.Walls {
			if wall.Sky {
				skyWalls++
			}

			if wall.Portal >= 0 {
				portals++
			}
		}
	}

	if skyWalls != 33 || skySectors < 74 || portals < 318 || compiled.StaticSolids == nil {
		t.Fatalf("sample lost outdoor layout: sky walls=%d sectors=%d portals=%d", skyWalls, skySectors, portals)
	}
}

func TestEnvironmentCompileOwnsTimeAndCloudControls(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	expanded.Environment.TimeOfDay = new(float64(0))
	expanded.Environment.CloudAmount = new(float64(.45))

	compiled, err := geometry.Compile(
		expanded,
		environmentMaterials(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if compiled.Environment.TimeOfDay == nil || *compiled.Environment.TimeOfDay != 0 ||
		compiled.Environment.CloudAmount == nil || *compiled.Environment.CloudAmount != .45 {
		t.Fatal("compiled environment lost control presence", compiled.Environment)
	}

	*compiled.Environment.TimeOfDay = 12

	*compiled.Environment.CloudAmount = 1
	if *expanded.Environment.TimeOfDay != 0 || *expanded.Environment.CloudAmount != .45 {
		t.Fatal("compiled environment aliases authoring controls")
	}
}

func TestEnvironmentFurnitureGlazingAndRoofKeepWalkwaysClear(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	items := compiled.StaticSolids.Items
	byID := make(map[string]world.Solid, len(items))
	triangles := 0

	for _, solid := range items {
		byID[solid.ID] = solid
		triangles += len(solid.Footprint)*2 + (len(solid.Footprint)-2)*2
		assertEnvironmentReservedGround(t, solid)
	}

	if len(items) > world.MaxStaticSolids || triangles >= 8192 {
		t.Fatalf("scene exceeds static solid or sun-caster budget: %d solids, %d triangles", len(items), triangles)
	}

	for _, solidID := range []string{
		"observation-table-west/top", "observation-table-east/top", "courtyard-table/top",
		"courtyard-bench-west/seat", "courtyard-bench-east/seat",
		"facade-pier-west", "facade-pier-center", "facade-pier-east", "upper-window-0-spandrel", "upper-window-0-lintel",
		"west-window-glass", "east-window-glass", "upper-facade-pier-0",
		"roof-north-0", "roof-south-0", "roof-ridge-0", "chimney-cap",
	} {
		if _, found := byID[solidID]; !found {
			t.Fatalf("missing expanded furniture or building solid %q", solidID)
		}
	}

	for id, solid := range byID {
		if strings.Contains(id, "window") && strings.HasSuffix(id, "-glass") {
			if solid.SideMaterial != 9 || math.Abs(solid.Footprint[0].Y+3.27) > 1e-9 ||
				math.Abs(solid.Footprint[2].Y+3.23) > 1e-9 {
				t.Fatalf("facade glass %q lost its shared plane or material: %+v", id, solid)
			}
		}
	}

	roofNorth, roofSouth := byID["roof-north-0"], byID["roof-south-0"]
	if roofNorth.Top.B >= 0 || roofSouth.Top.B <= 0 ||
		math.Abs(roofNorth.Top.B*(-7.6)+roofNorth.Top.C-9.4) > 1e-7 ||
		math.Abs(roofSouth.Top.B*(-7.6)+roofSouth.Top.C-9.4) > 1e-7 || byID["upper-facade-pier-0"].Bottom.C < 4 {
		t.Fatalf(
			"upper story lost its pitched roof or obstructs the ground floor: north=%+v south=%+v bottom=%+v",
			roofNorth.Top,
			roofSouth.Top,
			byID["upper-facade-pier-0"].Bottom,
		)
	}

	for _, sector := range compiled.Sectors {
		if sector.Sky && !strings.Contains(sector.SourceRoom, "window") && !strings.HasPrefix(sector.SourceRoom, "covered-passage") &&
			sector.Ceiling.C <= byID["chimney-cap"].Top.C {
			t.Fatal("outdoor ceiling clips the building silhouette")
		}
	}

	assertEnvironmentWalkwaysAndGlass(t, compiled, items)
}

func environmentMaterials() map[string]uint32 {
	return map[string]uint32{
		"stone":           1,
		"tile":            2,
		"wall":            3,
		"ceiling":         4,
		"ground":          5,
		"wood":            6,
		"metal":           7,
		"slate":           8,
		"glass":           9,
		"wood-edge":       10,
		"furniture-steel": 11,
	}
}

func TestEnvironmentFieldCellsPreserveOpenGroundAndChunkOwnership(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	xedges := []float64{-24, -12, -9, -4, -.6, .6, 4, 9, 12, 24}
	wantBounds := make(map[string][4]float64, 36)

	for row := range 4 {
		for column := range len(xedges) - 1 {
			y := 24 + float64(row)*10
			wantBounds[fmt.Sprintf("test-ground-%d-%d", column, row)] = [4]float64{xedges[column], y, xedges[column+1], y + 10}
		}
	}

	seen := make(map[string]bool)

	for _, room := range expanded.Rooms {
		if !strings.HasPrefix(room.ID, "test-ground-") {
			continue
		}

		bounds, found := wantBounds[room.ID]
		if !found || seen[room.ID] || !room.Sky || room.Floor.C != 0 || room.Ceiling.C != 12 || room.FloorMaterial != "ground" ||
			room.FloorUV == nil || room.FloorUV.Scale == nil || room.FloorUV.Scale.X != 12 || room.FloorUV.Scale.Y != 12 {
			t.Fatalf("grass quadrant lost its identity, open floor or world UV mapping: %+v", room)
		}

		seen[room.ID] = true
		area := 0.0

		for index, edge := range room.Boundary {
			if edge.Start.X < bounds[0] || edge.Start.X > bounds[2] || edge.Start.Y < bounds[1] || edge.Start.Y > bounds[3] {
				t.Fatalf("grass quadrant %q extends outside its quarter of the reserved rectangle", room.ID)
			}

			next := room.Boundary[(index+1)%len(room.Boundary)].Start
			area += edge.Start.X*next.Y - next.X*edge.Start.Y
		}

		if math.Abs(area/2-(bounds[2]-bounds[0])*10) > 1e-7 {
			t.Fatalf("grass quadrant %q leaves a gap or changes the reserved field: area=%g", room.ID, area/2)
		}
	}

	if len(seen) != len(wantBounds) {
		t.Fatal("reserved field lost a quadrant", seen)
	}

	assertEnvironmentFieldConnections(t, expanded, xedges)

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}
	// This geometry fixture checks complete ownership independently of packing;
	// the authored lighting pipeline validates page capacity and retained density.
	plan, err := worldchunk.Plan(compiled, worldchunk.Options{NoLightmap: true})
	if err != nil {
		t.Fatal("daylight detail solids and field must have complete chunk receiver ownership", err)
	}

	solidOwners := make(map[int]int, len(compiled.StaticSolids.Items))

	for index, chunk := range plan.Chunks {
		for _, solid := range chunk.Solids {
			if _, duplicate := solidOwners[solid]; duplicate {
				t.Fatal("detail solid is assigned more than once", solid)
			}

			solidOwners[solid] = index
		}
	}

	if len(solidOwners) != len(compiled.StaticSolids.Items) {
		t.Fatal("detail solid has no streamed chunk owner", len(solidOwners), len(compiled.StaticSolids.Items))
	}
}

func environmentBodyOverlaps(footprint []world.Vec2, position world.Vec2, radius float64) bool {
	for index, start := range footprint {
		end := footprint[(index+1)%len(footprint)]

		dx, dy := end.X-start.X, end.Y-start.Y
		if dx*(position.Y-start.Y)-dy*(position.X-start.X) < -radius*math.Hypot(dx, dy) {
			return false
		}
	}

	return true
}

func TestEnvironmentBalancedLightmapDensityFitsChunkPages(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := os.ReadFile(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight", "level.toml"))
	if err != nil {
		t.Fatal(err)
	}

	var recipe struct {
		Lightmap struct {
			Enabled, Offline bool
			Density          float64
			PageSize         int `toml:"page_size"`
			Padding          int
			Environment      struct {
				Times           []float64
				DefaultTime     float64 `toml:"default_time"`
				WeatherVariants bool    `toml:"weather_variants"`
			}
		}
		Chunks struct{ Enabled bool }
	}
	if err := toml.Unmarshal(manifest, &recipe); err != nil {
		t.Fatal(err)
	}

	if !recipe.Lightmap.Enabled || !recipe.Lightmap.Offline || !recipe.Chunks.Enabled || recipe.Lightmap.Density < 24 ||
		recipe.Lightmap.PageSize != 1024 || recipe.Lightmap.Padding != 2 ||
		!slices.Equal(recipe.Lightmap.Environment.Times, []float64{0, 9, 12, 17}) ||
		recipe.Lightmap.Environment.DefaultTime != 12 || recipe.Lightmap.Environment.WeatherVariants ||
		compiled.Environment.TimeOfDay == nil || *compiled.Environment.TimeOfDay != 12 {
		t.Fatal("environment lost its bounded offline lighting conditions", recipe)
	}

	options := worldlightmap.Options{
		PageSize:             recipe.Lightmap.PageSize,
		TexelsPerUnit:        recipe.Lightmap.Density,
		Padding:              recipe.Lightmap.Padding,
		CapsWithinSourceRoom: true,
		CompactPacking:       true,
	}

	owners, err := worldchunk.BalancedSolidOwners(compiled, options)
	if err != nil {
		t.Fatal("environment receivers must fit the retained density with balanced detail owners", err)
	}

	plan, err := worldchunk.Plan(compiled, worldchunk.Options{Lightmap: options, SolidOwners: owners, CompactLightmaps: true})
	if err != nil {
		t.Fatal("balanced daylight chunks must preserve authored density", err)
	}

	if len(plan.Chunks) > 6 {
		t.Fatal("environment chart packing exceeds its all-view residency allowance", len(plan.Chunks))
	}
	// Mirror the host's documented conservative residency reservation for this
	// authored fixture, not encoded loading costs: four mesh scalar copies,
	// three decoded RNM copies, actor charts and retained environment metadata.
	globalSurfaces := 0
	for _, sector := range compiled.Sectors {
		globalSurfaces += len(sector.Walls) + 2
	}

	for _, solid := range compiled.StaticSolids.Items {
		globalSurfaces += len(solid.Footprint) + 2
	}

	var resident uint64

	for _, chunk := range plan.Chunks {
		charts := 0
		for _, sector := range chunk.Sectors {
			charts += len(compiled.Sectors[sector].Walls) + 2
		}

		for _, solid := range chunk.Solids {
			charts += len(compiled.StaticSolids.Items[solid].Footprint) + 2
		}

		actor := uint64(128 + len(compiled.Sectors)*64 + charts*216)
		resident += chunk.MeshBytes*4 + chunk.LightmapBytes*3 + actor*2 +
			worldlightmap.MaxEnvironmentCatalogSize + uint64(globalSurfaces)*72 + 1024
	}

	if resident > 192*1024*1024 {
		t.Fatalf("complete environment residency exceeds192MiB before loading: %.3fMiB", float64(resident)/(1024*1024))
	}

	for _, chunk := range plan.Chunks {
		if chunk.Layout.TexelsPerUnit != options.TexelsPerUnit || len(chunk.Layout.Pages) != 1 ||
			chunk.Layout.Pages[0].Width > options.PageSize || chunk.Layout.Pages[0].Height > options.PageSize {
			t.Fatal("environment chunk silently lowered density or increased page bounds", chunk.Layout)
		}
	}

	t.Logf(
		"balanced density %g fits %d sectors and %d static solids in %d bounded chunks: resident upper %.3fMiB (encoded loading pending)",
		options.TexelsPerUnit,
		len(compiled.Sectors),
		len(compiled.StaticSolids.Items),
		len(plan.Chunks),
		float64(resident)/(1024*1024),
	)
}

func TestEnvironmentBuildingRoofsSeparateExposedSurfacesAtEveryCorner(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	solids := make(map[string]world.Solid, len(compiled.StaticSolids.Items))
	for _, solid := range compiled.StaticSolids.Items {
		solids[solid.ID] = solid
	}

	elevation := func(plane world.Plane, point world.Vec2) float64 {
		return plane.A*point.X + plane.B*point.Y + plane.C
	}

	passageRoof, found := solids["passage-roof"]
	if !found {
		t.Fatal("passage roof is missing")
	}

	for _, sector := range compiled.Sectors {
		if !strings.HasPrefix(sector.SourceRoom, "covered-passage") {
			continue
		}

		for _, wall := range sector.Walls {
			if gap := elevation(passageRoof.Bottom, wall.Start) - elevation(sector.Ceiling, wall.Start); gap < .04 {
				t.Fatalf("passage ceiling overlaps roof at %+v: clearance=%g", wall.Start, gap)
			}
		}
	}

	// Every seam along both side walls and end walls is closed by a colliding
	// opaque cap whose slope overlaps the roof, not merely near its underside.
	for _, side := range []string{"west", "east", "south", "north"} {
		wallCap, found := solids["passage-wall-cap-"+side]
		if !found || !wallCap.Collision || wallCap.SideMaterial != 3 {
			t.Fatal("missing opaque corridor wall closure", side)
		}

		for _, point := range wallCap.Footprint {
			gap := elevation(wallCap.Top, point) - elevation(passageRoof.Bottom, point)
			if gap < .02 || gap > .04 || wallCap.Bottom.C > 3.8 {
				t.Fatalf("open roof seam %s at %+v: overlap=%g", side, point, gap)
			}
		}
	}

	assertEnvironmentGableClearance(t, solids, elevation)
}

func TestEnvironmentExteriorFacadeStopsRaysOutsideWindowApertures(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	for _, x := range []float64{-11.5, -10, -9.01, -.55, 0, .55, 9.01, 10, 11.5} {
		for _, z := range []float64{.05, .8, 1.7, 3.14, 3.5, 3.95} {
			if !environmentFacadeStopsRay(&compiled, x, z) {
				t.Fatalf("exterior ray reaches the sky through a non-window facade at x=%g z=%g", x, z)
			}
		}
	}

	for _, sampleX := range []float64{-8, -5, -2, 2, 5, 8} {
		for _, z := range []float64{.05, .8, 1.049, 3.151, 3.5, 3.95} {
			if !environmentFacadeStopsRay(&compiled, sampleX, z) {
				t.Fatalf("exterior ray reaches the sky below/above a recessed window at x=%g z=%g", sampleX, z)
			}
		}

		for _, z := range []float64{1.3, 2, 3} {
			if environmentFacadeStopsRay(&compiled, sampleX, z) {
				t.Fatalf("opaque facade panel blocks window glazing at x=%g z=%g", sampleX, z)
			}
		}
	}
}

// Trace a courtyard ray along -Y against the closed exterior facade faces.
// Filtering to structural panels prevents furniture or translucent glass from
// accidentally satisfying the building enclosure regression.
func environmentFacadeStopsRay(document *world.Document, sampleX, sampleZ float64) bool {
	if environmentSolidFacadeStopsRay(document, sampleX, sampleZ) {
		return true
	}

	surfaces, err := worldlightmap.Surfaces(document)
	if err != nil {
		return false
	}

	for _, surface := range surfaces {
		if surface.Binding.Kind != "sector-wall" {
			continue
		}

		for _, polygon := range surface.Polygons {
			minimumX, maximumX, minimumZ, maximumZ, front := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1), true
			for _, point := range polygon {
				front = front && math.Abs(point.Y+3.2) < 1e-9
				minimumX, maximumX, minimumZ, maximumZ = math.Min(
					minimumX,
					point.X,
				), math.Max(
					maximumX,
					point.X,
				), math.Min(
					minimumZ,
					point.Z,
				), math.Max(
					maximumZ,
					point.Z,
				)
			}

			if front && sampleX >= minimumX && sampleX <= maximumX && sampleZ >= minimumZ && sampleZ <= maximumZ {
				return true
			}
		}
	}

	return false
}

func TestEnvironmentOnlyFurniturePrefabsAndWindowGlassRemain(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight", "world.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	var authored struct {
		Prefabs   []struct{ ID string }
		Instances []struct{ Prefab string }
	}
	if err := yaml.Unmarshal(contents, &authored); err != nil {
		t.Fatal(err)
	}

	if len(authored.Prefabs) != 3 || len(authored.Instances) != 5 {
		t.Fatal("environment must use only three furniture prefabs and five static placements", authored)
	}

	for _, prefab := range authored.Prefabs {
		if !slices.Contains([]string{"table", "chair", "bench"}, prefab.ID) {
			t.Fatal("unexpected outdoor glass prefab", prefab.ID)
		}
	}

	expanded, err := source.Decode(contents)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	if compiled.SolidRenderers == nil || len(compiled.SolidRenderers.Prefabs) != 1 || compiled.SolidRenderers.Prefabs[0].ID != "chair" ||
		len(compiled.SolidRenderers.Prefabs[0].Solids) != 14 {
		t.Fatal("chair must remain one shared runtime prefab with eight seat bevels", compiled.SolidRenderers)
	}

	assertChairBevels(t, compiled.SolidRenderers.Prefabs[0])

	panes := 0

	for _, solid := range compiled.StaticSolids.Items {
		if strings.Contains(solid.ID, "chair") {
			t.Fatal("movable chair remains in static collision/bakes", solid.ID)
		}

		if strings.Contains(solid.ID, "windscreen") {
			t.Fatal("outdoor glass screen remains", solid.ID)
		}

		if solid.SideMaterial == 9 {
			panes++

			if !strings.Contains(solid.ID, "window") {
				t.Fatal("glass must belong to a building window", solid.ID)
			}
		}
	}

	if panes != 5 {
		t.Fatal("the two ground-floor and three upper window panes must remain", panes)
	}
}

func assertChairBevels(t *testing.T, chair world.SolidPrefab) {
	t.Helper()

	bevels := 0

	for _, solid := range chair.Solids {
		if !strings.HasPrefix(solid.ID, "seat-bevel-") {
			continue
		}

		bevels++

		if solid.TopMaterial != environmentMaterials()["wood-edge"] ||
			math.Abs(math.Hypot(solid.Top.A, solid.Top.B)-1) > 1e-7 {
			t.Fatal("chair seat edge lost its 45-degree dielectric bevel", solid)
		}

		for _, point := range solid.Footprint {
			height := solid.Top.A*point.X + solid.Top.B*point.Y + solid.Top.C
			if point.X < -.380001 || point.X > .380001 || point.Y < -.390001 || point.Y > .390001 ||
				height < -.022001 || height > -.009999 {
				t.Fatal("chair bevel escaped the existing visual/collider envelope", solid, point, height)
			}
		}
	}

	if bevels != 8 {
		t.Fatal("chair lost a seat edge", bevels)
	}
}

func TestEnvironmentStoneBoundaryClosesExposedEdgesWithoutSurfaceOverlaps(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	var walls []world.Solid

	for _, item := range compiled.StaticSolids.Items {
		if !strings.HasPrefix(item.ID, "boundary-") {
			continue
		}

		walls = append(walls, item)
		if !item.Collision || item.SideMaterial != 1 || item.TopMaterial != 1 || item.Bottom.C != -.08 ||
			len(item.Footprint) != 4 {
			t.Fatalf("boundary must be opaque colliding stone with its underside buried: %+v", item)
		}

		bounds := environmentSolidBounds(item)
		if math.Min(bounds[2]-bounds[0], bounds[3]-bounds[1]) < .399 ||
			math.Min(bounds[2]-bounds[0], bounds[3]-bounds[1]) > .401 ||
			math.Max(bounds[2]-bounds[0], bounds[3]-bounds[1]) > 8.001 {
			t.Fatalf("boundary section changes its thickness or exceeds bounded chart extent: %+v", item)
		}

		for _, corner := range item.Footprint {
			height := item.Top.A*corner.X + item.Top.B*corner.Y + item.Top.C
			if height < .839 || height > .921 {
				t.Fatalf("boundary is not a low gently uneven wall: %q height=%g", item.ID, height)
			}
		}
	}

	if len(walls) != 27 {
		t.Fatal("boundary lost an exposed-edge section", len(walls))
	}

	for first, wall := range walls {
		a := environmentSolidBounds(wall)
		for _, other := range walls[first+1:] {
			b := environmentSolidBounds(other)
			if math.Min(a[2], b[2])-math.Max(a[0], b[0]) > 1e-7 &&
				math.Min(a[3], b[3])-math.Max(a[1], b[1]) > 1e-7 {
				t.Fatalf("boundary joints overlap exposed top faces: %q and %q", wall.ID, other.ID)
			}
		}
	}

	assertEnvironmentBoundaryClearance(t, walls)
}

func TestEnvironmentFacadeFootingCoversTheExteriorFloorGap(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	var footings []world.Solid

	for _, solid := range compiled.StaticSolids.Items {
		if strings.HasPrefix(solid.ID, "facade-footing-") {
			footings = append(footings, solid)
		}
	}

	if len(footings) != 3 {
		t.Fatal("front foundation is incomplete", len(footings))
	}

	for column := range 49 {
		for row := range 10 {
			point := world.Vec2{X: -12 + float64(column)*.5, Y: -4.12 + float64(row)*(.94/9)}

			covered := false
			for _, solid := range footings {
				covered = covered || environmentBodyOverlaps(solid.Footprint, point, 1e-9)
				if solid.Bottom.C != -.12 || solid.Top.C != .08 || solid.Top.A != 0 || solid.Top.B != 0 || !solid.Collision {
					t.Fatal("foundation must form a real raised stone threshold above the courtyard floor", solid)
				}
			}

			if !covered {
				t.Fatalf("front foundation leaves a floor hole at %+v", point)
			}
		}
	}
}

func environmentSolidBounds(solid world.Solid) [4]float64 {
	bounds := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, point := range solid.Footprint {
		bounds[0] = math.Min(bounds[0], point.X)
		bounds[1] = math.Min(bounds[1], point.Y)
		bounds[2] = math.Max(bounds[2], point.X)
		bounds[3] = math.Max(bounds[3], point.Y)
	}

	return bounds
}

func TestEnvironmentPassageAndWindowMasksHaveOnePhysicalEnvelope(t *testing.T) {
	t.Parallel()

	expanded, err := source.Load(filepath.Join("..", "..", "..", "samples", "environment", "levels", "daylight"), "world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	document, err := geometry.Compile(expanded, environmentMaterials())
	if err != nil {
		t.Fatal(err)
	}

	surfaces, err := worldlightmap.Surfaces(&document)
	if err != nil {
		t.Fatal(err)
	}

	for _, surface := range surfaces {
		assertEnvironmentNavigationSurfaceMask(t, document, surface)
	}

	var roof world.Solid

	for _, item := range document.StaticSolids.Items {
		if item.ID == "passage-roof" {
			roof = item
		}
	}

	bounds := environmentSolidBounds(roof)
	if bounds[0] >= 12 || bounds[1] >= -12 || bounds[2] <= 16 || bounds[3] <= 4 || roof.Top.A >= 0 || roof.Top.B != 0 {
		t.Fatal("lean-to must cover every passage edge and pitch away from the house", roof)
	}

	for _, point := range roof.Footprint {
		if roof.Top.A*point.X+roof.Top.C >= 4.2 {
			t.Fatal("lean-to intersects the upper-story walls", roof)
		}
	}
	// Every vertical roof edge uses a distinct plane from the obsolete room
	// walls. This catches X12 side overlaps even when horizontal cap gaps pass.
	for index, first := range roof.Footprint {
		last := roof.Footprint[(index+1)%len(roof.Footprint)]

		for _, sector := range document.Sectors {
			for _, wall := range sector.Walls {
				if first.X == last.X && wall.Start.X == first.X && wall.End.X == first.X ||
					first.Y == last.Y && wall.Start.Y == first.Y && wall.End.Y == first.Y {
					t.Fatal("lean-to edge shares a vertical room-wall plane", first, last, wall)
				}
			}
		}
	}
}

func assertEnvironmentWalkwaysAndGlass(t *testing.T, compiled world.Document, items []world.Solid) {
	t.Helper()

	// Check the four teleports and the ground-floor route through the real side
	// passage with a 35cm player radius and standing body, rather than just eyes.
	positions := make([]world.Vec2, 0, 102)

	positions = append(positions, world.Vec2{X: -8.3, Y: -10.5}, world.Vec2{X: 1, Y: 0}, world.Vec2{X: 1, Y: 13}, world.Vec2{X: 0, Y: 32})
	// Join the oblique opening view to the established central passage route,
	// passing behind the west dining table instead of through its footprint.
	for step := range 18 {
		positions = append(positions, world.Vec2{X: -8.3 + 8.3*float64(step)/17, Y: -10.5})
	}

	for step := range 6 {
		positions = append(positions, world.Vec2{X: 0, Y: -10.5 + float64(step)/2})
	}

	for step := range 29 {
		positions = append(positions, world.Vec2{X: float64(step) / 2, Y: -8})
	}

	for step := range 19 {
		positions = append(positions, world.Vec2{X: 14, Y: -8 + float64(step)/2})
	}

	for step := range 27 {
		positions = append(positions, world.Vec2{X: 14 - float64(step)/2, Y: 1})
	}

	for _, position := range positions {
		for _, solid := range items {
			if solid.Collision && solid.Bottom.C < 1.8 && solid.Top.C > .02 &&
				environmentBodyOverlaps(solid.Footprint, position, .35) {
				t.Fatalf("solid %q blocks player at (%g,%g)", solid.ID, position.X, position.Y)
			}
		}
	}

	if compiled.Animations == nil || len(compiled.Animations.Presets) != 1 {
		t.Fatal("glass lost its static translucent binding")
	}

	glass := compiled.Animations.Presets[0]
	if glass.Kind != "liquid" || glass.Opacity == nil || *glass.Opacity != .16 || glass.Flow != (world.Vec2{}) ||
		glass.Amplitude != 0 || glass.SurfaceAmplitude != 0 || glass.Speed != 0 || glass.Frequency != 1 {
		t.Fatal("glass is opaque or animated", glass)
	}
}

func assertEnvironmentReservedGround(t *testing.T, solid world.Solid) {
	t.Helper()

	// Leave the reserved grass area empty for later vegetation experiments.
	for _, point := range solid.Footprint {
		if point.Y >= 24 && !(strings.HasPrefix(solid.ID, "boundary-") &&
			(point.Y <= 24.401 || point.Y >= 63.599 || point.X <= -23.599 || point.X >= 23.599)) {
			t.Fatalf("solid %q intrudes into the north test ground", solid.ID)
		}
	}
}

func assertEnvironmentFieldConnections(t *testing.T, expanded source.Expanded, xedges []float64) {
	t.Helper()

	// Ordinary portals connect the south cells to the courtyard and join every
	// interior grid border; the split never introduces visible barriers.
	wantLinks := make(map[[2]string]bool)
	for column := range 7 {
		wantLinks[[2]string{fmt.Sprintf("courtyard-%d-3", column), fmt.Sprintf("test-ground-%d-0", column+1)}] = false
	}

	for row := range 4 {
		for column := range len(xedges) - 1 {
			roomID := fmt.Sprintf("test-ground-%d-%d", column, row)
			if column < len(xedges)-2 {
				wantLinks[[2]string{roomID, fmt.Sprintf("test-ground-%d-%d", column+1, row)}] = false
			}

			if row < 3 {
				wantLinks[[2]string{roomID, fmt.Sprintf("test-ground-%d-%d", column, row+1)}] = false
			}
		}
	}

	for _, connection := range expanded.Connections {
		pair := [2]string{connection.A.Room, connection.B.Room}
		if _, found := wantLinks[pair]; found {
			if connection.NonEuclidean || connection.Direction != "" && connection.Direction != "both" {
				t.Fatal("grass connection changed ordinary traversal", connection)
			}

			wantLinks[pair] = true
		}
	}

	for pair, found := range wantLinks {
		if !found {
			t.Fatal("grass field has a disconnected border", pair)
		}
	}
}

func assertEnvironmentGableClearance(t *testing.T, solids map[string]world.Solid, elevation func(world.Plane, world.Vec2) float64) {
	t.Helper()

	for _, direction := range []string{"north", "south"} {
		roof := solids["roof-"+direction+"-0"]
		for _, index := range []int{0, 1} {
			id := fmt.Sprintf("gable-%s-%d", direction, index)

			gable, found := solids[id]
			if !found {
				t.Fatal("building gable is missing", id)
			}

			for _, point := range gable.Footprint {
				gableTop := elevation(gable.Top, point)
				if gap := elevation(roof.Bottom, point) - gableTop; gap < .01 || gap > .03 ||
					gableTop <= elevation(gable.Bottom, point) {
					t.Fatalf("gable %s touches the exposed roof slab or loses its closed volume at %+v: clearance=%g", gable.ID, point, gap)
				}
			}
		}
	}
}

func assertEnvironmentBoundaryClearance(t *testing.T, walls []world.Solid) {
	t.Helper()

	// Sample the complete exposed outline below the coping. Each outside ray
	// hits stone before the finite ground edge can become a hole into the sky.
	for _, run := range []struct {
		start, end world.Vec2
		inward     world.Vec2
	}{
		{world.Vec2{X: -12, Y: -3.2}, world.Vec2{X: -12, Y: 24}, world.Vec2{X: 1}},
		{world.Vec2{X: 12, Y: 4}, world.Vec2{X: 12, Y: 24}, world.Vec2{X: -1}},
		{world.Vec2{X: -24, Y: 24}, world.Vec2{X: -12, Y: 24}, world.Vec2{Y: 1}},
		{world.Vec2{X: 12, Y: 24}, world.Vec2{X: 24, Y: 24}, world.Vec2{Y: 1}},
		{world.Vec2{X: -24, Y: 24.4}, world.Vec2{X: -24, Y: 64}, world.Vec2{X: 1}},
		{world.Vec2{X: 24, Y: 24.4}, world.Vec2{X: 24, Y: 64}, world.Vec2{X: -1}},
		{world.Vec2{X: -24, Y: 64}, world.Vec2{X: 24, Y: 64}, world.Vec2{Y: -1}},
	} {
		steps := int(math.Ceil(math.Hypot(run.end.X-run.start.X, run.end.Y-run.start.Y) * 2))
		for sample := range steps + 1 {
			ratio := float64(sample) / float64(steps)
			point := world.Vec2{X: run.start.X + (run.end.X-run.start.X)*ratio + run.inward.X*.2,
				Y: run.start.Y + (run.end.Y-run.start.Y)*ratio + run.inward.Y*.2}

			blocked := false
			for _, wall := range walls {
				blocked = blocked || environmentBodyOverlaps(wall.Footprint, point, 0)
			}

			if !blocked {
				t.Fatalf("exposed boundary ray misses the low stone wall at %+v", point)
			}
		}
	}
	// The reserved field remains free of interior obstacles; its four edges
	// contain the wall, while the courtyard connection stays fully traversable.
	for _, position := range []world.Vec2{{X: 0, Y: 24}, {X: -11, Y: 24}, {X: 11, Y: 24}, {X: 0, Y: 32}, {X: -22, Y: 44}, {X: 22, Y: 44}} {
		for _, wall := range walls {
			if environmentBodyOverlaps(wall.Footprint, position, .35) {
				t.Fatalf("boundary blocks the field or its courtyard connection at %+v: %q", position, wall.ID)
			}
		}
	}
}

func environmentSolidFacadeStopsRay(document *world.Document, sampleX, sampleZ float64) bool {
	for _, solid := range document.StaticSolids.Items {
		if !strings.HasPrefix(solid.ID, "facade-pier-") && !strings.HasSuffix(solid.ID, "-spandrel") &&
			!strings.HasSuffix(solid.ID, "-lintel") {
			continue
		}

		for index, start := range solid.Footprint {
			end := solid.Footprint[(index+1)%len(solid.Footprint)]
			if start.X == end.X {
				continue
			}

			fraction := (sampleX - start.X) / (end.X - start.X)

			sampleY := start.Y + fraction*(end.Y-start.Y)
			if fraction < 0 || fraction > 1 || math.Abs(sampleY+3.2) > 1e-9 {
				continue
			}

			bottom := solid.Bottom.A*sampleX + solid.Bottom.B*sampleY + solid.Bottom.C

			top := solid.Top.A*sampleX + solid.Top.B*sampleY + solid.Top.C
			if sampleZ >= bottom && sampleZ <= top && solid.SideMaterial != 9 {
				return true
			}
		}
	}

	return false
}

func assertEnvironmentNavigationSurfaceMask(t *testing.T, document world.Document, surface worldlightmap.Surface) {
	t.Helper()

	if surface.Binding.Kind == "sector-ceiling" &&
		strings.HasPrefix(document.Sectors[surface.Binding.Index].SourceRoom, "covered-passage") &&
		len(surface.Polygons) != 0 {
		t.Fatal("passage contributes a duplicate opaque ceiling")
	}

	if surface.Binding.Kind != "sector-wall" {
		return
	}

	for _, polygon := range surface.Polygons {
		atPassage := true
		atObservationWindow := document.Sectors[surface.Binding.Index].Walls[surface.Binding.Edge].Portal >= 0

		for _, point := range polygon {
			atPassage = atPassage && math.Abs(point.X-12) < 1e-9 && point.Y >= -3.201 && point.Y <= 4.001
			atObservationWindow = atObservationWindow && math.Abs(point.Y+4) < 1e-9 && math.Abs(point.X) >= .6 && math.Abs(point.X) <= 9
		}

		for _, point := range polygon {
			if atPassage && point.Z > 3.8+1e-9 {
				t.Fatal("virtual portal ceiling became an opaque floating wall", surface.Binding, polygon)
			}
		}

		if atObservationWindow {
			assertEnvironmentWindowLintel(t, document, surface, polygon)
		}
	}
}

func assertEnvironmentWindowLintel(t *testing.T, document world.Document, surface worldlightmap.Surface, polygon []world.Vec3) {
	t.Helper()

	// Real interior lintels may remain as navigation surfaces only
	// while buried inside the closed physical window headers.
	var center world.Vec3
	for _, point := range polygon {
		center.X += point.X / float64(len(polygon))
		center.Y += point.Y / float64(len(polygon))
		center.Z += point.Z / float64(len(polygon))
	}

	if center.Z > 3.15 {
		buried := false

		for _, solid := range document.StaticSolids.Items {
			if (solid.ID == "west-window-lintel" || solid.ID == "east-window-lintel") &&
				environmentBodyOverlaps(solid.Footprint, world.Vec2{X: center.X, Y: center.Y}, 0) &&
				center.Z > solid.Bottom.C && center.Z < solid.Top.C {
				buried = true
			}
		}

		if !buried {
			t.Fatal("window navigation lintel escapes the physical facade", surface.Binding, polygon)
		}
	}
}

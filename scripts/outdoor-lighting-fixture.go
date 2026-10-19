//go:build ignore

// outdoor-lighting-fixture owns the retained outdoor lighting regression layout.
// Run from the repository root with mise run generate-outdoor-lighting-fixture.
package main

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
	"sort"
	"strings"

	"github.com/karty-game/karty/internal/sampleformat"
)

type point struct{ x, y float64 }
type room struct {
	id, floorMaterial string
	floor, ceiling    float64
	sky               bool
	points            []point
}

var rooms = authoredRooms()

type link struct {
	a, b       string
	start, end point
}

var links = authoredLinks()

func rectangle(id, material string, x0, y0, x1, y1, floor, ceiling float64, sky bool) room {
	return room{id, material, floor, ceiling, sky, []point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}}
}

func authoredRooms() []room {
	xedges := []float64{-12, -9, -4, -.6, .6, 4, 9, 12}
	var result []room
	// Align small floor/ceiling cells with every window and doorway edge. Full
	// ordinary portals preserve the original open room without duplicate charts
	// from collinear polygon decomposition.
	for row, span := range [][2]float64{{-12, -10}, {-10, -6}, {-6, -4}} {
		for column := range len(xedges) - 1 {
			id := fmt.Sprintf("observation-%d-%d", column, row)
			if column == 3 && row == 1 {
				id = "observation"
			}
			result = append(result, rectangle(id, "tile", xedges[column], span[0], xedges[column+1], span[1], 0, 4, false))
		}
	}
	result = append(result,
		rectangle("west-window", "stone", -9, -4, -4, -3.2, 1.05, 3.15, true),
		rectangle("west-window-inner", "stone", -4, -4, -.6, -3.2, 1.05, 3.15, true),
		rectangle("east-window-inner", "stone", .6, -4, 4, -3.2, 1.05, 3.15, true),
		rectangle("east-window", "stone", 4, -4, 9, -3.2, 1.05, 3.15, true),
		rectangle("covered-passage-south", "tile", 12, -12, 16, -10, 0, 3.8, true),
		rectangle("covered-passage", "tile", 12, -10, 16, -6, 0, 3.8, true),
		room{"covered-passage-middle", "tile", 0, 3.8, true, []point{{12, -6}, {16, -6}, {16, 0}, {12, 0}, {12, -3.2}, {12, -4}}},
		rectangle("covered-passage-north", "tile", 12, 0, 16, 4, 0, 3.8, true),
	)
	for row, span := range [][2]float64{{-3.2, 0}, {0, 4}, {4, 14}, {14, 24}} {
		for column := range len(xedges) - 1 {
			id := fmt.Sprintf("courtyard-%d-%d", column, row)
			if column == 3 && row == 1 {
				id = "courtyard"
			}
			result = append(result, rectangle(id, "stone", xedges[column], span[0], xedges[column+1], span[1], 0, 12, true))
		}
	}
	// Keep the same rectangular field cells and vegetation candidate budgets.
	// Affine floor planes form broad continuous rolls without triangulation.
	groundEdges := append([]float64{-24}, xedges...)
	groundEdges = append(groundEdges, 24)
	for row := range 4 {
		for column := range len(groundEdges) - 1 {
			material := "ground"
			if column == 4 {
				material = "path"
			}
			result = append(
				result,
				rectangle(
					fmt.Sprintf("test-ground-%d-%d", column, row),
					material,
					groundEdges[column],
					24+float64(row)*10,
					groundEdges[column+1],
					34+float64(row)*10,
					0,
					12,
					true,
				),
			)
		}
	}
	return result
}

// The trail and its thinner shoulders follow existing room boundaries. Trees
// add one seeded candidate in each selected 50-square-metre sector; all root
// placement remains procedural on the GPU.
func writeFieldLayers(data *bytes.Buffer, column, row int) {
	if column == 4 {
		data.WriteString("    floor_uv: {mode: triplanar, scale: {x: 3, y: 3}}\n")
		return
	}
	data.WriteString("    floor_uv: {mode: triplanar, scale: {x: 12, y: 12}}\n")
	grassDensity := []float64{3.5, 3, 2.5, 2}[row]
	bushDensity := []float64{0.045, 0.075, 0.12, 0.18}[row]
	if column == 0 || column == 8 {
		grassDensity *= .75
		bushDensity *= 1.5
	}
	if column == 3 || column == 5 {
		grassDensity *= .25
		bushDensity *= .1
	}
	fmt.Fprintf(
		data,
		"    vegetation_layers:\n      - {preset: field-grass, density: %g, seed: %d, surface: floor}\n      - {preset: field-bush, density: %g, seed: %d, surface: floor}\n",
		grassDensity,
		731+row*31+column,
		bushDensity,
		1931+row*31+column,
	)
	if (column == 2 || column == 6) && row > 0 {
		fmt.Fprintf(data, "      - {preset: field-tree, density: 0.04, seed: %d, surface: floor}\n", 6101+row*31+column)
	}
}

func authoredLinks() []link {
	var result []link
	for first, a := range rooms {
		for _, b := range rooms[first+1:] {
			for index, start := range a.points {
				end := a.points[(index+1)%len(a.points)]
				for other, opposite := range b.points {
					if start != b.points[(other+1)%len(b.points)] || end != opposite {
						continue
					}
					// Keep the original four-metre passage entrance; adjoining room
					// partitions must not open additional doorways in the exterior wall.
					if strings.HasPrefix(a.id, "observation") && strings.HasPrefix(b.id, "covered-passage") &&
						!(start.y == -10 && end.y == -6 || start.y == -6 && end.y == -10) {
						continue
					}
					result = append(result, link{a.id, b.id, start, end})
				}
			}
		}
	}
	return result
}

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	materials := []string{
		"stone",
		"tile",
		"wall",
		"ceiling",
		"ground",
		"path",
		"wood",
		"wood-edge",
		"metal",
		"furniture-steel",
		"slate",
		"glass",
	}
	images := map[string][]byte{}
	for _, name := range materials {
		var data bytes.Buffer
		var pixels image.Image
		if name == "ground" {
			// Retain the procedural comparison source; the shipped material now
			// uses low ground cover that continues between the upright plants.
			if err := png.Encode(&data, groundTexture()); err != nil {
				return err
			}
			if err := os.WriteFile(
				filepath.Join("testdata", "outdoor-lighting", "material-sources", "grass-soil-2048.png"),
				data.Bytes(),
				0o644,
			); err != nil {
				return err
			}
			data.Reset()
			master, err := vegetationMaster("forest-floor.png")
			if err != nil {
				return err
			}
			pixels = groundAtlasInput(master)
		} else {
			pixels = texture(name)
		}
		if err := png.Encode(&data, pixels); err != nil {
			return err
		}
		images[name] = data.Bytes()
	}
	for _, preset := range []string{"daylight"} {
		directory := filepath.Join("testdata", "outdoor-lighting", "levels", preset)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
		var manifest bytes.Buffer
		fmt.Fprintf(&manifest, "[level]\nname = \"levels.%s\"\nkind = \"world\"\n\n[world]\nsource = \"world.yaml\"\n", preset)
		// Static lighting endpoints are baked offline into streamed chunk pages.
		// Time controls select four pages; cloud cover dims them without rebaking.
		// Covered passages use three reflections and thirty-two hemisphere samples
		// to find daylight paths through their limited openings.
		manifest.WriteString(
			"\n[lightmap]\nenabled = true\noffline = true\ndensity = 16\npage_size = 1024\npadding = 2\nbake_samples = 32\nbake_bounces = 3\nbake_denoise = \"medium\"\n\n[lightmap.environment]\ntimes = [0, 9, 12, 17]\ndefault_time = 12\nweather_variants = false\n\n[chunks]\nenabled = true\n",
		)
		for _, name := range materials {
			fmt.Fprintf(&manifest, "\n[[textures]]\nname = %q\nsource = %q\nprofile = \"environment\"\n", name, name+".png")
			// Material classes, not room roles: the field is grass/soil, the wall
			// has brick courses, and the tile/ceiling are rough concrete.
			// Keep this recipe aligned with the sample material sources.
			preset := map[string]string{"stone": "stone", "tile": "concrete", "wall": "brick", "ceiling": "concrete", "ground": "foliage", "path": "stone", "wood": "wood", "wood-edge": "wood", "metal": "metal", "furniture-steel": "metal", "slate": "stone", "glass": "water"}[name]
			metallic := 0.0
			normal, height, ao := .35, .05, .5
			if name == "metal" {
				metallic = .35
			}
			if name == "wood" || name == "wood-edge" {
				normal, height, ao = .22, .025, .35
			}
			if name == "furniture-steel" {
				metallic, normal, height, ao = .8, .16, .01, .25
			}
			if name == "ground" {
				normal, height, ao = .22, .015, .35
			}
			if name == "path" {
				normal, height, ao = .18, .015, .35
			}
			neutral := ""
			if name == "glass" {
				// Glass uses the water recipe's smooth dielectric reflectance, with
				// no generated relief. Transparency is a static world binding.
				normal, height, ao = 0, 0, 0
				neutral = "normal_strength = 0\nao_depth_scale = 0\nheight_contrast = 0\n"
			}
			fmt.Fprintf(
				&manifest,
				"\n[textures.materialize]\npreset = %q\nmetallic_scale = %g\n%s\n[textures.material_strengths]\nnormal = %g\nheight = %g\nao = %g\nrim = 0.0\n",
				preset,
				metallic,
				neutral,
				normal,
				height,
				ao,
			)
			if err := os.WriteFile(filepath.Join(directory, name+".png"), images[name], 0o644); err != nil {
				return err
			}
		}
		for _, name := range []string{"grass-tuft", "small-bush"} {
			pixels, err := vegetationTexture(name)
			if err != nil {
				return err
			}
			for _, variant := range []struct {
				name   string
				pixels *image.NRGBA
			}{{name, pixels}, {name + "-normal", vegetationNormalTexture(pixels, name == "small-bush")}} {
				fmt.Fprintf(&manifest, "\n[[textures]]\nname = %q\nsource = %q\nprofile = \"sprite\"\n", variant.name, variant.name+".png")
				var data bytes.Buffer
				if err := png.Encode(&data, variant.pixels); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(directory, variant.name+".png"), data.Bytes(), 0o644); err != nil {
					return err
				}
			}
		}
		if err := os.WriteFile(filepath.Join(directory, "level.toml"), manifest.Bytes(), 0o644); err != nil {
			return err
		}
		formatted, err := sampleformat.YAML(context.Background(), worldYAML(preset))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, "world.yaml"), formatted, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func worldYAML(preset string) []byte {
	var data bytes.Buffer
	fmt.Fprintf(
		&data,
		"# $schema: ../../.karty/schemas/levels/%s.schema.json\n# Generated by scripts/outdoor-lighting-fixture.go; edit that tracked generator.\nversion: 18\nenvironment:\n  time_of_day: 12\n  lighting:\n    sun_color: {x: 1, y: 0.94, z: 0.82}\n    sun_intensity: 0.72\n    ambient: {x: 0.4, y: 0.52, z: 0.72}\n",
		preset,
	)
	data.WriteString(
		"  sky:\n    cloud_sparse: sky/clouds-sparse.png\n    cloud_dense: sky/clouds-dense.png\n    horizon: sky/mountains.png\n    settings:\n      cloud_uv_scale: [1, 0.95]\n      cloud_dense_scroll_scale: 0.55\n      cloud_dense_offset: [0.19, 0]\n      cloud_parallax: [0.000015, 0.000003]\n      horizon_parallax: 0.0015625\n      horizon_uv_scale: 1.5\n      horizon_haze: 0.35\n      horizon_emission: [0, 0, 0]\n",
	)
	data.WriteString(
		"vegetation:\n  version: 4\n  presets:\n    - {name: field-grass, texture: grass-tuft, normal_texture: grass-tuft-normal, width: {min: 0.85, max: 1.35}, height: {min: 0.65, max: 1}, wind_amplitude: 0.18, wind_frequency: 0.9, draw_distance: 64}\n    - {name: field-bush, geometry: shrub, texture: small-bush, normal_texture: small-bush-normal, width: {min: 1.6, max: 2.6}, height: {min: 0.8, max: 1.5}, wind_amplitude: 0.035, wind_frequency: 0.45, draw_distance: 64}\n    - {name: field-tree, geometry: tree, texture: small-bush, normal_texture: small-bush-normal, width: {min: 8, max: 11}, height: {min: 6.5, max: 8.5}, wind_amplitude: 0.12, wind_frequency: 0.35, draw_distance: 96}\nuv: {mode: triplanar, scale: {x: 2, y: 2}}\nlighting:\n  version: 1\n  actors: true\n  ambient: {x: 0.055, y: 0.065, z: 0.075}\n  lights:\n    - {id: observation-fill, position: {x: 0, y: -8, z: 3}, color: {x: 0.20, y: 0.17, z: 0.12}, radius: 7.5}\n    - {id: passage-fill, position: {x: 14, y: -3, z: 3}, color: {x: 0.16, y: 0.20, z: 0.22}, radius: 5.5}\nrooms:\n",
	)
	for _, r := range rooms {
		fmt.Fprintf(&data, "  - id: %s\n", r.id)
		if r.sky {
			data.WriteString("    sky: true\n")
		}
		if strings.HasPrefix(r.id, "test-ground-") {
			var column, row int
			fmt.Sscanf(r.id, "test-ground-%d-%d", &column, &row)
			writeFieldLayers(&data, column, row)
		}
		floor := fmt.Sprintf("{c: %g}", r.floor)
		if strings.HasPrefix(r.id, "test-ground-") {
			plane := terrainPlaneAt((r.points[0].x+r.points[2].x)/2, (r.points[0].y+r.points[2].y)/2)
			floor = plane.yaml()
		}
		fmt.Fprintf(
			&data,
			"    floor: %s\n    ceiling: {c: %g}\n    floor_material: %s\n    ceiling_material: ceiling\n    boundary:\n",
			floor,
			r.ceiling,
			r.floorMaterial,
		)
		for i, p := range r.points {
			q := r.points[(i+1)%len(r.points)]
			horizon := ""
			material := "wall"
			if (strings.HasPrefix(r.id, "test-ground-") ||
				(strings.HasPrefix(r.id, "courtyard") && (p.x == q.x && math.Abs(p.x) == 12 || p.y == -3.2 && q.y == -3.2))) && !linked(r.id, p, q) {
				horizon = ", sky: true"
			}
			if strings.Contains(r.id, "window") && p.x == q.x && !linked(r.id, p, q) ||
				strings.HasPrefix(r.id, "covered-passage") && p.x == 12 && q.x == 12 && p.y >= -4 && q.y >= -4 && !linked(r.id, p, q) {
				// Closed facade piers own these returns. A sky mask keeps the
				// navigation boundary without drawing a second coplanar wall.
				horizon = ", sky: true"
			}
			if strings.HasPrefix(r.id, "courtyard") && p.y == -3.2 && q.y == -3.2 && linked(r.id, p, q) {
				// The lower portal band is the exterior stone window spandrel.
				material = "stone"
			}
			fmt.Fprintf(
				&data,
				"      - {id: edge-%d, start: {x: %g, y: %g}, end: {x: %g, y: %g}, material: %s%s}\n",
				i,
				p.x,
				p.y,
				q.x,
				q.y,
				material,
				horizon,
			)
		}
	}
	data.WriteString("connections:\n")
	for i, l := range links {
		fmt.Fprintf(
			&data,
			"  - id: connection-%d\n    a: {room: %s, edge: edge-%d}\n    b: {room: %s, edge: edge-%d}\n",
			i,
			l.a,
			edge(l.a, l.start, l.end),
			l.b,
			edge(l.b, l.start, l.end),
		)
	}
	data.WriteString("solids:\n")
	// The crates make directional shadows legible. The north ground remains clear
	// so terrain and vegetation experiments can grow without rebuilding the entry.
	// Keep the directional shadow reference beside the west seating, leaving
	// the facade and the center of both courtyard viewpoints unobstructed.
	box(&data, "sun-test-crate", -9.4, 6, -6.9, 8.5, 0, 2.3, "wood")
	box(&data, "low-plinth", 3.5, 10, 7, 13, 0, .5, "stone")
	box(&data, "monolith", 4.5, 11, 6, 12, 0, 4, "metal")
	for _, p := range []point{{-9, 14}, {-9, 20}, {-3, 14}, {-3, 20}} {
		box(&data, fmt.Sprintf("canopy-post-%g-%g", p.x, p.y), p.x-.15, p.y-.15, p.x+.15, p.y+.15, 0, 3.3, "metal")
	}
	box(&data, "canopy-roof", -9.5, 13.5, -2.5, 20.5, 3.3, 3.6, "metal")
	box(&data, "west-bench", -11, 5, -9.5, 10, 0, .55, "stone")
	building(&data)
	boundaryWalls(&data)
	furniture(&data)
	data.WriteString(
		"animations:\n  version: 1\n  presets:\n    - {name: glass-pane, kind: liquid, flow: {x: 0, y: 0}, amplitude: 0, surface_amplitude: 0, speed: 0, frequency: 1, opacity: 0.16}\n  materials:\n    - {material: glass, preset: glass-pane}\n",
	)
	return []byte(strings.NewReplacer("{", "{ ", "}", " }").Replace(data.String()))
}

func boundaryWalls(data *bytes.Buffer) {
	// Follow only the exposed courtyard/field outline. Butt joints share edges,
	// never overlapping top faces; the field's connected interior stays open.
	// The first and last piece of each run have level tops at the corner joints.
	for _, span := range []struct {
		id             string
		x0, y0, x1, y1 float64
		alongX         bool
	}{
		{"courtyard-west", -12, -3.2, -11.6, 24.4, false},
		{"courtyard-east", 11.6, 4, 12, 24.4, false},
		{"field-shoulder-west", -24, 24, -12, 24.4, true},
		{"field-shoulder-east", 12, 24, 24, 24.4, true},
		{"field-west", -24, 24.4, -23.6, 63.6, false},
		{"field-east", 23.6, 24.4, 24, 63.6, false},
		{"field-north", -24, 63.6, 24, 64, true},
	} {
		start, end := span.y0, span.y1
		if span.alongX {
			start, end = span.x0, span.x1
		}
		count := int(math.Ceil((end - start) / 8))
		topProfile := make([]point, count+1)
		cuts := make([]float64, 0, count+6)
		for node := range topProfile {
			topProfile[node] = point{start + (end-start)*float64(node)/float64(count), .88}
			if node > 1 && node < count-1 {
				topProfile[node].y += .04 * math.Sin(float64(node)*2.3+float64(len(span.id)))
			}
			cuts = append(cuts, topProfile[node].x)
		}
		terrainProfile := terrainYProfile
		if span.alongX {
			terrainProfile = terrainXProfile
		}
		for _, knot := range terrainProfile {
			if knot.x > start && knot.x < end {
				cuts = append(cuts, knot.x)
			}
		}
		sort.Float64s(cuts)
		cuts = uniqueCuts(cuts)
		for index := range len(cuts) - 1 {
			a, b := cuts[index], cuts[index+1]
			x0, y0, x1, y1 := span.x0, a, span.x1, b
			if span.alongX {
				x0, y0, x1, y1 = a, span.y0, b, span.y1
			}
			floor := terrainPlaneAt((x0+x1)/2, (y0+y1)/2)
			bottom, top := floor, floor
			bottom.c -= .08
			slope, intercept := profileLineAt((a+b)/2, topProfile)
			if span.alongX {
				top.a += slope
			} else {
				top.b += slope
			}
			top.c += intercept
			solid(data, fmt.Sprintf("boundary-%s-%d", span.id, index), x0, y0, x1, y1, bottom.yaml(), top.yaml(), "stone")
		}
	}
}

// Separable profiles give every rectangle a single exact plane. The central
// courtyard opening stays at height zero; only the outer shoulders rise in X.
var terrainXProfile = []point{{-24, .18}, {-12, 0}, {12, 0}, {24, .18}}
var terrainYProfile = []point{{24, 0}, {34, 1.8}, {44, .5}, {54, 1.3}, {64, 0}}

type terrainPlane struct{ a, b, c float64 }

func (plane terrainPlane) yaml() string {
	return fmt.Sprintf("{a: %g, b: %g, c: %g}", plane.a, plane.b, plane.c)
}
func terrainPlaneAt(x, y float64) terrainPlane {
	if y < 24 {
		return terrainPlane{}
	}
	a, xIntercept := profileLineAt(x, terrainXProfile)
	b, yIntercept := profileLineAt(y, terrainYProfile)
	return terrainPlane{a, b, xIntercept + yIntercept}
}
func profileLineAt(value float64, profile []point) (float64, float64) {
	for index := range len(profile) - 1 {
		low, high := profile[index], profile[index+1]
		if value <= high.x {
			slope := (high.y - low.y) / (high.x - low.x)
			return slope, low.y - slope*low.x
		}
	}
	return 0, profile[len(profile)-1].y
}
func uniqueCuts(cuts []float64) []float64 {
	result := cuts[:0]
	for _, cut := range cuts {
		if len(result) == 0 || math.Abs(cut-result[len(result)-1]) > 1e-9 {
			result = append(result, cut)
		}
	}
	return result
}

func building(data *bytes.Buffer) {
	// Keep the real lower windows open in the portal graph. Thin glazing and
	// mullions are detail solids; the courtyard is still reached via the passage.
	// Room walls face the interior. These closed exterior panels enclose the
	// ground floor from the courtyard while leaving both glazed apertures open.
	// A shallow raised foundation spans the former doorway floor gap and meets
	// the courtyard paving. Its top is above the floor, never coplanar with it.
	sectionedBox(data, "facade-footing", -12, -4.12, 12, -3.18, -.12, .08, "stone")
	box(data, "facade-pier-west", -12, -4.1, -9, -3.2, 0, 4.2, "wall")
	box(data, "facade-pier-center", -.6, -4.1, .6, -3.2, 0, 4.2, "wall")
	box(data, "facade-pier-east", 9, -4.1, 12, -3.2, 0, 4.2, "wall")
	// The existing window-sector sill floors and courtyard lower portal bands
	// provide the stone spandrels. There is no second solid cap over those floors.
	window(data, "west-window", -8.93, -.67, 1.12, 3.08)
	window(data, "east-window", .67, 8.93, 1.12, 3.08)
	box(data, "west-window-lintel", -9, -4.1, -.6, -3.2, 3.15, 4.2, "stone")
	box(data, "east-window-lintel", .6, -4.1, 9, -3.2, 3.15, 4.2, "stone")
	for _, x := range []float64{-6.3, -3.3, 3.3, 6.3} {
		box(data, fmt.Sprintf("window-mullion-%g", x), x-.035, -3.35, x+.035, -3.15, 1.155, 3.045, "metal")
	}

	// A decorative upper storey is a solid volume, not another navigable room.
	// Cornices, repeated windows and a pitched slate roof break up the silhouette.
	exteriorTrim(data, "storey-belt", -12, -12, 12, -3.2, .25, 3.95, 4.2, "stone")
	// The existing lower room ceiling closes this inaccessible storey. A closed
	// exterior wall shell avoids spending atlas space on hidden solid-volume caps.
	roofSlope, roofRidgeY := (9.4-7.3)/5.05, -7.6
	roofThickness, gableClearance := .18, .02
	wallTop := 9.4 - roofSlope*4.4 - roofThickness - gableClearance
	for index, span := range [][2]float64{{-12, -9.1}, {-5.9, -1.6}, {1.6, 5.9}, {9.1, 12}} {
		box(data, fmt.Sprintf("upper-facade-pier-%d", index), span[0], -4.1, span[1], -3.2, 4.2, wallTop, "wall")
	}
	sectionedBox(data, "upper-storey-back", -12, -12, 12, -11.7, 4.2, wallTop, "wall")
	box(data, "upper-storey-west", -12, -11.7, -11.7, -4.1, 4.2, wallTop, "wall")
	box(data, "upper-storey-east", 11.7, -11.7, 12, -4.1, 4.2, wallTop, "wall")
	exteriorTrim(data, "upper-cornice", -12, -12, 12, -3.2, .25, 6.95, 7.25, "stone")
	for index, center := range []float64{-7.5, 0, 7.5} {
		id := fmt.Sprintf("upper-window-%d", index)
		// Real wall apertures integrate the recessed windows into the same
		// facade plane as the ground floor; the inaccessible loft has a backing.
		box(data, id+"-spandrel", center-1.6, -4.1, center+1.6, -3.2, 4.2, 4.6, "stone")
		box(data, id+"-lintel", center-1.6, -4.1, center+1.6, -3.2, 6.3, wallTop, "wall")
		box(data, id+"-backing", center-1.6, -3.45, center+1.6, -3.41, 4.6, 6.3, "metal")
		window(data, id, center-1.6, center+1.6, 4.6, 6.3)
		box(data, id+"-mullion", center-.035, -3.35, center+.035, -3.15, 4.635, 6.265, "metal")
	}
	// Thin pitched slabs rise to the centered ridge at y=-7.6, z=9.4. Separate end
	// gables close the roof without baking concealed filled-wedge cross sections.
	sectionedSlopedPlate(data, "roof-south", -12.65, -12.65, 12.65, roofRidgeY, roofThickness, roofSlope, 9.4-roofRidgeY*roofSlope, "slate")
	sectionedSlopedPlate(data, "roof-north", -12.65, roofRidgeY, 12.65, -2.55, roofThickness, -roofSlope, 9.4+roofRidgeY*roofSlope, "slate")
	for index, x := range []float64{-12, 11.75} {
		// Brick stops below the roof underside rather than sharing its exposed
		// top plane. This is real construction clearance, not a depth bias.
		slopedBox(
			data,
			fmt.Sprintf("gable-south-%d", index),
			x,
			-11.99,
			x+.25,
			roofRidgeY,
			wallTop,
			roofSlope,
			9.4-roofRidgeY*roofSlope-roofThickness-gableClearance,
			"wall",
		)
		slopedBox(
			data,
			fmt.Sprintf("gable-north-%d", index),
			x,
			roofRidgeY,
			x+.25,
			-3.21,
			wallTop,
			-roofSlope,
			9.4+roofRidgeY*roofSlope-roofThickness-gableClearance,
			"wall",
		)
	}
	sectionedBox(data, "roof-ridge", -12.7, roofRidgeY-.11, 12.7, roofRidgeY+.11, 9.3, 9.48, "slate")
	box(data, "chimney", -8.3, -9.6, -7.5, -8.8, 8.2, 10, "wall")
	box(data, "chimney-cap", -8.45, -9.75, -7.35, -8.65, 9.9, 10.15, "stone")
	// A single lean-to pitches away from the house across X. Its attachment is
	// embedded in the exterior belt, below the upper wall. Passage sky ceilings
	// retain navigation confinement but no longer contribute opaque roof faces.
	slope := (4.0 - 4.15) / (16.4 - 12)
	solid(data, "passage-roof", 11.95, -12.4, 16.4, 4.3,
		fmt.Sprintf("{a: %g, c: %g}", slope, 4.15-12*slope-.17), fmt.Sprintf("{a: %g, c: %g}", slope, 4.15-12*slope), "slate")
	// The sky navigation ceiling does not draw an upper wall. Close the gap
	// above each physical wall with a cap following the roof's underside.
	// Tops embed in the roof; bottoms embed in the existing 3.8m wall heads.
	for _, edge := range []struct {
		id             string
		x0, y0, x1, y1 float64
	}{
		{"west", 11.98, -12, 12.18, 4}, {"east", 15.82, -12, 16.08, 4},
		{"south", 12.18, -12, 15.82, -11.78}, {"north", 12.18, 3.78, 15.82, 4},
	} {
		solid(data, "passage-wall-cap-"+edge.id, edge.x0, edge.y0, edge.x1, edge.y1,
			"{c: 3.78}", fmt.Sprintf("{a: %g, c: %g}", slope, 4.15-12*slope-.17+.03), "wall")
	}
}

func window(data *bytes.Buffer, id string, x0, x1, z0, z1 float64) {
	box(data, id+"-glass", x0, -3.27, x1, -3.23, z0, z1, "glass")
	box(data, id+"-frame-left", x0-.07, -3.35, x0+.035, -3.15, z0-.07, z1+.07, "metal")
	box(data, id+"-frame-right", x1-.035, -3.35, x1+.07, -3.15, z0-.07, z1+.07, "metal")
	box(data, id+"-frame-bottom", x0+.035, -3.35, x1-.035, -3.15, z0-.07, z0+.035, "metal")
	box(data, id+"-frame-top", x0+.035, -3.35, x1-.035, -3.15, z1-.035, z1+.07, "metal")
}

func furniture(data *bytes.Buffer) {
	// Tables and benches expand into static solids; chairs are packaged movable props.
	data.WriteString("prefabs:\n")
	prefab(data, "table", func(parts *bytes.Buffer) {
		beveledBox(parts, "top", -1.4, -.85, 1.4, .85, .8, .94, .12, .018, "wood", "wood-edge")
		for index, p := range []point{{-1.15, -.6}, {1.15, -.6}, {1.15, .6}, {-1.15, .6}} {
			clippedBox(parts, fmt.Sprintf("leg-%d", index), p.x-.075, p.y-.075, p.x+.075, p.y+.075, 0, .8, .015, "furniture-steel")
		}
	})
	prefab(data, "chair", func(parts *bytes.Buffer) {
		// Center visual geometry on the rigid body so tipping keeps the collider aligned.
		beveledBox(parts, "seat", -.38, -.37, .38, .39, -.11, -.01, .055, .012, "wood", "wood-edge")
		clippedBox(parts, "back", -.38, -.39, .38, -.30, -.01, .54, .025, "wood")
		for index, p := range []point{{-.29, -.29}, {.29, -.29}, {.29, .29}, {-.29, .29}} {
			clippedBox(
				parts,
				fmt.Sprintf("leg-%d", index),
				p.x-.045,
				p.y+.01-.045,
				p.x+.045,
				p.y+.01+.045,
				-.54,
				-.11,
				.008,
				"furniture-steel",
			)
		}
	})
	prefab(data, "bench", func(parts *bytes.Buffer) {
		beveledBox(parts, "seat", -1.6, -.32, 1.6, .32, .45, .58, .09, .015, "wood", "wood-edge")
		box(parts, "back", -1.6, -.39, 1.6, -.27, .7, 1.13, "wood")
		for index, x := range []float64{-1.18, 1.18} {
			clippedBox(parts, fmt.Sprintf("support-%d", index), x-.07, -.38, x+.07, .35, 0, .78, .015, "furniture-steel")
		}
	})
	data.WriteString("instances:\n")
	for _, item := range []struct {
		id, prefab string
		x, y, yaw  float64
	}{
		{"observation-table-west", "table", -5.5, -8.3, 0},
		{"observation-table-east", "table", 6.5, -9.9, 0},
		{"courtyard-table", "table", -6, 2, 0},
		{"courtyard-bench-west", "bench", -8, 10.8, 90},
		{"courtyard-bench-east", "bench", 8.3, 4, 180},
	} {
		fmt.Fprintf(
			data,
			"  - {id: %s, prefab: %s, transform: {translation: {x: %g, y: %g, z: 0}, yaw_degrees: %g}}\n",
			item.id,
			item.prefab,
			item.x,
			item.y,
			item.yaw,
		)
	}
}

func prefab(data *bytes.Buffer, id string, parts func(*bytes.Buffer)) {
	var body bytes.Buffer
	parts(&body)
	if id == "chair" {
		body = *bytes.NewBufferString(strings.ReplaceAll(body.String(), "collision: true", "collision: false"))
	}
	fmt.Fprintf(data, "  - id: %s\n", id)
	if id == "chair" {
		data.WriteString("    renderable: true\n")
	}
	data.WriteString("    solids:\n")
	for _, line := range strings.Split(strings.TrimSuffix(body.String(), "\n"), "\n") {
		fmt.Fprintf(data, "    %s\n", line)
	}
}

func sectionedBox(data *bytes.Buffer, id string, x0, y0, x1, y1, z0, z1 float64, material string) {
	for index := range 3 {
		left := x0 + (x1-x0)*float64(index)/3
		right := x0 + (x1-x0)*float64(index+1)/3
		box(data, fmt.Sprintf("%s-%d", id, index), left, y0, right, y1, z0, z1, material)
	}
}

func perimeter(data *bytes.Buffer, id string, x0, y0, x1, y1, thickness, z0, z1 float64, material string) {
	sectionedBox(data, id+"-front", x0, y1-thickness, x1, y1, z0, z1, material)
	sectionedBox(data, id+"-back", x0, y0, x1, y0+thickness, z0, z1, material)
	box(data, id+"-west", x0, y0+thickness, x0+thickness, y1-thickness, z0, z1, material)
	box(data, id+"-east", x1-thickness, y0+thickness, x1, y1-thickness, z0, z1, material)
}

func exteriorTrim(data *bytes.Buffer, id string, x0, y0, x1, y1, projection, z0, z1 float64, material string) {
	// The stone bands embed slightly into the brick shell. Their inner caps are
	// closed by the upper wall; only the projecting trim faces remain exposed.
	// The east belt also embeds the lean-to attachment without an open roof gap.
	const inset = .15
	sectionedBox(data, id+"-front", x0-projection, y1-inset, x1+projection, y1+projection, z0, z1, material)
	sectionedBox(data, id+"-back", x0-projection, y0-projection, x1+projection, y0+inset, z0, z1, material)
	box(data, id+"-west", x0-projection, y0+inset, x0+inset, y1-inset, z0, z1, material)
	box(data, id+"-east", x1-inset, y0+inset, x1+projection, y1-inset, z0, z1, material)
}

func sectionedSlopedPlate(data *bytes.Buffer, id string, x0, y0, x1, y1, thickness, slope, intercept float64, material string) {
	for index := range 3 {
		left := x0 + (x1-x0)*float64(index)/3
		right := x0 + (x1-x0)*float64(index+1)/3
		solid(data, fmt.Sprintf("%s-%d", id, index), left, y0, right, y1,
			fmt.Sprintf("{b: %g, c: %g}", slope, intercept-thickness), fmt.Sprintf("{b: %g, c: %g}", slope, intercept), material)
	}
}

func slopedBox(data *bytes.Buffer, id string, x0, y0, x1, y1, bottom, slope, intercept float64, material string) {
	solid(data, id, x0, y0, x1, y1, fmt.Sprintf("{c: %g}", bottom), fmt.Sprintf("{b: %g, c: %g}", slope, intercept), material)
}

func edge(id string, a, b point) int {
	for _, r := range rooms {
		if r.id == id {
			for i, p := range r.points {
				q := r.points[(i+1)%len(r.points)]
				if (p == a && q == b) || (p == b && q == a) {
					return i
				}
			}
		}
	}
	panic("missing authored portal edge: " + id)
}

func linked(id string, start, end point) bool {
	for _, l := range links {
		if (l.a == id || l.b == id) && ((l.start == start && l.end == end) || (l.start == end && l.end == start)) {
			return true
		}
	}
	return false
}

func box(data *bytes.Buffer, id string, x0, y0, x1, y1, z0, z1 float64, material string) {
	solid(data, id, x0, y0, x1, y1, fmt.Sprintf("{c: %g}", z0), fmt.Sprintf("{c: %g}", z1), material)
}

// Clip only the corners of the existing footprint: no extra solids, overlapping
// caps or changes to the furniture's outer bounds or chair physics box.
func clippedBox(data *bytes.Buffer, id string, x0, y0, x1, y1, z0, z1, clip float64, material string) {
	fmt.Fprintf(data, "  - id: %s\n    footprint: [", id)
	for index, p := range []point{
		{x0 + clip, y0}, {x1 - clip, y0}, {x1, y0 + clip}, {x1, y1 - clip},
		{x1 - clip, y1}, {x0 + clip, y1}, {x0, y1 - clip}, {x0, y0 + clip},
	} {
		if index > 0 {
			data.WriteString(", ")
		}
		fmt.Fprintf(data, "{x: %g, y: %g}", p.x, p.y)
	}
	fmt.Fprintf(
		data,
		"]\n    bottom: {c: %g}\n    top: {c: %g}\n    side_material: %s\n    top_material: %s\n    bottom_material: %s\n    collision: true\n",
		z0,
		z1,
		material,
		material,
		material,
	)
}

// Partition the original footprint into a flat center and eight 45-degree edge
// bands. Volumes share only boundaries; the outer bounds and chair collider stay fixed.
func beveledBox(data *bytes.Buffer, id string, x0, y0, x1, y1, z0, z1, clip, bevel float64, material, edgeMaterial string) {
	outer := []point{
		{x0 + clip, y0}, {x1 - clip, y0}, {x1, y0 + clip}, {x1, y1 - clip},
		{x1 - clip, y1}, {x0 + clip, y1}, {x0, y1 - clip}, {x0, y0 + clip},
	}
	inner := make([]point, len(outer))
	normals := make([]point, len(outer))
	for index, p := range outer {
		next := outer[(index+1)%len(outer)]
		length := math.Hypot(next.x-p.x, next.y-p.y)
		normals[index] = point{-(next.y - p.y) / length, (next.x - p.x) / length}
	}
	for index, p := range outer {
		previous, current := normals[(index+len(outer)-1)%len(outer)], normals[index]
		determinant := previous.x*current.y - previous.y*current.x
		inner[index] = point{
			p.x + bevel*(current.y-previous.y)/determinant,
			p.y + bevel*(previous.x-current.x)/determinant,
		}
	}
	polygonSolid(data, id, inner, z0, fmt.Sprintf("{c: %g}", z1), material)
	for index, p := range outer {
		next := (index + 1) % len(outer)
		normal := normals[index]
		cap := fmt.Sprintf("{a: %g, b: %g, c: %g}", normal.x, normal.y, z1-bevel-normal.x*p.x-normal.y*p.y)
		polygonSolid(data, fmt.Sprintf("%s-bevel-%d", id, index), []point{p, outer[next], inner[next], inner[index]}, z0, cap, edgeMaterial)
	}
}

func polygonSolid(data *bytes.Buffer, id string, footprint []point, bottom float64, top, material string) {
	fmt.Fprintf(data, "  - id: %s\n    footprint: [", id)
	for index, p := range footprint {
		if index > 0 {
			data.WriteString(", ")
		}
		fmt.Fprintf(data, "{x: %g, y: %g}", p.x, p.y)
	}
	fmt.Fprintf(
		data,
		"]\n    bottom: {c: %g}\n    top: %s\n    side_material: %s\n    top_material: %s\n    bottom_material: %s\n    collision: true\n",
		bottom,
		top,
		material,
		material,
		material,
	)
}

func solid(data *bytes.Buffer, id string, x0, y0, x1, y1 float64, bottom, top, material string) {
	fmt.Fprintf(
		data,
		"  - id: %s\n    footprint: [{x: %g, y: %g}, {x: %g, y: %g}, {x: %g, y: %g}, {x: %g, y: %g}]\n    bottom: %s\n    top: %s\n    side_material: %s\n    top_material: %s\n    bottom_material: %s\n    collision: true\n",
		id,
		x0,
		y0,
		x1,
		y0,
		x1,
		y1,
		x0,
		y1,
		bottom,
		top,
		material,
		material,
		material,
	)
}

func texture(name string) image.Image {
	if name == "ground" {
		return groundTexture()
	}
	if name == "path" {
		return pathTexture()
	}
	const size = 128
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	// Warm masonry and oak sit above quieter cool paving. Charcoal metal and
	// slate define window frames and roof edges without a repeated panel grid.
	base := map[string][3]int{"stone": {148, 160, 164}, "tile": {112, 131, 137}, "wall": {185, 156, 133}, "ceiling": {157, 161, 156}, "wood": {158, 112, 68}, "wood-edge": {167, 125, 82}, "metal": {43, 56, 61}, "furniture-steel": {116, 127, 132}, "slate": {61, 72, 83}, "glass": {143, 174, 177}}[name]
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			noise := ((x*17 + y*31 + (x*y)%37) % 7) - 3
			shade := noise
			switch name {
			case "stone", "tile":
				if x%64 < 2 || y%64 < 2 {
					shade -= 11
				}
				if x%64 == 3 || y%64 == 3 {
					shade += 3
				}
			case "wall":
				if y%32 < 2 || (x+(y/32%2)*32)%64 < 2 {
					shade -= 10
				}
				shade += ((x/64+y/32*3)%5 - 2) * 2
			case "wood", "wood-edge":
				shade += int(3 * math.Sin(float64(x)/3+2*math.Sin(float64(y)/21)))
				if x%32 < 2 {
					shade -= 12
				}
			case "metal":
				shade = noise / 2
			case "furniture-steel":
				shade = noise/2 + int(2*math.Sin(float64(y)*1.3))
			case "slate":
				shade += (x/7+y/3)%5 - 2
				if y%32 < 2 || (x+(y/32%2)*16)%32 < 2 {
					shade -= 10
				}
			case "glass":
				// Mild tint and broad streaks are visible through low static opacity.
				// The smoothness/normal maps stay neutral rather than turning this
				// albedo variation into ripples or scratches in the reflection.
				shade = int(2 * math.Sin(float64(x+y)/18))
			}
			out.SetNRGBA(
				x,
				y,
				color.NRGBA{
					uint8(max(0, min(255, base[0]+shade))),
					uint8(max(0, min(255, base[1]+shade))),
					uint8(max(0, min(255, base[2]+shade))),
					255,
				},
			)
		}
	}
	return out
}

// The source preserves broad grass/soil patches after the material atlas samples
// its 256px tile. All noise periods divide the image, keeping both edges seamless.
func groundTexture() image.Image {
	const size = 2048
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			u, v := float64(x)/size, float64(y)/size
			broad := .62*groundNoise(u, v, 4, 31) + .27*groundNoise(u, v, 8, 67) + .11*groundNoise(u, v, 16, 109)
			soil := smoothGround(.39, .66, broad)
			dry := groundNoise(u, v, 16, 151)
			grain := (groundHash(x, y, 199) - .5) * 7
			flecks := (groundNoise(u, v, 128, 211) - .5) * 9
			grass := [3]float64{105 + dry*17, 115 + dry*10, 77 + dry*12}
			earth := [3]float64{124, 110, 85}
			pixel := color.NRGBA{A: 255}
			channels := []*uint8{&pixel.R, &pixel.G, &pixel.B}
			for index, channel := range channels {
				value := grass[index]*(1-soil) + earth[index]*soil + grain + flecks
				*channel = uint8(max(0, min(255, value)))
			}
			out.SetNRGBA(x, y, pixel)
		}
	}
	// Sparse fine fibers give nearby grass some direction without painted grid lines.
	for y := range size / 4 {
		for x := range size / 4 {
			seed := groundHash(x, y, 257)
			px, py := x*4+int(seed*4), y*4+int(groundHash(x, y, 263)*4)
			length := 3 + int(groundHash(x, y, 269)*6)
			for step := range length {
				column, row := (px+step/3)%size, (py+step)%size
				pixel := out.NRGBAAt(column, row)
				if pixel.G <= pixel.R+2 {
					continue
				}
				shade := 3 + int(seed*6)
				pixel.R = uint8(min(255, int(pixel.R)+shade))
				pixel.G = uint8(min(255, int(pixel.G)+shade))
				pixel.B = uint8(min(255, int(pixel.B)+shade/2))
				out.SetNRGBA(column, row, pixel)
			}
		}
	}
	return out
}

// Match the existing material atlas's nearest source sampling before import.
// The authored master stays in Git; only the compact derivative ships in levels.
func groundAtlasInput(source image.Image) image.Image {
	const size = 256
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			out.SetNRGBA(x, y, leafSprayPixel(source, x, y, size))
		}
	}
	return out
}

func groundNoise(u, v float64, period, seed int) float64 {
	x, y := u*float64(period), v*float64(period)
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(ix), y-float64(iy)
	fx, fy = fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := groundHash(ix%period, iy%period, seed), groundHash((ix+1)%period, iy%period, seed)
	c, d := groundHash(ix%period, (iy+1)%period, seed), groundHash((ix+1)%period, (iy+1)%period, seed)
	return (a*(1-fx)+b*fx)*(1-fy) + (c*(1-fx)+d*fx)*fy
}

func groundHash(x, y, seed int) float64 {
	value := uint32(x)*0x1f123bb5 ^ uint32(y)*0x5f356495 ^ uint32(seed)*0x6c8e9cf5
	value ^= value >> 16
	value *= 0x7feb352d
	value ^= value >> 15
	value *= 0x846ca68b
	value ^= value >> 16
	return float64(value&0xffffff) / 0x1000000
}

func smoothGround(lower, upper, value float64) float64 {
	value = max(0, min(1, (value-lower)/(upper-lower)))
	return value * value * (3 - 2*value)
}

// vegetationTexture creates the grass cutout and packs the shrub's leaf spray
// above its bark material. Grass roots touch the lower edge for anchored bending.
func vegetationTexture(name string) (*image.NRGBA, error) {
	if name == "small-bush" {
		return shrubMaterialTexture()
	}
	grass, err := vegetationMaster("grass-strip.png")
	if err != nil {
		return nil, err
	}
	// Remove low-alpha generation artifacts; the alpha-tested foliage keeps all
	// visible blade detail. Crop transparent padding below the roots only.
	clean := image.NewNRGBA(grass.Bounds())
	rootBottom := clean.Bounds().Min.Y
	for row := clean.Bounds().Min.Y; row < clean.Bounds().Max.Y; row++ {
		for column := clean.Bounds().Min.X; column < clean.Bounds().Max.X; column++ {
			pixel := color.NRGBAModel.Convert(grass.At(column, row)).(color.NRGBA)
			if pixel.A < 64 {
				pixel = color.NRGBA{}
			}
			if pixel.A >= 128 {
				rootBottom = max(rootBottom, row+1)
			}
			clean.SetNRGBA(column, row, pixel)
		}
	}
	if rootBottom == clean.Bounds().Min.Y {
		return nil, fmt.Errorf("grass master contains no alpha-tested blades")
	}
	cropped := clean.SubImage(image.Rect(clean.Bounds().Min.X, clean.Bounds().Min.Y, clean.Bounds().Max.X, rootBottom))
	const size = 512
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			img.SetNRGBA(column, row, leafSprayPixel(cropped, column, row, size))
		}
	}
	return img, nil
}

// Leaf planes use the entire upper square at v=0..0.5 with a small UV inset;
// branches sample the opaque bark below it at v=.625..875.
func shrubMaterialTexture() (*image.NRGBA, error) {
	leaf, err := vegetationMaster("shrub-leaf-spray.png")
	if err != nil {
		return nil, err
	}
	const size = 512
	img := image.NewNRGBA(image.Rect(0, 0, size, size*2))
	for y := range size {
		for x := range size {
			img.SetNRGBA(x, y, leafSprayPixel(leaf, x, y, size))
		}
	}
	for y := size; y < size*2; y++ {
		for x := range size {
			u, v := float64(x)/size, float64(y-size)/size
			patch := groundNoise(u, v, 8, 715)*.7 + groundNoise(u, v, 23, 716)*.3
			bark := .5 + .5*math.Sin(u*140+groundNoise(u, v, 9, 718)*4)
			shade := color.NRGBA{uint8(66 + patch*27 + bark*10), uint8(49 + patch*21 + bark*7), uint8(31 + patch*14 + bark*5), 255}
			img.SetNRGBA(x, y, shade)
		}
	}
	return img, nil
}

// Average premultiplied source pixels to preserve leaf edges and their alpha
// without introducing dark outlines when reducing the tracked master offline.
func leafSprayPixel(source image.Image, x, y, size int) color.NRGBA {
	bounds := source.Bounds()
	x0, x1 := bounds.Min.X+x*bounds.Dx()/size, bounds.Min.X+(x+1)*bounds.Dx()/size
	y0, y1 := bounds.Min.Y+y*bounds.Dy()/size, bounds.Min.Y+(y+1)*bounds.Dy()/size
	var red, green, blue, alpha uint64
	for row := y0; row < y1; row++ {
		for column := x0; column < x1; column++ {
			r, g, b, a := source.At(column, row).RGBA()
			red += uint64(r)
			green += uint64(g)
			blue += uint64(b)
			alpha += uint64(a)
		}
	}
	if alpha == 0 {
		return color.NRGBA{}
	}
	count := uint64((x1 - x0) * (y1 - y0))
	return color.NRGBA{uint8(red * 255 / alpha), uint8(green * 255 / alpha), uint8(blue * 255 / alpha), uint8(alpha / count >> 8)}
}

// Masters stay in Git; only packed derivatives ship with the level.
func vegetationMaster(name string) (image.Image, error) {
	path := filepath.Join("testdata", "outdoor-lighting", "material-sources", name)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vegetation master %s: %w", name, err)
	}
	pixels, decodeErr := png.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil {
		return nil, fmt.Errorf("decode vegetation master %s: %w", name, decodeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if pixels.Bounds().Dx() != pixels.Bounds().Dy() || pixels.Bounds().Dx() < 512 {
		return nil, fmt.Errorf("vegetation master %s must be square and at least 512px: %v", name, pixels.Bounds())
	}
	return pixels, nil
}

// These are estimated relief maps, not measured surface normals. Smoothed
// alpha gives blade/leaf edges curvature; subtle luminance adds vein detail.
// Normal textures are opaque data. Background and bark remain neutral +Z.
func vegetationNormalTexture(albedo *image.NRGBA, shrub bool) *image.NRGBA {
	bounds := albedo.Bounds()
	normals := image.NewNRGBA(bounds)
	height := make([]float64, bounds.Dx()*bounds.Dy())
	leafBottom := bounds.Max.Y
	if shrub {
		leafBottom = bounds.Min.Y + bounds.Dy()/2
	}
	for row := bounds.Min.Y; row < leafBottom; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := albedo.NRGBAAt(column, row)
			var coverage float64
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if row+dy < leafBottom {
						coverage += float64(albedo.NRGBAAt(column+dx, row+dy).A) / 255
					}
				}
			}
			luminance := (float64(pixel.R)*.2126 + float64(pixel.G)*.7152 + float64(pixel.B)*.0722) / 255
			height[(row-bounds.Min.Y)*bounds.Dx()+column-bounds.Min.X] = .55*coverage/25 + .15*luminance*float64(pixel.A)/255
		}
	}
	sample := func(column, row int) float64 {
		if column < bounds.Min.X || column >= bounds.Max.X || row < bounds.Min.Y || row >= leafBottom {
			return 0
		}
		return height[(row-bounds.Min.Y)*bounds.Dx()+column-bounds.Min.X]
	}
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			normal := color.NRGBA{128, 128, 255, 255}
			if row < leafBottom && albedo.NRGBAAt(column, row).A >= 64 {
				nx := -(sample(column+1, row) - sample(column-1, row)) * 2.5
				ny := -(sample(column, row+1) - sample(column, row-1)) * 2.5
				inverseLength := 1 / math.Sqrt(nx*nx+ny*ny+1)
				encode := func(value float64) uint8 { return uint8(math.Round((value + 1) * 127.5)) }
				normal = color.NRGBA{encode(nx * inverseLength), encode(ny * inverseLength), encode(inverseLength), 255}
			}
			normals.SetNRGBA(column, row, normal)
		}
	}
	return normals
}

// A compacted soil tile has broad earthy variation and sparse small pebbles,
// without masonry seams or a painted repeating trail edge.
func pathTexture() *image.NRGBA {
	const size = 256
	pixels := image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := range size {
		for column := range size {
			u, v := float64(column)/size, float64(row)/size
			patch := groundNoise(u, v, 4, 821)*.6 + groundNoise(u, v, 17, 823)*.4
			grain := (groundHash(column, row, 827) - .5) * 9
			shade := color.NRGBA{uint8(88 + patch*35 + grain), uint8(65 + patch*27 + grain), uint8(43 + patch*18 + grain), 255}
			if groundHash(column/4, row/4, 829) > .988 && math.Pow(float64(column%4)-1.5, 2)+math.Pow(float64(row%4)-1.5, 2) < 2.5 {
				shade = color.NRGBA{uint8(112 + patch*17 + grain), uint8(104 + patch*14 + grain), uint8(83 + patch*11 + grain), 255}
			}
			pixels.SetNRGBA(column, row, shade)
		}
	}
	return pixels
}

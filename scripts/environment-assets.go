//go:build ignore

// environment-assets owns the daylight environment lab's authored layout.
// Run from the repository root with mise run generate-environment.
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
	// One unchanged open grass rectangle, partitioned for bounded high-density
	// floor charts. World-space UVs and ordinary portals hide every grid seam.
	groundEdges := append([]float64{-24}, xedges...)
	groundEdges = append(groundEdges, 24)
	for row := range 4 {
		for column := range len(groundEdges) - 1 {
			result = append(
				result,
				rectangle(
					fmt.Sprintf("test-ground-%d-%d", column, row),
					"ground",
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
	materials := []string{"stone", "tile", "wall", "ceiling", "ground", "wood", "wood-edge", "metal", "furniture-steel", "slate", "glass"}
	images := map[string][]byte{}
	for _, name := range materials {
		var data bytes.Buffer
		pixels := texture(name)
		if err := png.Encode(&data, pixels); err != nil {
			return err
		}
		if name == "ground" {
			directory := filepath.Join("samples", "environment", "material-sources")
			if err := os.MkdirAll(directory, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(directory, "grass-soil-2048.png"), data.Bytes(), 0o644); err != nil {
				return err
			}
			data.Reset()
			if err := png.Encode(&data, groundAtlasInput(pixels)); err != nil {
				return err
			}
		}
		images[name] = data.Bytes()
	}
	for _, preset := range []string{"daylight"} {
		directory := filepath.Join("samples", "environment", "levels", preset)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
		var manifest bytes.Buffer
		fmt.Fprintf(&manifest, "[level]\nname = \"levels.%s\"\nkind = \"world\"\n\n[world]\nsource = \"world.yaml\"\n", preset)
		// Static lighting endpoints are baked offline into streamed chunk pages.
		// Time controls select four pages; cloud cover dims them without rebaking.
		manifest.WriteString(
			"\n[lightmap]\nenabled = true\noffline = true\ndensity = 24\npage_size = 1024\npadding = 2\nbake_samples = 16\nbake_bounces = 1\nbake_denoise = \"medium\"\n\n[lightmap.environment]\ntimes = [0, 9, 12, 17]\ndefault_time = 12\nweather_variants = false\n\n[chunks]\nenabled = true\n",
		)
		for _, name := range materials {
			fmt.Fprintf(&manifest, "\n[[textures]]\nname = %q\nsource = %q\nprofile = \"environment\"\n", name, name+".png")
			// Material classes, not room roles: the field is grass/soil, the wall
			// has brick courses, and the tile/ceiling are rough concrete.
			// Keep this recipe aligned with docs/sample-materials.md.
			preset := map[string]string{"stone": "stone", "tile": "concrete", "wall": "brick", "ceiling": "concrete", "ground": "foliage", "wood": "wood", "wood-edge": "wood", "metal": "metal", "furniture-steel": "metal", "slate": "stone", "glass": "water"}[name]
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
	backdrop := "mountains"
	fmt.Fprintf(
		&data,
		"# $schema: ../../.karty/schemas/levels/%s.schema.json\n# Generated by scripts/environment-assets.go; edit that tracked generator.\nversion: 12\nenvironment:\n  preset: %s\n  backdrop: %s\n  time_of_day: 12\n",
		preset,
		preset,
		backdrop,
	)
	data.WriteString(
		"  sky:\n    cloud_sparse: sky/clouds-sparse.png\n    cloud_dense: sky/clouds-dense.png\n    horizon: sky/mountains.png\n    settings:\n      cloud_uv_scale: [1, 0.95]\n      cloud_dense_scroll_scale: 0.55\n      cloud_dense_offset: [0.19, 0]\n      cloud_parallax: [0.000015, 0.000003]\n      horizon_parallax: 0.0015625\n      horizon_uv_scale: 1.5\n      horizon_haze: 0.35\n      horizon_emission: [0, 0, 0]\n",
	)
	data.WriteString(
		"uv: {mode: triplanar, scale: {x: 2, y: 2}}\nlighting:\n  version: 1\n  actors: true\n  ambient: {x: 0.055, y: 0.065, z: 0.075}\n  lights:\n    - {id: observation-fill, position: {x: 0, y: -8, z: 3}, color: {x: 0.20, y: 0.17, z: 0.12}, radius: 7.5}\n    - {id: passage-fill, position: {x: 14, y: -3, z: 3}, color: {x: 0.16, y: 0.20, z: 0.22}, radius: 5.5}\nrooms:\n",
	)
	for _, r := range rooms {
		fmt.Fprintf(&data, "  - id: %s\n", r.id)
		if r.sky {
			data.WriteString("    sky: true\n")
		}
		if strings.HasPrefix(r.id, "test-ground-") {
			data.WriteString("    floor_uv: {mode: triplanar, scale: {x: 12, y: 12}}\n")
		}
		fmt.Fprintf(
			&data,
			"    floor: {c: %g}\n    ceiling: {c: %g}\n    floor_material: %s\n    ceiling_material: ceiling\n    boundary:\n",
			r.floor,
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
		heights := make([]float64, count+1)
		for node := range heights {
			heights[node] = .88
			if node > 1 && node < count-1 {
				heights[node] += .04 * math.Sin(float64(node)*2.3+float64(len(span.id)))
			}
		}
		for index := range count {
			a := start + (end-start)*float64(index)/float64(count)
			b := start + (end-start)*float64(index+1)/float64(count)
			slope := (heights[index+1] - heights[index]) / (b - a)
			axis := "b"
			x0, y0, x1, y1 := span.x0, a, span.x1, b
			if span.alongX {
				axis = "a"
				x0, y0, x1, y1 = a, span.y0, b, span.y1
			}
			solid(data, fmt.Sprintf("boundary-%s-%d", span.id, index), x0, y0, x1, y1,
				"{c: -0.08}", fmt.Sprintf("{%s: %g, c: %g}", axis, slope, heights[index]-slope*a), "stone")
		}
	}
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
			pixel := color.NRGBAModel.Convert(source.At(x*source.Bounds().Dx()/size, y*source.Bounds().Dy()/size)).(color.NRGBA)
			out.SetNRGBA(x, y, pixel)
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

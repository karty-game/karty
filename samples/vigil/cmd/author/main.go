// Author prepares original image sheets and the shared crypt geometry.
// Run from the CLI repository with mise run author-vigil.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"example.com/vigil/internal/dungeon"
)

type record = map[string]any

var solids, contents, lights []record

const (
	target = "levels/crypt"
	// Authoring runs from samples/vigil; game builds use only retained level assets.
	sourceArt = "../../scripts/vigil/assets"
)

func main() {
	must(os.MkdirAll(target, 0755))
	prepareImages()
	for i, b := range dungeon.Obstacles {
		id := fmt.Sprintf("obstacle-%02d", i)
		sprite(id+"-contact", "shadow", b.X+b.W/2, b.Y+b.D/2, .023, b.W+.75, b.D+.75, "fixed", "blend")
		contact := contents[len(contents)-1]["actor"].(record)
		contact["pitch_degrees"] = 90
		contact["sprite"].(record)["origin_y"] = .5
		switch {
		case i < 4:
			pew(id, b)
		case i < 8:
			column(id, b)
		case i < 14:
			sarcophagus(id, b)
		default:
			box(id, b.X, b.Y, b.W, b.D, 0, 1.02, "stone", true)
			box(id+"-cloth", b.X-.08, b.Y-.07, b.W+.16, b.D+.14, 1.02, 1.07, "runner", false)
		}
	}
	// A worn processional runner with inset stone edging.
	box("runner", 10.9, 1.3, 2.2, 21.7, .008, .018, "runner", false)
	for _, x := range []float64{10.76, 13.1} {
		box(fmt.Sprintf("border-%g", x), x, 1.3, .14, 21.7, .008, .025, "stone", false)
	}
	// Chancel steps are decorative only; the route ends before the raised altar.
	for i := range 3 {
		box(
			fmt.Sprintf("altar-step-%d", i),
			9.7+float64(i)*.18,
			23.15+float64(i)*.1,
			4.6-float64(i)*.36,
			1.65-float64(i)*.2,
			0,
			float64(i+1)*.12,
			"stone",
			false,
		)
	}
	// Large Latin cross, freestanding behind the altar; no borrowed game assets.
	box("altar-cross-stem", 11.9, 24.58, .2, .16, 1.1, 3.85, "gold", false)
	box("altar-cross-arms", 11.1, 24.58, 1.8, .16, 2.85, 3.05, "gold", false)
	// Far-wall buttresses with stepped caps and pointed inset arches. Front
	// boundary faces naturally cull from the fixed cutaway viewpoint.
	for i, x := range []float64{6.2, 9, 14.4, 17.3} {
		box(fmt.Sprintf("apse-buttress-%d", i), x, 24.55, .5, .45, 0, 3.7, "stone", false)
		box(fmt.Sprintf("apse-cap-%d", i), x-.08, 24.5, .66, .5, 3.7, 3.9, "stone", false)
	}
	for i, x := range []float64{7.8, 15.8} {
		sprite(fmt.Sprintf("window-%d", i), "window", x, 24.91, 1.7, 1.6, 2.4, "fixed", "cutout")
		lamp(fmt.Sprintf("window-light-%d", i), x, 24, 2.6, []float64{.18, .27, .42}, 5)
	}
	// Pointed arcade: each rib is a convex extrusion with a sloped underside.
	// Six short segments describe the Gothic profile without imported meshes.
	for _, x := range []float64{1.8, 4.4, 19.6, 22.2} {
		arch(fmt.Sprintf("transept-arch-%g", x), x, 16.78, false)
	}
	for _, y := range []float64{2.3, 5.8, 19.2, 22.2} {
		arch(fmt.Sprintf("nave-arch-%g", y), 17.78, y, true)
	}
	// Stone base courses add scale without making a second wall mesh.
	for i := 0; i < 12; i++ {
		a, b := dungeon.Outline[i], dungeon.Outline[(i+1)%12]
		if a.Y == b.Y && a.Y > 1 {
			box(fmt.Sprintf("course-%d", i), math.Min(a.X, b.X), a.Y-.12, math.Abs(a.X-b.X), .12, 0, .2, "stone", false)
		}
	}
	// Rows of candles, with visible holders and independent flame phases.
	for i, p := range []dungeon.Point{{7, 2.5}, {17, 2.5}, {7, 12.5}, {17, 12.5}, {8, 22}, {16, 22}, {1, 11.7}, {23, 11.7}, {1, 15.8}, {23, 15.8}} {
		candle(fmt.Sprintf("candle-%d", i), p.X, p.Y, 0, .65+float64(i%3)*.14, true)
	}
	for i, p := range dungeon.Shrines {
		id := fmt.Sprintf("vigil-%d", i)
		box(id+"-plinth", p.X-.28, p.Y-.28, .56, .56, 0, .32, "stone", false)
		box(id+"-stem", p.X-.055, p.Y-.055, .11, .11, .32, 1.15, "gold", false)
		box(id+"-bowl", p.X-.22, p.Y-.22, .44, .44, 1.08, 1.2, "gold", false)
	}
	// Candles flanking the reliquary, resting on its lid.
	for i, x := range []float64{10.9, 11.2, 12.8, 13.1} {
		candle(fmt.Sprintf("altar-candle-%d", i), x, 23.9, 1.08, .3+float64(i%2)*.12, true)
	}
	prop("altar-saint", "z-saint", 11.05, 24.15, 1.08, .8)
	prop("altar-scripture", "z-books", 12.3, 23.6, 1.08, 1)
	for i, p := range []dungeon.Point{{2.1, 10.4}, {22.1, 15.2}, {8, 19.3}, {16.1, 19.3}} {
		prop(fmt.Sprintf("memorial-votives-%d", i), "z-votives", p.X, p.Y, .995, .6)
	}
	// Sparse broken chips sit along wall bases, leaving the procession clear.
	for i := range 24 {
		x := .6 + float64((i*17)%50)*.1
		y := 16.45 - float64(i%4)*.13
		if i >= 12 {
			x += 18
		}
		box(fmt.Sprintf("fallen-stone-%d", i), x, y, .08+float64(i%3)*.04, .09, 0, .05+float64(i%4)*.025, "stone", false)
	}
	// Wear follows the architecture: damp walls, debris at their feet, dirty
	// aisle edges and a few old blood marks in the burial chambers.
	for i, p := range []dungeon.Point{{7, 2.3}, {16.7, 6.5}, {1.8, 11.7}, {5.1, 15.8}, {18.8, 9}, {22.5, 16}, {8, 21.7}, {16.5, 23.4}} {
		groundDecal(fmt.Sprintf("damp-floor-%d", i), "z-decal-damp", p.X, p.Y, 1.7+float64(i%3)*.35, 1.4, float64(i*47))
	}
	for i, p := range []dungeon.Point{{7, 8.7}, {5, 16.2}, {1.5, 8.6}, {19.1, 16}, {23, 9}, {17, 20.7}} {
		groundDecal(fmt.Sprintf("rubble-%d", i), "z-decal-rubble", p.X, p.Y, 1.2, 1.0, float64(i*71))
	}
	for i, p := range []dungeon.Point{{4.8, 10.9}, {19.2, 13.9}, {8.5, 20.2}} {
		groundDecal(fmt.Sprintf("blood-%d", i), "z-decal-blood", p.X, p.Y, 1.1, 1.55, float64(35+i*79))
	}
	for i, p := range []dungeon.Point{{1.7, 16.97}, {4.8, 16.97}, {20, 16.97}, {22.5, 16.97}, {9.4, 24.97}, {14.4, 24.97}} {
		wallDecal(fmt.Sprintf("wall-seep-%d", i), p.X, p.Y, .1, 1.25, 2.7, 0)
	}
	for i, p := range []dungeon.Point{{17.97, 2}, {17.97, 6.5}, {23.97, 10}, {23.97, 15.2}, {17.97, 21.8}} {
		wallDecal(fmt.Sprintf("side-seep-%d", i), p.X, p.Y, .1, 1.1, 2.5, 90)
	}
	// Memorial groupings belong to the wall bays, rather than scattering
	// isolated props across the navigable procession. Each cluster uses the
	// existing small upright accents and fixed ground marks.
	for i, p := range []dungeon.Point{{6.45, 3.1}, {17.48, 6.55}, {6.52, 20.5}} {
		groundDecal(fmt.Sprintf("memorial-wear-%d", i), "z-decal-damp", p.X, p.Y, .7, 1.1, float64(i*27))
		prop(fmt.Sprintf("memorial-books-%d", i), "z-books", p.X, p.Y, .035, .36)
	}
	// The hero's tiny warm pool anchors the otherwise cold limestone palette.
	lamp("nave-fill", 12, 7, 3, []float64{.16, .2, .29}, 7.4)
	lamp("crossing-fill", 12, 13, 3.5, []float64{.15, .2, .3}, 8)
	lamp("altar-fill", 12, 23.2, 3.2, []float64{.72, .46, .22}, 5.5)
	boundary := []record{}
	for i, p := range dungeon.Outline {
		q := dungeon.Outline[(i+1)%len(dungeon.Outline)]
		a, b := dungeon.OuterCorner(i), dungeon.OuterCorner((i+1)%len(dungeon.Outline))
		// Solid outer bands preserve the original inner footprint and navigation.
		// The near sides use low, capped masonry for the fixed overhead cutaway.
		solids = append(
			solids,
			record{
				"id":              fmt.Sprintf("masonry-%02d", i),
				"footprint":       []record{xy(p.X, p.Y), xy(a.X, a.Y), xy(b.X, b.Y), xy(q.X, q.Y)},
				"bottom":          record{"c": 0},
				"top":             record{"c": dungeon.WallHeight(i)},
				"side_material":   "wall",
				"top_material":    "stone",
				"bottom_material": "stone",
				"top_uv":          record{"mode": "planar", "scale": xy(1.4, 1.4)},
				"collision":       true,
			},
		)
		boundary = append(boundary, record{"id": fmt.Sprintf("edge-%d", i), "start": xy(a.X, a.Y), "end": xy(b.X, b.Y), "material": "wall"})
	}
	world := record{
		"version": 8,
		"rooms": []record{
			{
				"id":               "abbey",
				"boundary":         boundary,
				"floor":            record{"c": 0},
				"ceiling":          record{"c": dungeon.VisionHeight},
				"floor_material":   "floor",
				"ceiling_material": "wall",
				"floor_uv":         record{"scale": xy(2.4, 1.2)},
				"wall_uv":          record{"mode": "wrap", "scale": xy(1.4, 1.4)},
				"floor_secondary": record{
					"texture":  "z-dirt-variation",
					"strength": .38,
					"uv":       record{"mode": "planar", "scale": xy(6.3, 7.9), "rotation_degrees": 17, "offset": xy(.13, .37)},
				},
				"wall_secondary": record{
					"texture":  "z-dirt-variation",
					"strength": .2,
					"uv":       record{"mode": "wrap", "scale": xy(6.7, 3.7), "offset": xy(.23, .47)},
				},
			},
		},
		"solids":   solids,
		"contents": contents,
		"lighting": record{"version": 1, "actors": true, "ambient": xyz(.085, .1, .135), "lights": lights},
		"animations": record{
			"version": 1,
			"presets": []record{{"name": "candle-fire", "kind": "flipbook", "frames": []string{"flame-0", "flame-1", "flame-2"}, "fps": 9}},
		},
		"uv": record{"mode": "triplanar", "scale": xy(.8, .8)},
	}
	// JSON is a strict YAML subset; checked-in output remains directly editable.
	data, err := json.MarshalIndent(world, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(target, "world.yaml"), append(data, '\n'), 0644))
	writeManifest()
	writeSightArt()
	fmt.Printf(
		"Vigil: %d solids, %d actors, %d lights, %d walk frames and %d idle poses\n",
		len(solids),
		len(contents),
		len(lights),
		dungeon.WalkTextureCount,
		dungeon.WalkDirections,
	)
}

func xy(x, y float64) record     { return record{"x": x, "y": y} }
func xyz(x, y, z float64) record { return record{"x": x, "y": y, "z": z} }
func box(id string, x, y, w, d, bottom, top float64, material string, collision bool) {
	solids = append(
		solids,
		record{
			"id":              id,
			"footprint":       []record{xy(x, y), xy(x+w, y), xy(x+w, y+d), xy(x, y+d)},
			"bottom":          record{"c": bottom},
			"top":             record{"c": top},
			"side_material":   material,
			"top_material":    material,
			"bottom_material": material,
			"collision":       collision,
		},
	)
	if material == "tomb" {
		s := solids[len(solids)-1]
		s["side_material"] = "stone"
		s["top_uv"] = record{"mode": "planar", "scale": xy(w, d), "offset": xy(-x/w, -y/d)}
	}
	if material == "runner" {
		solids[len(solids)-1]["top_uv"] = record{"mode": "planar", "scale": xy(w, 4), "offset": xy(-x/w, 0)}
	}
}

func arch(id string, x, y float64, side bool) {
	for _, sign := range []float64{-1, 1} {
		if side {
			box(fmt.Sprintf("%s-jamb-%g", id, sign), x, y+sign*.85-.1, .22, .2, .2, 2.6, "stone", false)
		} else {
			box(fmt.Sprintf("%s-jamb-%g", id, sign), x+sign*.85-.1, y, .2, .22, .2, 2.6, "stone", false)
		}
	}
	for i := range 12 {
		a := -.85 + float64(i)*1.7/12 + .004
		b := a + 1.7/12 - .008
		za := 2.35 + 1.15*(1-math.Pow(math.Abs(a)/.85, .7))
		zb := 2.35 + 1.15*(1-math.Pow(math.Abs(b)/.85, .7))
		slope := (zb - za) / (b - a)
		if side {
			box(fmt.Sprintf("%s-rib-%d", id, i), x, y+a, .22, b-a, 2, 3.85, "stone", false)
			solids[len(solids)-1]["bottom"] = record{"b": slope, "c": za - slope*(y+a)}
			solids[len(solids)-1]["top"] = record{"b": slope, "c": za - slope*(y+a) + .23}
		} else {
			box(fmt.Sprintf("%s-rib-%d", id, i), x+a, y, b-a, .22, 2, 3.85, "stone", false)
			solids[len(solids)-1]["bottom"] = record{"a": slope, "c": za - slope*(x+a)}
			solids[len(solids)-1]["top"] = record{"a": slope, "c": za - slope*(x+a) + .23}
		}
	}
}

// Small devotional accents retain their original pre-rendered artwork. Large
// furnishings use solid geometry so their visible volume also writes depth.
func prop(id, texture string, x, y, z, width float64) {
	standingSprite(id, texture, x, y, z, width, 0, "cutout")
}

func groundDecal(id, texture string, x, y, w, h, yaw float64) {
	sprite(id, texture, x, y, .034, w, h, "fixed", "blend")
	a := contents[len(contents)-1]["actor"].(record)
	a["pitch_degrees"] = 90
	a["yaw_degrees"] = yaw
	a["sprite"].(record)["origin_y"] = .5
}

func wallDecal(id string, x, y, z, w, h, yaw float64) {
	sprite(id, "z-decal-seep", x, y, z, w, h, "fixed", "blend")
	contents[len(contents)-1]["actor"].(record)["yaw_degrees"] = yaw
}
func sprite(id, texture string, x, y, z, w, h float64, facing, alpha string) {
	contents = append(
		contents,
		record{
			"id":       id,
			"kind":     "actor",
			"position": xyz(x, y, z),
			"actor": record{
				"sprite": record{
					"texture":  texture,
					"facing":   facing,
					"alpha":    alpha,
					"width":    w,
					"height":   h,
					"origin_x": .5,
					"origin_y": 1,
				},
			},
		},
	)
}
func lamp(id string, x, y, z float64, c []float64, r float64) {
	lights = append(lights, record{"id": id, "position": xyz(x, y, z), "color": xyz(c[0], c[1], c[2]), "radius": r})
}
func candle(id string, x, y, z, h float64, lit bool) {
	if z == 0 && lit {
		prop(id+"-votives", "z-votives", x, y, .025, .55)
		lamp(id+"-light", x, y, .65, []float64{.85, .48, .2}, 3.65)
		return
	}
	box(id+"-foot", x-.11, y-.11, .22, .22, z, z+.08, "gold", false)
	box(id+"-wax", x-.045, y-.045, .09, .09, z+.08, z+h, "wax", false)
	if lit {
		standingSprite(id+"-flame", "flame-0", x, y, z+h, .19, .36, "blend")
		contents[len(contents)-1]["actor"].(record)["animation"] = record{"preset": "candle-fire", "phase_seconds": math.Mod(x+y, 1)}
		lamp(id+"-light", x, y, z+h+.15, []float64{.85, .48, .2}, 3.65)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func load(path string) image.Image {
	f, e := os.Open(path)
	must(e)
	defer f.Close()
	im, e := png.Decode(f)
	must(e)
	return im
}
func save(name string, im image.Image) {
	f, e := os.Create(filepath.Join(target, name+".png"))
	must(e)
	must(png.Encode(f, im))
	must(f.Close())
}
func crop(im image.Image, r image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), im, r.Min, draw.Src)
	return dst
}

func prepareImages() {
	prepareRenderedCharacter()
	// Generated prop placement is not a uniform atlas: retain explicit regions
	// so tall plinths and mouldings are never clipped by an inferred grid.
	props := load(filepath.Join(sourceArt, "crypt-props.png"))
	regions := []struct {
		name string
		r    image.Rectangle
	}{
		{"z-column", image.Rect(230, 0, 425, 445)},
		{"z-pew", image.Rect(700, 0, 1248, 425)},
		{"z-sarcophagus", image.Rect(50, 445, 620, 890)},
		{"z-saint", image.Rect(805, 417, 1045, 883)},
		{"z-votives", image.Rect(120, 880, 530, 1248)},
		{"z-books", image.Rect(640, 840, 1248, 1248)},
	}
	for _, region := range regions {
		im := crop(props, region.r)
		if region.name == "z-books" { // Exclude the neighbouring statue's feet.
			for y := 0; y < 44; y++ {
				for x := 0; x < 410; x++ {
					im.SetNRGBA(x, y, color.NRGBA{})
				}
			}
		}
		save(region.name, trim(im))
	}
	materials := load(filepath.Join(sourceArt, "crypt-materials.png"))
	b := materials.Bounds()
	w, h := b.Dx()/2, b.Dy()/2
	for i, name := range []string{"floor", "wall", "runner"} {
		x, y := (i%2)*w, (i/2)*h
		save(name, crop(materials, image.Rect(x, y, x+w, y+h)))
	}
	// Preserve the dedicated floor image's 2:1 aspect through its authored UV
	// scale; the renderer's bounded material tiles remain the ordinary 256².
	save("floor", load(filepath.Join(sourceArt, "crypt-floor.png")))
	decals := load(filepath.Join(sourceArt, "crypt-decals.png"))
	db := decals.Bounds()
	dw, dh := db.Dx()/2, db.Dy()/2
	for i, name := range []string{"z-decal-damp", "z-decal-rubble", "z-decal-blood", "z-decal-seep"} {
		x, y := (i%2)*dw, (i/2)*dh
		save(name, crop(decals, image.Rect(x, y, x+dw, y+dh)))
	}
	variation := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := range 256 {
		for x := range 256 {
			v := .56*noise(
				float64(x)*4/256,
				float64(y)*4/256,
				4,
			) + .28*noise(
				float64(x)*7/256,
				float64(y)*7/256,
				7,
			) + .16*noise(
				float64(x)*13/256,
				float64(y)*13/256,
				13,
			)
			c := uint8(188 + v*37)
			variation.SetNRGBA(x, y, color.NRGBA{c, c, c, 255})
		}
	}
	save("z-dirt-variation", variation)
	for i, name := range []string{"stone", "gold", "wood", "wax"} {
		if name == "stone" {
			// The original limestone artwork gives mouldings and capitals the
			// same worn surface detail as the floor, rather than flat grey paint.
			save(name, crop(materials, image.Rect(0, 0, w, h)))
			continue
		}
		base := [][3]float64{{113, 110, 98}, {117, 87, 43}, {74, 51, 34}, {193, 173, 126}}[i]
		im := image.NewNRGBA(image.Rect(0, 0, 128, 128))
		for y := range 128 {
			for x := range 128 {
				v := float64((x*71+y*119+x*y*17)%29) - 14
				if name == "wood" {
					// Broad longitudinal grain with quieter pores, so carved rails
					// retain their silhouette instead of disappearing into noise.
					v *= .42
					v += math.Sin(float64(x)*.31+math.Sin(float64(y)*.025)*1.7) * 6
					v += math.Sin(float64(x)*.075+float64(y)*.007) * 3
				}
				im.SetNRGBA(x, y, color.NRGBA{uint8(base[0] + v), uint8(base[1] + v), uint8(base[2] + v), 255})
			}
		}
		save(name, im)
	}
	for frame := range 3 {
		im := image.NewNRGBA(image.Rect(0, 0, 64, 96))
		for y := range 96 {
			for x := range 64 {
				fy := float64(y) / 96
				cx := 32 + math.Sin(fy*8+float64(frame)*2)*(1-fy)*7
				r := math.Hypot((float64(x)-cx)/(4+fy*12), (float64(y)-58)/42)
				if r < 1 {
					a := uint8(math.Min(1, (1-r)*4) * 235)
					im.SetNRGBA(x, y, color.NRGBA{255, uint8(150 + 90*(1-r)), uint8(25 + 170*(1-r)*(1-r)), a})
				}
			}
		}
		save(fmt.Sprintf("flame-%d", frame), im)
	}
	prepareFireEffectImages()
	shadow := image.NewNRGBA(image.Rect(0, 0, 96, 96))
	for y := range 96 {
		for x := range 96 {
			r := math.Hypot((float64(x)-48)/45, (float64(y)-48)/45)
			if r < 1 {
				shadow.SetNRGBA(x, y, color.NRGBA{4, 5, 8, uint8(110 * (1 - r) * (1 - r))})
			}
		}
	}
	save("shadow", shadow)
	window := image.NewNRGBA(image.Rect(0, 0, 128, 192))
	for y := range 192 {
		for x := range 128 {
			fx, fy := float64(x-64)/60, float64(y)/192
			if math.Abs(fx) > .9 || fy < .32*math.Abs(fx)+.02 {
				continue
			}
			c := color.NRGBA{uint8(48 + (x*13+y*7)%18), 31, uint8(37 + (x*5+y*3)%14), 255}
			if x%24 < 3 || y%28 < 3 {
				c = color.NRGBA{106, 84, 47, 255}
			}
			if math.Abs(fx) < .055 || y > 72 && y < 82 {
				c = color.NRGBA{205, 158, 78, 255}
			}
			if math.Abs(fx) > .78 || fy < .32*math.Abs(fx)+.09 {
				c = color.NRGBA{71, 68, 59, 255}
			}
			window.SetNRGBA(x, y, c)
		}
	}
	save("window", window)
	// A shared opaque texel lets the 2D sight mask batch ordinary sprite draws.
	// Generating geometric mask data needs no additional authored artwork.
	black := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	draw.Draw(black, black.Bounds(), image.NewUniform(color.NRGBA{A: 255}), image.Point{}, draw.Src)
	save("z-vision", black)
}

func noise(x, y float64, period int) float64 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	fx = fx * fx * (3 - 2*fx)
	fy = fy * fy * (3 - 2*fy)
	hash := func(a, b int) float64 {
		a, b = a%period, b%period
		n := uint32(a)*374761393 + uint32(b)*668265263
		n = (n ^ (n >> 13)) * 1274126177
		return float64(n^(n>>16))/4294967295*2 - 1
	}
	a := hash(x0, y0)*(1-fx) + hash(x0+1, y0)*fx
	b := hash(x0, y0+1)*(1-fx) + hash(x0+1, y0+1)*fx
	return a*(1-fy) + b*fy
}

func trim(im *image.NRGBA) *image.NRGBA {
	b := im.Bounds()
	r := image.Rectangle{Min: b.Max}
	for y := range b.Dy() {
		for x := range b.Dx() {
			if im.NRGBAAt(x, y).A > 20 {
				r.Min.X = min(r.Min.X, x)
				r.Min.Y = min(r.Min.Y, y)
				r.Max.X = max(r.Max.X, x+1)
				r.Max.Y = max(r.Max.Y, y+1)
			}
		}
	}
	if r.Empty() {
		panic("empty prop region")
	}
	// Two transparent pixels retain filtering safety without changing the base.
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx()+4, r.Dy()+4))
	draw.Draw(dst, image.Rect(2, 2, r.Dx()+2, r.Dy()+2), im, r.Min, draw.Src)
	return dst
}

func writeManifest() { writeCompactManifest() }

package main

import (
	"fmt"

	"example.com/vigil/internal/dungeon"
)

// Every furnishing has a full-footprint blocking base, matching the navigator's
// conservative rectangle. All relief remains inside that same walking boundary.
//
//nolint:mnd // Dimensions describe the authored carving profile in world units.
func pew(identifier string, bounds dungeon.Box) {
	first := len(solids)

	box(identifier+"-base", bounds.X, bounds.Y, bounds.W, bounds.D, 0, .1, "wood", true)
	box(identifier+"-seat", bounds.X+.16, bounds.Y+.04, bounds.W-.32, bounds.D-.21, .43, .55, "wood", false)
	box(identifier+"-cushion", bounds.X+.23, bounds.Y+.09, bounds.W-.46, bounds.D-.32, .55, .575, "runner", false)
	box(identifier+"-back", bounds.X+.16, bounds.Y+bounds.D-.16, bounds.W-.32, .105, .55, 1.16, "wood", false)
	chamfered(
		identifier+"-crest",
		dungeon.Box{X: bounds.X + .14, Y: bounds.Y + bounds.D - .19, W: bounds.W - .28, D: .17},
		.035,
		1.16,
		1.24,
		"wood",
	)

	for side, x := range []float64{bounds.X + .025, bounds.X + bounds.W - .205} {
		pewEnd(fmt.Sprintf("%s-end-%d", identifier, side), dungeon.Box{X: x, Y: bounds.Y, W: .18, D: bounds.D})
	}
	// Panels are sunk behind the framing on both faces: the fixed camera can
	// see either face after a route crosses the pew. No coplanar painted fronts.
	for face, faceY := range []float64{bounds.Y + bounds.D - .185, bounds.Y + bounds.D - .055} {
		name := fmt.Sprintf("%s-face-%d", identifier, face)
		box(name+"-lower-rail", bounds.X+.18, faceY, bounds.W-.36, .025, .64, .70, "wood", false)
		box(name+"-upper-rail", bounds.X+.18, faceY, bounds.W-.36, .025, 1.08, 1.14, "wood", false)

		for panel := range 4 {
			x := bounds.X + .2 + float64(panel)*(bounds.W-.44)/3
			box(fmt.Sprintf("%s-stile-%d", name, panel), x, faceY, .04, .025, .70, 1.08, "wood", false)
		}

		box(name+"-cross-stem", bounds.X+bounds.W/2-.021, faceY, .042, .03, .76, 1.04, "gold", false)
		box(name+"-cross-arms", bounds.X+bounds.W/2-.10, faceY, .2, .03, .94, .982, "gold", false)
	}

	mapPewGrain(solids[first:])
}

// The arm rises into the rear finial through a continuous carved profile.
// Each segment is a convex depth-writing prism with a genuinely sloping cap.
//
//nolint:mnd // Dimensions describe the authored carving profile in world units.
func pewEnd(identifier string, bounds dungeon.Box) {
	profile := [...]dungeon.Point{
		{X: .025, Y: .88},
		{X: .12, Y: .96},
		{X: .28, Y: .86},
		{X: .42, Y: .87},
		{X: bounds.D - .19, Y: 1.25},
		{X: bounds.D - .08, Y: 1.34},
		{X: bounds.D - .025, Y: 1.28},
	}
	for i, a := range profile[:len(profile)-1] {
		c := profile[i+1]
		box(fmt.Sprintf("%s-profile-%d", identifier, i), bounds.X, bounds.Y+a.X, bounds.W, c.X-a.X, .16, 1, "wood", false)
		slope := (c.Y - a.Y) / (c.X - a.X)
		solids[len(solids)-1]["top"] = record{"b": slope, "c": a.Y - slope*(bounds.Y+a.X)}
	}

	chamfered(
		identifier+"-foot",
		dungeon.Box{X: bounds.X - .012, Y: bounds.Y + .018, W: bounds.W + .024, D: bounds.D - .036},
		.035,
		.1,
		.2,
		"wood",
	)
	chamfered(identifier+"-finial", dungeon.Box{X: bounds.X + .015, Y: bounds.Y + bounds.D - .19, W: .15, D: .16}, .035, 1.25, 1.43, "wood")
}

//nolint:errcheck,forcetypeassert,mnd // All items are author-owned solid records; grain rotations are art dimensions.
func mapPewGrain(items []record) {
	for _, solid := range items {
		if solid["side_material"] != "wood" {
			continue
		}

		rotation := 0.0
		// Rails/seat grain follows their length; upright boards retain vertical
		// grain. Sloping end pieces retain the same scale across their joints.
		top, bottom := solid["top"].(record), solid["bottom"].(record)
		if len(top) == 1 && top["c"].(float64)-bottom["c"].(float64) <= .18 {
			rotation = 90
		}

		solid["side_uv"] = record{"mode": "planar", "scale": xy(.8, 1.4), "rotation_degrees": rotation}
		solid["top_uv"] = record{"mode": "planar", "scale": xy(1.4, .7), "rotation_degrees": 90.0}
	}
}

//nolint:mnd // Dimensions describe the authored carving profile in world units.
func column(identifier string, bounds dungeon.Box) {
	box(identifier+"-base", bounds.X, bounds.Y, bounds.W, bounds.D, 0, .12, "stone", true)
	chamfered(identifier+"-plinth", inset(bounds, .025), .12, .12, .23, "stone")
	chamfered(identifier+"-lower-ring", inset(bounds, .055), .14, .23, .32, "stone")
	chamfered(identifier+"-base-neck", inset(bounds, .12), .14, .32, .42, "stone")
	chamfered(identifier+"-shaft", inset(bounds, .18), .12, .42, 2.77, "stone")
	// Individual torus bases and faceted capitals make the engaged shafts read
	// as clustered Gothic masonry rather than stripes on a square pillar.
	for i, p := range []dungeon.Point{{X: bounds.X + .13, Y: bounds.Y + bounds.D/2}, {X: bounds.X + bounds.W - .13, Y: bounds.Y + bounds.D/2}, {X: bounds.X + bounds.W/2, Y: bounds.Y + .13}, {X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.D - .13}} {
		name := fmt.Sprintf("%s-engaged-%d", identifier, i)
		chamfered(name+"-foot", dungeon.Box{X: p.X - .105, Y: p.Y - .105, W: .21, D: .21}, .055, .32, .45, "stone")
		chamfered(name+"-shaft", dungeon.Box{X: p.X - .075, Y: p.Y - .075, W: .15, D: .15}, .043, .45, 2.77, "stone")
		chamfered(name+"-capital", dungeon.Box{X: p.X - .105, Y: p.Y - .105, W: .21, D: .21}, .065, 2.77, 2.95, "stone")
	}

	chamfered(identifier+"-neck", inset(bounds, .13), .12, 2.77, 2.87, "stone")
	chamfered(identifier+"-capital", inset(bounds, .075), .16, 2.87, 3.02, "stone")
	chamfered(identifier+"-abacus", inset(bounds, .035), .11, 3.02, 3.13, "stone")
	chamfered(identifier+"-cap", bounds, .07, 3.13, 3.22, "stone")
}

//nolint:mnd // Dimensions describe the authored carving profile in world units.
func sarcophagus(identifier string, bounds dungeon.Box) {
	box(identifier+"-base", bounds.X, bounds.Y, bounds.W, bounds.D, 0, .12, "stone", true)
	chamfered(identifier+"-plinth", inset(bounds, .035), .08, .12, .22, "stone")
	chamfered(identifier+"-plinth-bead", inset(bounds, .075), .07, .22, .28, "stone")
	chamfered(identifier+"-body", inset(bounds, .15), .06, .28, .73, "stone")
	chamfered(identifier+"-lower-rail", inset(bounds, .11), .065, .28, .34, "stone")
	chamfered(identifier+"-upper-rail", inset(bounds, .10), .065, .69, .76, "stone")
	chamfered(identifier+"-lid-shadow", inset(bounds, .13), .07, .76, .79, "stone")
	chamfered(identifier+"-lid", inset(bounds, .025), .09, .79, .88, "stone")
	chamfered(identifier+"-lid-inset", inset(bounds, .10), .085, .88, .94, "stone")
	chamfered(identifier+"-memorial-panel", inset(bounds, .19), .07, .94, .96, "stone")
	box(identifier+"-cross-stem", bounds.X+.48, bounds.Y+bounds.D/2-.052, bounds.W-.96, .104, .96, .99, "stone", false)
	box(identifier+"-cross-arms", bounds.X+bounds.W*.61-.052, bounds.Y+.34, .104, bounds.D-.68, .96, .99, "stone", false)
	// Three recessed coffers on each long side are divided by engaged
	// pilasters. Side decoration remains within the projecting lid's footprint.
	for face, faceY := range []float64{bounds.Y + .12, bounds.Y + bounds.D - .17} {
		for panel := range 4 {
			x := bounds.X + .24 + float64(panel)*(bounds.W-.6)/3
			box(fmt.Sprintf("%s-face-%d-pilaster-%d", identifier, face, panel), x, faceY, .12, .05, .34, .69, "stone", false)
		}
	}
}

func inset(bounds dungeon.Box, amount float64) dungeon.Box {
	return dungeon.Box{X: bounds.X + amount, Y: bounds.Y + amount, W: bounds.W - 2*amount, D: bounds.D - 2*amount}
}

func chamfered(identifier string, bounds dungeon.Box, bevel, bottom, top float64, material string) {
	solids = append(
		solids,
		record{
			"id": identifier,
			"footprint": []record{
				xy(bounds.X+bevel, bounds.Y),
				xy(bounds.X+bounds.W-bevel, bounds.Y),
				xy(bounds.X+bounds.W, bounds.Y+bevel),
				xy(bounds.X+bounds.W, bounds.Y+bounds.D-bevel),
				xy(bounds.X+bounds.W-bevel, bounds.Y+bounds.D),
				xy(bounds.X+bevel, bounds.Y+bounds.D),
				xy(bounds.X, bounds.Y+bounds.D-bevel),
				xy(bounds.X, bounds.Y+bevel),
			},
			"bottom":          record{"c": bottom},
			"top":             record{"c": top},
			"side_material":   material,
			"top_material":    material,
			"bottom_material": material,
			"collision":       false,
		},
	)
}

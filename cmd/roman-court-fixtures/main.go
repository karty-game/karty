// roman-court-fixtures regenerates the basin and torch prefabs in the fixture.
package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strings"
)

const begin = "  # BEGIN generated hanging-brazier prefab\n"
const end = "  # END generated hanging-brazier prefab\n"
const chainLinks = 12
const chainBottom = 4.49
const chainTop = 8.05
const chainOverlap = .04
const quarterArcDivisor = 12
const cornerSegments = 6

type point struct{ x, y float64 }

func main() {
	const path = "testdata/roman-court/levels/showcase/world.yaml"

	root, err := os.OpenRoot(".")
	if err != nil {
		panic(err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var out bytes.Buffer
	out.WriteString(begin + "  - id: hanging-brazier\n    solids:\n")

	polygon := func(radius float64, count int) []point {
		points := make([]point, count)
		for i := range points {
			angle := 2 * math.Pi * float64(i) / float64(count)
			points[i] = point{radius * math.Cos(angle), radius * math.Sin(angle)}
		}

		return points
	}
	solid := func(id string, points []point, bottom, top float64, material string) {
		fmt.Fprintf(&out, "      - id: %s\n        footprint:\n", id)

		for _, p := range points {
			fmt.Fprintf(&out, "          - { x: %.6f, y: %.6f }\n", p.x, p.y)
		}

		fmt.Fprintf(
			&out,
			"        bottom: { c: %.6f }\n        top: { c: %.6f }\n        side_material: %s\n        top_material: %s\n        bottom_material: %s\n        collision: true\n",
			bottom,
			top,
			material,
			material,
			material,
		)
	}
	box := func(id string, x0, y0, x1, y1, bottom, top float64, material string) {
		solid(id, []point{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}, bottom, top, material)
	}

	solid("bowl", polygon(.42, 12), 3.5, 3.86, "z-hanging-bronze")
	solid("coals", polygon(.36, 12), 3.86, 3.90, "z-hanging-coals")

	outer, inner := polygon(.45, 12), polygon(.36, 12)
	for i := range outer {
		next := (i + 1) % len(outer)
		solid(fmt.Sprintf("rim-%02d", i), []point{outer[i], outer[next], inner[next], inner[i]}, 3.86, 3.90, "z-hanging-bronze")
	}

	box("west-strap", -.32, -.025, -.28, .025, 3.98, 4.5, "z-hanging-bronze")
	box("east-strap", .28, -.025, .32, .025, 3.98, 4.5, "z-hanging-bronze")
	box("north-strap", -.025, .28, .025, .32, 3.98, 4.5, "z-hanging-bronze")
	box("crown-crossbar", -.32, -.025, .32, .025, 4.46, 4.5, "z-hanging-bronze")
	box("crown-spoke", -.025, 0, .025, .32, 4.46, 4.5, "z-hanging-bronze")
	solid("crown", polygon(.08, 8), 4.45, 4.51, "z-hanging-bronze")
	// Alternating XZ/YZ rectangular links have genuine openings and overlap
	// neighboring links. No alpha masking or room partitions form the chain.
	for i := range chainLinks {
		bottom := chainBottom + float64(i)*(chainTop-chainBottom)/chainLinks
		height := (chainTop-chainBottom)/chainLinks + chainOverlap

		prefix := fmt.Sprintf("chain-%02d-", i)
		if i%2 == 0 {
			box(prefix+"left", -.08, -.02, -.04, .02, bottom, bottom+height, "z-hanging-iron")
			box(prefix+"right", .04, -.02, .08, .02, bottom, bottom+height, "z-hanging-iron")
			box(prefix+"bottom", -.04, -.02, .04, .02, bottom, bottom+.04, "z-hanging-iron")
			box(prefix+"top", -.04, -.02, .04, .02, bottom+height-.04, bottom+height, "z-hanging-iron")
		} else {
			box(prefix+"left", -.02, -.08, .02, -.04, bottom, bottom+height, "z-hanging-iron")
			box(prefix+"right", -.02, .04, .02, .08, bottom, bottom+height, "z-hanging-iron")
			box(prefix+"bottom", -.02, -.04, .02, .04, bottom, bottom+.04, "z-hanging-iron")
			box(prefix+"top", -.02, -.04, .02, .04, bottom+height-.04, bottom+height, "z-hanging-iron")
		}
	}

	out.WriteString(end)

	source := string(data)
	if start := strings.Index(source, begin); start >= 0 {
		finish := strings.Index(source[start:], end)
		if finish < 0 {
			panic("missing hanging-brazier end marker")
		}

		source = source[:start] + out.String() + source[start+finish+len(end):]
	} else {
		at := strings.Index(source, "  - id: roman-peristyle\n")
		if at < 0 {
			panic("missing Roman court prefab")
		}

		source = source[:at] + out.String() + "\n" + source[at:]
	}
	// Slim wall torches keep the fire and its light above all supporting metal.
	out.Reset()

	const wallBegin = "  # BEGIN generated wall-sconce prefab\n"

	const wallEnd = "  # END generated wall-sconce prefab\n"

	out.WriteString(wallBegin + "  - id: wall-sconce\n    solids:\n")
	box("backplate", -.07, -.015, .07, .015, 2.18, 2.55, "z-hanging-iron")
	box("arm", -.025, 0, .025, .32, 2.30, 2.34, "z-hanging-iron")

	pan := polygon(.095, 8)
	coal := polygon(.075, 8)
	stem := polygon(.045, 8)

	for i := range pan {
		pan[i].y += .32
		coal[i].y += .32
		stem[i].y += .32
	}

	solid("stem", stem, 2.22, 2.64, "z-hanging-bronze")
	solid("coals", coal, 2.64, 2.68, "z-hanging-coals")

	for i := range pan {
		next := (i + 1) % len(pan)
		solid(fmt.Sprintf("rim-%02d", i), []point{pan[i], pan[next], coal[next], coal[i]}, 2.62, 2.66, "z-hanging-bronze")
	}

	out.WriteString(wallEnd)
	source = replaceGenerated(source, wallBegin, wallEnd, out.String(), "  - id: roman-peristyle\n")

	// Rounded corners and one low ledge replace the stepped hexagonal mound.

	out.Reset()

	const poolBegin = "    # BEGIN generated rounded basin solids\n"

	const poolEnd = "    # END generated rounded basin solids\n"

	out.WriteString(poolBegin)

	ledge, rim, water := roundedPolygon(2.7, 2.3, 1.1), roundedPolygon(2.42, 2.02, .82), roundedPolygon(2.12, 1.72, .52)
	for i := range ledge {
		next := (i + 1) % len(ledge)
		solid(fmt.Sprintf("basin-ledge-%02d", i), []point{ledge[i], ledge[next], rim[next], rim[i]}, 0, .10, "z-court-marble")
		solid(fmt.Sprintf("basin-rim-%02d", i), []point{rim[i], rim[next], water[next], water[i]}, 0, .44, "z-court-marble")
	}

	solid("basin-bottom", water, 0, .16, "z-court-mosaic")

	startWater := out.Len()

	solid("basin-water", water, .17, .36, "z-water-boundary")

	text := out.String()
	waterText := strings.Replace(text[startWater:], "top_material: z-water-boundary", "top_material: z-court-water", 1)
	waterText = strings.Replace(waterText, "collision: true", "collision: false", 1)

	text = text[:startWater] + waterText + poolEnd
	if strings.Contains(source, poolBegin) {
		source = replaceGenerated(source, poolBegin, poolEnd, text, "")
	} else {
		start := strings.Index(source, "      - id: basin-step-")
		if start < 0 {
			panic("missing old basin solids")
		}

		finish := strings.Index(source[start:], "    connections:\n")
		if finish < 0 {
			panic("missing courtyard connections")
		}

		source = source[:start] + text + source[start+finish:]
	}

	if err := root.WriteFile(path, []byte(source), 0o600); err != nil {
		panic(err)
	}
}

func replaceGenerated(source, begin, end, text, anchor string) string {
	if start := strings.Index(source, begin); start >= 0 {
		finish := strings.Index(source[start:], end)
		if finish < 0 {
			panic("missing generated section end")
		}

		return source[:start] + text + source[start+finish+len(end):]
	}

	at := strings.Index(source, anchor)
	if anchor == "" || at < 0 {
		panic("missing generated section anchor")
	}

	return source[:at] + text + "\n" + source[at:]
}

func roundedPolygon(halfX, halfY, radius float64) []point {
	centers := []point{
		{halfX - radius, -halfY + radius},
		{halfX - radius, halfY - radius},
		{-halfX + radius, halfY - radius},
		{-halfX + radius, -halfY + radius},
	}
	points := make([]point, 0, (cornerSegments+1)*len(centers))

	for corner, center := range centers {
		for segment := range cornerSegments + 1 {
			angle := -math.Pi/2 + float64(corner)*math.Pi/2 + float64(segment)*math.Pi/quarterArcDivisor
			points = append(points, point{-8 + center.x + radius*math.Cos(angle), 4 + center.y + radius*math.Sin(angle)})
		}
	}

	return points
}

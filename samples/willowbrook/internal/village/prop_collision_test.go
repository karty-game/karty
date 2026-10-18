package village

import (
	"testing"

	"github.com/karty-game/tiled"
)

func TestAuthoredPropFootprintsBlockBodiesFromEverySide(t *testing.T) {
	original := Colliders
	t.Cleanup(func() { Colliders = original })
	for _, layer := range authoredLevel.Layers {
		for _, prop := range layer.Objects {
			if prop.Class != "cottage" && prop.Class != "tree" {
				continue
			}
			suffix := "-trunk"
			if prop.Class == "cottage" {
				suffix = "-footprint"
			}
			var body *tiled.Region
			for i := range original {
				if original[i].Name == prop.Name+suffix {
					body = &original[i]
				}
			}
			if body == nil {
				t.Fatalf("%s has no authored solid footprint", prop.Name)
			}
			// Isolate this actual Tiled shape so neighbouring cliffs/boundaries
			// cannot conceal a missing side of the prop's own footprint.
			Colliders = []tiled.Region{*body}
			anchor := Point{prop.X, prop.Y - 24*prop.Width/160}
			probes := []Point{{prop.X, prop.Y - 32*prop.Width/160}, {prop.X + 25*prop.Width/160, prop.Y - 6*prop.Width/160}}
			if prop.Class == "cottage" {
				anchor = Point{prop.X, prop.Y - 50}
				probes = []Point{{prop.X, prop.Y - 80}, {prop.X + 108, prop.Y - 30}, {prop.X - 110, prop.Y - 20}}
			}
			for _, p := range probes {
				if Walkable(p) {
					t.Errorf("%s leaves visible body/root area walkable at %+v", prop.Name, p)
				}
			}
			for _, d := range []Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				start := Point{anchor.X - d.X*200, anchor.Y - d.Y*200}
				end := Move(start, Point{X: d.X * 400, Y: d.Y * 400})
				if !Walkable(end) || (end.X-anchor.X)*d.X+(end.Y-anchor.Y)*d.Y >= 0 {
					t.Errorf("movement crossed %s from direction %+v: %+v -> %+v", prop.Name, d, start, end)
				}
			}
			// The image above the ground footprint may still occlude a player
			// walking behind its roof or canopy.
			behind := Point{prop.X, prop.Y - prop.Height*.8}
			if !Walkable(behind) {
				t.Errorf("%s blocks its whole roof/canopy instead of its solid body", prop.Name)
			}
		}
	}
}

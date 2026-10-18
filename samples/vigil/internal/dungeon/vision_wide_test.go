package dungeon

import "testing"

func TestVisionCoversWideCameraSideRegions(t *testing.T) {
	if FogRight-FogLeft > 8192 {
		t.Fatal("loading curtain exceeds the SDK vector extent")
	}
	focus := Point{Spawn.X + .8, Spawn.Y + .8}
	var v Vision
	v.Update(Spawn, 10, focus)
	hidden := GroundToScreen(Point{2, 9}, 0, 10, focus)
	if hidden.X >= 0 || hidden.Y < 0 || hidden.Y >= 720 || !masked(&v, hidden) {
		t.Fatal("wide-camera concealed floor escaped mask", hidden)
	}
	// At every allowed zoom and focus, the horizontal overscan includes the
	// entire crypt; outside it there is only the black scene background.
	for _, height := range []float64{10, 14, 22} {
		for _, from := range Outline {
			for _, corner := range Outline {
				for _, z := range []float64{0, VisionHeight} {
					p := GroundToScreen(corner, z, height, Point{from.X + .8, from.Y + .8})
					if p.X < FogLeft || p.X > FogRight {
						t.Fatal("projected scene exceeds mask domain", height, from, corner, p)
					}
				}
			}
		}
	}
}

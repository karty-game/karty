package village

import (
	"math"
	"testing"
)

func TestInteractionRequiresReachableApproachAcrossAuthoredSolids(t *testing.T) {
	// These are actual bridge posts, not an invented wall that could miss a
	// regression in the source map. Find a short, dry pair on opposite sides.
	found := false
	for _, body := range Colliders {
		if body.Name != "bridge-far-left-post" && body.Name != "bridge-far-right-post" {
			continue
		}
		for _, pair := range [][2]Point{
			{{body.X - Radius - .1, body.Y + body.Height/2}, {body.X + body.Width + Radius + .1, body.Y + body.Height/2}},
			{{body.X + body.Width/2, body.Y - RadiusY - .1}, {body.X + body.Width/2, body.Y + body.Height + RadiusY + .1}},
		} {
			a, b := pair[0], pair[1]
			if !Walkable(a) || !Walkable(b) || math.Hypot(a.X-b.X, a.Y-b.Y) > InteractionRadius() {
				continue
			}
			found = true
			if CanInteract(a, b) || CanInteract(b, a) {
				t.Fatalf("interaction passed through authored solid %q: %+v %+v", body.Name, a, b)
			}
		}
	}
	if !found {
		// With a wider player, the post plus both clearances can exceed the
		// complete interaction radius. Close but blocked points still cannot
		// interact, even if their centers pass the distance-only check.
		for _, body := range Colliders {
			if body.Name == "bridge-far-left-post" {
				a := Point{body.X - Radius - .1, body.Y + body.Height/2}
				b := Point{body.X + body.Width/2, body.Y + body.Height/2}
				if !Walkable(a) || math.Hypot(a.X-b.X, a.Y-b.Y) > InteractionRadius() {
					t.Fatal("authored post no longer provides the interaction regression approach")
				}
				if CanInteract(a, b) {
					t.Fatal("interaction accepted a target inside the authored post")
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatal("missing authored post interaction fixture")
	}
}

func TestQuestPointerTargetsIncludeRaisedFacesAndTouchPadding(t *testing.T) {
	for index := range QuestStations {
		width, height := QuestVisualSize(index)
		for _, zoom := range []float64{.8, 1, 1.6} {
			footX, footY := 640.0, 400.0
			// Top paper/chest face is well above the foot. All four corners
			// including the logical touch margin should be selectable.
			for _, p := range []Point{
				{footX - width*zoom/2 - 11, footY - height*zoom - 11},
				{footX + width*zoom/2 + 11, footY + 11},
				{footX, footY - height*zoom*.75},
			} {
				if !QuestHit(index, p.X, p.Y, footX, footY, zoom) {
					t.Fatalf("visible/touch-padded target missed for index %d at zoom %g: %+v", index, zoom, p)
				}
			}
			if QuestHit(index, footX+width*zoom/2+13, footY, footX, footY, zoom) ||
				QuestHit(index, footX, footY-height*zoom-13, footX, footY, zoom) {
				t.Fatal("selectable region extended into unrelated scenery")
			}
		}
	}
}

package source

import (
	"fmt"
	"testing"

	"github.com/karty-game/tiled"
)

func TestAnimationUsesLocalTileIDsAndExactMillisecondHolds(t *testing.T) {
	set := `{"tilewidth":16,"tileheight":24,"columns":2,"tilecount":2,"margin":2,"spacing":1,"image":"atlas.png","imagewidth":37,"imageheight":28,"tiles":[{"id":0,"animation":[{"tileid":0,"duration":240},{"tileid":1,"duration":360}]}]}`
	m, err := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"opacity":0.45,"data":[1,1]}]`, set), "maps/village.tmj")
	if err != nil {
		t.Fatal(err)
	}
	s := m.Sprites[0]
	if len(s.Frames) != 2 || s.Frames[0].Sprite.SourceX != 2 || s.Frames[1].Sprite.SourceX != 19 || s.Frames[1].Sprite.Opacity != .45 {
		t.Fatalf("frame atlas rectangle or inherited opacity lost: %+v", s)
	}
	for _, test := range []struct {
		ms    int64
		frame int
	}{{0, 0}, {239, 0}, {240, 1}, {599, 1}, {600, 0}, {1240, 0}, {-1, 1}} {
		for _, tile := range m.Sprites {
			if got := tiled.FrameAt(tile.Frames, test.ms); got != test.frame {
				t.Fatalf("at %dms: got frame %d, want %d", test.ms, got, test.frame)
			}
		}
	}
}

func TestAnimationRejectsMissingFramesAndInvalidDurations(t *testing.T) {
	for _, frame := range []string{`{"tileid":1,"duration":300}`, `{"tileid":0,"duration":0}`, `{"tileid":0,"duration":-1}`, `{"tileid":0,"duration":60001}`, `{"tileid":-1,"duration":300}`} {
		set := fmt.Sprintf(`{"tiles":[{"id":0,"image":"water.png","imagewidth":32,"imageheight":32,"animation":[%s]}]}`, frame)
		if _, err := Load(fixture(`[{"type":"tilelayer","width":2,"height":1,"data":[1,0]}]`, set), "maps/village.tmj"); err == nil {
			t.Fatalf("accepted invalid animation frame %s", frame)
		}
	}
}

package village

import (
	"fmt"
	"math"
	"testing"

	"github.com/karty-game/controllers"
	"github.com/karty-game/tiled/karty2d"
)

// The published SDK tracks at most 4096 entities. Reserve 64 for the player's
// sprites, quest objects and HUD, independently from the Tiled residency cache.
const mapSpriteCapacity = 4096 - 64

type boundedMapBackend struct {
	next karty2d.Handle
	live map[karty2d.Handle]bool
	peak int
}

func (b *boundedMapBackend) Create(karty2d.SpriteState) (karty2d.Handle, error) {
	if len(b.live) >= mapSpriteCapacity {
		return 0, fmt.Errorf("Tiled cache transiently exceeded %d sprites", mapSpriteCapacity)
	}
	if b.live == nil {
		b.live = make(map[karty2d.Handle]bool)
	}
	b.next++
	b.live[b.next] = true
	b.peak = max(b.peak, len(b.live))
	return b.next, nil
}
func (b *boundedMapBackend) Update(handle karty2d.Handle, _ karty2d.SpriteState) error {
	if !b.live[handle] {
		return fmt.Errorf("updated retired sprite %d", handle)
	}
	return nil
}
func (b *boundedMapBackend) Destroy(handle karty2d.Handle) error {
	if !b.live[handle] {
		return fmt.Errorf("retired missing sprite %d", handle)
	}
	delete(b.live, handle)
	return nil
}

func TestClickMovementPreservesSpriteCapacityDuringCameraResidencyChanges(t *testing.T) {
	// The first three clicks reproduce the published desktop crash. The fourth
	// also covers the portrait stress route, projected into 1280x720 game space.
	clicks := [][2]float64{{754.28, 209.66}, {514.65, 264.23}, {1058.13, 183.02}, {323.69, 84.83}}
	backend := &boundedMapBackend{}
	// Retain the original unchunked map as a worst-case residency fixture,
	// independently of the build processor's background optimization.
	view, err := karty2d.New(
		Level,
		backend,
		func(string) (uint32, bool) { return 1, true },
		karty2d.Options{Wind: true, MaxLiveSprites: mapSpriteCapacity},
	)
	if err != nil {
		t.Fatal(err)
	}
	options := controllers.DefaultTopDown2DOptions()
	options.Speed, options.RunSpeed = WalkSpeed(), RunSpeed()
	options.FollowOffset = controllers.Vec2{Y: Level.Properties.Number("cameraOffsetY", -110)}
	options.Zoom = Zoom()
	controller, err := controllers.NewTopDown2D(options)
	if err != nil {
		t.Fatal(err)
	}
	position, focus := Spawn, CameraFocus(Spawn)
	loaded := false
	for frame := 0; frame < 1000; frame++ {
		if frame >= 420 && frame < 420+len(clicks)*24 && (frame-420)%24 == 0 {
			click := clicks[(frame-420)/24]
			controller.SetTarget(controllers.Vec2{X: focus.X + (click[0]-640)/Zoom(), Y: focus.Y + (click[1]-360)/Zoom()})
		}
		x := math.Floor(focus.X/128) * 128
		y := math.Floor(focus.Y/128) * 128
		before := len(backend.live)
		ready, err := view.Update(
			karty2d.Bounds{MinX: x - 900/Zoom() - 256, MaxX: x + 900/Zoom() + 256, MinY: y - 360/Zoom() - 256, MaxY: y + 360/Zoom() + 256},
			int64(frame)*1000/60,
		)
		if err != nil {
			t.Fatalf("frame%d camera%+v live%d peak%d: %v", frame, focus, before, backend.peak, err)
		}
		if !loaded {
			loaded = ready
			continue
		}
		delta := controller.Step(controllers.Vec2{X: position.X, Y: position.Y})
		position = Move(position, Point{X: delta.X, Y: delta.Y})
		next := controller.Follow(controllers.Vec2{X: focus.X, Y: focus.Y}, controllers.Vec2{X: position.X, Y: position.Y}, false)
		focus = Point{X: next.X, Y: next.Y}
	}
	if !loaded || math.Hypot(position.X-Spawn.X, position.Y-Spawn.Y) < 100 || backend.peak > mapSpriteCapacity {
		t.Fatal("click route did not execute within capacity", loaded, position, backend.peak)
	}
	t.Logf("camera click route: peak%d/%d Tiled sprites, endpoint%+v", backend.peak, mapSpriteCapacity, position)
}

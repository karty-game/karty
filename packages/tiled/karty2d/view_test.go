package karty2d

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/karty-game/tiled"
)

var errBackend = errors.New("injected sprite backend failure")

type fakeBackend struct {
	next                                Handle
	live                                map[Handle]SpriteState
	created                             []SpriteState
	updates, destroys                   int
	failCreate, failUpdate, failDestroy bool
	capacity, peak                      int
}

func (b *fakeBackend) Create(s SpriteState) (Handle, error) {
	if b.failCreate || (b.capacity > 0 && len(b.live) >= b.capacity) {
		return 0, errBackend
	}
	if b.live == nil {
		b.live = make(map[Handle]SpriteState)
	}
	b.next++
	b.live[b.next] = s
	b.created = append(b.created, s)
	b.peak = max(b.peak, len(b.live))
	return b.next, nil
}
func (b *fakeBackend) Update(h Handle, s SpriteState) error {
	if b.failUpdate {
		return errBackend
	}
	if _, ok := b.live[h]; !ok {
		return errBackend
	}
	b.updates++
	b.live[h] = s
	return nil
}
func (b *fakeBackend) Destroy(h Handle) error {
	if b.failDestroy {
		return errBackend
	}
	if _, ok := b.live[h]; !ok {
		return errBackend
	}
	delete(b.live, h)
	b.destroys++
	return nil
}
func imageResolver(key string) (uint32, bool) {
	if strings.HasPrefix(key, "missing") {
		return 0, false
	}
	if strings.HasPrefix(key, "second") {
		return 2, true
	}
	return 1, true
}
func sprite(x, y float64) tiled.Sprite {
	return tiled.Sprite{Image: "first.png", X: x, Y: y, Width: 32, Height: 48, SourceWidth: 16, SourceHeight: 24, SortY: y + 48, Opacity: 1}
}
func view(t *testing.T, sprites []tiled.Sprite, b *fakeBackend, options Options) *View {
	t.Helper()
	v, err := New(&tiled.Map{Sprites: sprites}, b, imageResolver, options)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func update(t *testing.T, v *View, bounds Bounds, ms int64) bool {
	t.Helper()
	ready, err := v.Update(bounds, ms)
	if err != nil {
		t.Fatal(err)
	}
	return ready
}

func TestViewPreservesTransformsLayerOpacityAndDoesNotResendStaticSprites(t *testing.T) {
	s := sprite(100, 200)
	s.Layer = 3
	s.SortY = 248
	s.FlipX = true
	s.Diagonal = true
	s.Rotation = 30
	s.Opacity = .4
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{s}, b, Options{})
	if !update(t, v, Bounds{0, 0, 500, 500}, 0) || len(b.created) != 1 {
		t.Fatal("initial residency")
	}
	actual := b.created[0]
	sx, sy, rotation := tiled.Transform(s)
	if actual != (SpriteState{AssetID: 1, X: 116 + 8*(math.Cos(math.Pi/6)-math.Sin(math.Pi/6)), Y: 224 + 8*(math.Sin(math.Pi/6)+math.Cos(math.Pi/6)), ScaleX: sx, ScaleY: sy, Rotation: rotation, Opacity: .4, Layer: 300496}) {
		t.Fatal("lost source pose, flags, opacity or sorting", actual)
	}
	for i := range 100 {
		update(t, v, Bounds{float64(i) / 10, 0, 500, 500}, int64(i*16))
	}
	if len(b.created) != 1 || b.updates != 0 || b.destroys != 0 {
		t.Fatal("camera movement resent static sprites", len(b.created), b.updates, b.destroys)
	}
	if s.X != 100 || s.Rotation != 30 {
		t.Fatal("view mutated source sprite")
	}
	layer, err := v.LayerFor(3, 248)
	if err != nil || layer != actual.Layer {
		t.Fatal("game actor layer differs from map", layer, err)
	}
}

func TestViewRollingCursorCannotStarveWhileBoundsChangeEveryFrame(t *testing.T) {
	b := &fakeBackend{}
	sprites := []tiled.Sprite{sprite(0, 0), sprite(50, 0), sprite(100, 0), sprite(150, 0)}
	v := view(t, sprites, b, Options{ResidencyBudget: 1})
	for frame := range 4 {
		before := len(b.created)
		update(t, v, Bounds{-10 - float64(frame), -10, 300, 100}, int64(frame*16))
		if len(b.created)-before > 1 {
			t.Fatal("lifecycle budget exceeded")
		}
	}
	if len(b.created) != 4 || b.created[3].X != 166 {
		t.Fatal("later sprite starved behind a restarting cursor", b.created)
	}
	if !update(t, v, Bounds{-20, -10, 300, 100}, 100) {
		t.Fatal("settled pass not ready")
	}
}

func TestViewResidencyCapEvictsBeforeRetryingMissingVisibleSprites(t *testing.T) {
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{sprite(100, 0), sprite(0, 0)}, b, Options{MaxLiveSprites: 1})
	if !update(t, v, Bounds{-1, -1, 33, 49}, 0) {
		t.Fatal("initial pass")
	}
	if !update(t, v, Bounds{99, -1, 133, 49}, 100) || v.LiveCount() != 1 || b.destroys != 1 || len(b.created) != 2 {
		t.Fatal("residency replacement failed")
	}
	if _, err := v.Update(Bounds{-1, -1, 133, 49}, 100); !errors.Is(err, ErrLiveLimit) {
		t.Fatal("overfull visible set did not report its limit", err)
	}
}

func TestViewCullingUsesRotatedFullImageBounds(t *testing.T) {
	s := sprite(100, 100)
	s.Width = 100
	s.Height = 10
	s.Rotation = 90
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{s}, b, Options{})
	update(t, v, Bounds{149, 58, 151, 60}, 0)
	if v.LiveCount() != 1 {
		t.Fatal("rotated tall image was culled by its unrotated foot/rectangle")
	}
	update(t, v, Bounds{101, 101, 102, 102}, 0)
	if v.LiveCount() != 0 || b.destroys != 1 {
		t.Fatal("rotation-aware image bounds retained a nonintersecting image")
	}
}

func animated(x float64) tiled.Sprite {
	s := sprite(x, 0)
	second := s
	second.Image = "second.png"
	second.Width = 40
	second.X -= 4
	s.Frames = []tiled.Frame{{Sprite: s, Duration: 100}, {Sprite: second, Duration: 100}}
	return s
}

func TestViewAnimationBudgetAndSharedClockSurviveCullAndRecreation(t *testing.T) {
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{animated(0), animated(40), animated(80)}, b, Options{AnimationBudget: 1})
	bounds := Bounds{-10, -10, 200, 100}
	update(t, v, bounds, 0)
	for want := 1; want <= 3; want++ {
		update(t, v, bounds, 150)
		if b.updates != want {
			t.Fatal("animation budget or cursor failed", b.updates, want)
		}
	}
	update(t, v, bounds, 199)
	if b.updates != 3 {
		t.Fatal("unchanged authored hold resent animation state")
	}
	for _, state := range b.live {
		if state.AssetID != 2 || state.ScaleX != 2.5 {
			t.Fatal("animated image or dimensions lost", state)
		}
	}
	update(t, v, Bounds{1000, 1000, 1100, 1100}, 250)
	if v.LiveCount() != 0 {
		t.Fatal("animated sprites not retired")
	}
	update(t, v, bounds, 350)
	for _, state := range b.live {
		if state.AssetID != 2 {
			t.Fatal("recreated tile restarted its animation rather than using shared clock", state)
		}
	}
}

func TestViewBackendFailuresRemainRetryableAndCloseIsBudgeted(t *testing.T) {
	b := &fakeBackend{failCreate: true}
	v := view(t, []tiled.Sprite{animated(0), animated(40)}, b, Options{ResidencyBudget: 1})
	bounds := Bounds{-10, -10, 200, 100}
	if _, err := v.Update(bounds, 0); !errors.Is(err, errBackend) || v.LiveCount() != 0 {
		t.Fatal("failed creation published state", err)
	}
	b.failCreate = false
	update(t, v, bounds, 0)
	update(t, v, bounds, 0)
	b.failUpdate = true
	if _, err := v.Update(bounds, 150); !errors.Is(err, errBackend) {
		t.Fatal("update failure hidden", err)
	}
	b.failUpdate = false
	update(t, v, bounds, 150)
	b.failDestroy = true
	if ready, err := v.Close(); ready || !errors.Is(err, errBackend) || v.LiveCount() != 2 {
		t.Fatal("failed retirement lost handles", ready, err, v.LiveCount())
	}
	b.failDestroy = false
	if ready, err := v.Close(); ready || err != nil || v.LiveCount() != 1 {
		t.Fatal("close exceeded lifecycle budget", ready, err, v.LiveCount())
	}
	if ready, err := v.Close(); !ready || err != nil || v.LiveCount() != 0 {
		t.Fatal("close did not finish", ready, err)
	}
	if ready, err := v.Close(); !ready || err != nil {
		t.Fatal("close is not idempotent")
	}
	if _, err := v.Update(bounds, 200); !errors.Is(err, ErrClosed) {
		t.Fatal("closed view accepted updates")
	}
}

func TestViewRejectsInvalidInputsBeforeBackendCalls(t *testing.T) {
	b := &fakeBackend{}
	s := animated(0)
	s.Frames[1].Sprite.Image = "missing.png"
	if _, err := New(
		&tiled.Map{Sprites: []tiled.Sprite{s}},
		b,
		imageResolver,
		Options{},
	); !errors.Is(err, ErrInvalidMap) ||
		len(b.created) != 0 {
		t.Fatal("missing future animation asset was not prevalidated", err)
	}
	for _, options := range []Options{{ResidencyBudget: -1}, {AnimationBudget: 257}, {MaxLiveSprites: tiled.MaxSprites + 1}, {LayerStride: -1}, {SortScale: math.NaN()}} {
		if _, err := New(&tiled.Map{}, b, imageResolver, options); !errors.Is(err, ErrInvalidOptions) {
			t.Fatal("invalid options accepted", options, err)
		}
	}
	v := view(t, []tiled.Sprite{sprite(0, 0)}, b, Options{})
	for _, bounds := range []Bounds{{10, 0, 0, 10}, {math.NaN(), 0, 1, 1}, {0, 0, math.Inf(1), 1}} {
		if _, err := v.Update(bounds, 0); !errors.Is(err, ErrInvalidBounds) {
			t.Fatal("invalid world bounds accepted", err)
		}
	}
	if _, err := v.Update(Bounds{}, -1); !errors.Is(err, ErrInvalidBounds) || len(b.created) != 0 {
		t.Fatal("invalid clock reached backend", err)
	}
	if _, err := v.LayerFor(2147483647, 0); !errors.Is(err, ErrInvalidMap) {
		t.Fatal("overflowing layer accepted", err)
	}
	for _, invalid := range []tiled.Sprite{
		{Image: "first.png", X: math.NaN(), Width: 1, Height: 1, SourceWidth: 1, SourceHeight: 1, Opacity: 1},
		{Image: "first.png", Rotation: 1e308, Width: 1, Height: 1, SourceWidth: 1, SourceHeight: 1, Opacity: 1},
		{Image: "first.png", Width: 1e308, Height: 1, SourceWidth: 1, SourceHeight: 1, Opacity: 1},
	} {
		if _, err := New(&tiled.Map{Sprites: []tiled.Sprite{invalid}}, b, imageResolver, Options{}); !errors.Is(err, ErrInvalidMap) {
			t.Fatal("invalid transformed state accepted", invalid, err)
		}
	}
}

func TestViewWindIsOptInQuantizedAndKeepsTheRootFixed(t *testing.T) {
	s := sprite(100, 200)
	s.Properties = tiled.Properties{
		{Name: "windAmplitude", Value: float64(1.2)},
		{Name: "windPeriod", Value: float64(5)},
	}
	for _, enabled := range []bool{false, true} {
		b := &fakeBackend{}
		v := view(t, []tiled.Sprite{s}, b, Options{Wind: enabled})
		bounds := Bounds{0, 0, 500, 500}
		update(t, v, bounds, 0)
		update(t, v, bounds, 1)
		if b.updates != 0 {
			t.Fatal("wind changed between quantized ticks")
		}
		update(t, v, bounds, 67)
		if enabled && b.updates != 1 || !enabled && b.updates != 0 {
			t.Fatal("optional wind property was not respected", enabled, b.updates)
		}
		for _, state := range b.live {
			if math.Abs(state.X-s.Height/2*math.Sin(state.Rotation)-(s.X+s.Width/2)) > 1e-8 ||
				math.Abs(state.Y+s.Height/2*math.Cos(state.Rotation)-(s.Y+s.Height)) > 1e-8 {
				t.Fatal("wind detached its authored root", state)
			}
		}
	}
}

func TestViewWindCullingIncludesWideImageCorners(t *testing.T) {
	s := sprite(0, 0)
	s.Width, s.Height = 1000, 2
	s.Properties = tiled.Properties{
		{Name: "windAmplitude", Value: float64(2)},
		{Name: "windPeriod", Value: float64(1)},
	}
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{s}, b, Options{Wind: true})
	// The right corner moves about 17 px downward at two degrees. Padding
	// based only on the two-pixel height incorrectly culled this visible corner.
	update(t, v, Bounds{998, 16, 1001, 20}, 250)
	if v.LiveCount() != 1 {
		t.Fatal("wide wind-rotated corner was culled")
	}
	for _, state := range b.live {
		x := state.X + s.Width/2*math.Cos(state.Rotation) - s.Height/2*math.Sin(state.Rotation)
		y := state.Y + s.Width/2*math.Sin(state.Rotation) + s.Height/2*math.Cos(state.Rotation)
		if x < 998 || x > 1001 || y < 16 || y > 20 {
			t.Fatal("fixture did not put the actual image corner in the viewport", x, y)
		}
	}
}

func TestViewSteadyStateCameraUpdatesAllocateNothing(t *testing.T) {
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{sprite(0, 0)}, b, Options{})
	update(t, v, Bounds{-10, -10, 100, 100}, 0)
	step := 0
	if allocations := testing.AllocsPerRun(100, func() {
		step++
		_, err := v.Update(Bounds{-10 - float64(step%2), -10, 100, 100}, int64(step*16))
		if err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatal("camera-only world bounds changed allocated", allocations)
	}
}

func TestViewCullingUsesDiagonalShiftAndExchangedDimensions(t *testing.T) {
	s := sprite(100, 100)
	s.Width, s.Height, s.Diagonal = 40, 90, true
	b := &fakeBackend{}
	v := view(t, []tiled.Sprite{s}, b, Options{})
	// Tiled's diagonal rectangle is [100,190] x [150,190], extending
	// right of the nominal 40-wide rectangle and clear of its old top.
	update(t, v, Bounds{180, 160, 185, 165}, 0)
	if v.LiveCount() != 1 {
		t.Fatal("diagonal overhang incorrectly culled")
	}
	update(t, v, Bounds{110, 110, 115, 115}, 1)
	if v.LiveCount() != 0 {
		t.Fatal("unpainted nominal rectangle retained")
	}
}

func TestViewRetiresBeforeAdmissionWithoutTransientBackendOverflow(t *testing.T) {
	const capacity = 16
	var sprites []tiled.Sprite
	// New-region entries come first, before the old residents in the rolling scan.
	for _, x := range []float64{2000, 0} {
		for i := range capacity {
			sprites = append(sprites, sprite(x+float64(i*40), 0))
		}
	}
	backend := &fakeBackend{capacity: capacity}
	v := view(t, sprites, backend, Options{ResidencyBudget: 7, MaxLiveSprites: 2 * capacity})
	for frame := 0; frame < 8; frame++ {
		if update(t, v, Bounds{-10, -10, 700, 100}, int64(frame*16)) {
			break
		}
	}
	if v.LiveCount() != capacity {
		t.Fatal("old region was not resident", v.LiveCount())
	}
	settled := false
	for frame := 0; frame < 12; frame++ {
		before := len(backend.created) + backend.destroys
		// Changing bounds during the eviction pass must retain rolling progress.
		settled = update(t, v, Bounds{1990 - float64(frame), -10, 2700, 100}, int64(200+frame*16))
		if len(backend.created)+backend.destroys-before > 7 {
			t.Fatal("lifecycle budget exceeded")
		}
		if settled {
			break
		}
	}
	if !settled || v.LiveCount() != capacity || backend.destroys != capacity || len(backend.created) != 2*capacity ||
		backend.peak > capacity {
		t.Fatal(
			"replacement failed or transiently exceeded capacity",
			settled,
			v.LiveCount(),
			backend.destroys,
			len(backend.created),
			backend.peak,
		)
	}
}

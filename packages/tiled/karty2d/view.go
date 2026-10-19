// Package karty2d presents Tiled maps through a game-provided ordinary 2D sprite
// backend. It has no generated SDK, graphics, filesystem or host dependency.
package karty2d

import (
	"errors"
	"fmt"
	"math"

	"github.com/karty-game/tiled"
)

type Handle uint64

// SpriteState is a centered ordinary sprite. Rotation is clockwise radians;
// negative scale implements Tiled flips. AssetID is local to the game backend.
type SpriteState struct {
	AssetID                                 uint32
	X, Y, ScaleX, ScaleY, Rotation, Opacity float64
	Layer                                   int32
}

// Backend operations must leave their previous state intact when returning an
// error. Create returns a unique nonzero live handle. The game adapter owns its
// generated SDK entities and mounted-level asset references.
type Backend interface {
	Create(SpriteState) (Handle, error)
	Update(Handle, SpriteState) error
	Destroy(Handle) error
}

type Resolver func(imageKey string) (uint32, bool)
type Bounds struct{ MinX, MinY, MaxX, MaxY float64 }
type Options struct {
	ResidencyBudget, AnimationBudget, MaxLiveSprites int
	LayerBase, LayerStride                           int32
	SortScale                                        float64
	// Wind enables the optional windAmplitude/windPeriod property extension.
	Wind bool
}

var (
	ErrInvalidMap     = errors.New("invalid Tiled view map or asset")
	ErrInvalidOptions = errors.New("invalid Tiled view options")
	ErrInvalidBounds  = errors.New("invalid Tiled view bounds or clock")
	ErrClosed         = errors.New("Tiled view is closing or closed")
	ErrLiveLimit      = errors.New("visible Tiled sprites exceed residency limit")
	ErrInvalidHandle  = errors.New("sprite backend returned a zero handle")
)

type preparedFrame struct {
	state  SpriteState
	sprite tiled.Sprite
}
type entry struct {
	base              preparedFrame
	frames            []preparedFrame
	timing            []tiled.Frame
	bounds            Bounds
	last              SpriteState
	handle            Handle
	amplitude, period float64
}

type View struct {
	backend           Backend
	options           Options
	entries           []entry
	effects           []int
	bounds            Bounds
	hasBounds         bool
	cursor, remaining int
	pending           bool
	evicting          bool
	animationCursor   int
	live              int
	closing           bool
	closeCursor       int
}

// New validates and resolves every base image and animation frame before making
// any backend calls. The map and its animation timing must stay immutable for
// the view's lifetime. Zero option fields select the documented defaults.
func New(level *tiled.Map, backend Backend, resolve Resolver, options Options) (*View, error) {
	if level == nil || backend == nil || resolve == nil || len(level.Sprites) > tiled.MaxSprites {
		return nil, ErrInvalidMap
	}
	if options.ResidencyBudget == 0 {
		options.ResidencyBudget = 128
	}
	if options.AnimationBudget == 0 {
		options.AnimationBudget = 256
	}
	if options.MaxLiveSprites == 0 {
		options.MaxLiveSprites = 8192
	}
	if options.LayerStride == 0 {
		options.LayerStride = 100000
	}
	if options.SortScale == 0 {
		options.SortScale = 2
	}
	if options.ResidencyBudget < 1 || options.ResidencyBudget > 256 || options.AnimationBudget < 1 || options.AnimationBudget > 256 ||
		options.MaxLiveSprites < 1 ||
		options.MaxLiveSprites > tiled.MaxSprites ||
		options.LayerStride < 1 ||
		!finite(options.SortScale) ||
		options.SortScale <= 0 {
		return nil, ErrInvalidOptions
	}
	v := &View{backend: backend, options: options, entries: make([]entry, len(level.Sprites))}
	frameCount := 0
	for i, s := range level.Sprites {
		e := &v.entries[i]
		base, err := v.prepare(s, resolve)
		if err != nil {
			return nil, err
		}
		e.base = base
		e.bounds = frameBounds(base)
		if len(s.Frames) > tiled.MaxAnimationFrames || frameCount > tiled.MaxTotalAnimationFrames-len(s.Frames) {
			return nil, ErrInvalidMap
		}
		frameCount += len(s.Frames)
		for _, f := range s.Frames {
			if f.Duration <= 0 || f.Duration > 60000 {
				return nil, ErrInvalidMap
			}
			frame, err := v.prepare(f.Sprite, resolve)
			if err != nil {
				return nil, err
			}
			e.frames = append(e.frames, frame)
			e.bounds = e.bounds.union(frameBounds(frame))
		}
		e.timing = s.Frames
		if options.Wind {
			e.amplitude = s.Properties.Number("windAmplitude", 0)
			e.period = s.Properties.Number("windPeriod", 5)
			if !finite(e.amplitude) || !finite(e.period) || e.period <= 0 {
				e.amplitude = 0
			}
			e.amplitude = max(-2, min(2, e.amplitude))
			if e.amplitude != 0 {
				margin := math.Hypot(s.Width, s.Height) * .08
				for _, f := range s.Frames {
					margin = max(margin, math.Hypot(f.Sprite.Width, f.Sprite.Height)*.08)
				}
				e.bounds.MinX -= margin
				e.bounds.MaxX += margin
				e.bounds.MinY -= margin
				e.bounds.MaxY += margin
			}
		}
		if len(e.frames) > 0 || e.amplitude != 0 {
			v.effects = append(v.effects, i)
		}
	}
	return v, nil
}

func (v *View) prepare(s tiled.Sprite, resolve Resolver) (preparedFrame, error) {
	if s.SourceWidth <= 0 || s.SourceHeight <= 0 || s.SourceWidth > tiled.MaxImageDimension || s.SourceHeight > tiled.MaxImageDimension ||
		!finite(s.Width) ||
		!finite(s.Height) ||
		s.Width <= 0 ||
		s.Height <= 0 ||
		s.Width > tiled.MaxCoordinate ||
		s.Height > tiled.MaxCoordinate ||
		!finite(s.X) ||
		!finite(s.Y) ||
		!finite(s.Rotation) ||
		math.Abs(s.X) > tiled.MaxCoordinate ||
		math.Abs(s.Y) > tiled.MaxCoordinate ||
		math.Abs(s.Rotation) > tiled.MaxCoordinate ||
		!finite(s.Opacity) ||
		s.Opacity < 0 ||
		s.Opacity > 1 {
		return preparedFrame{}, ErrInvalidMap
	}
	id, ok := resolve(tiled.ImageKey(s))
	if !ok || id == 0 {
		return preparedFrame{}, fmt.Errorf("image %q: %w", tiled.ImageKey(s), ErrInvalidMap)
	}
	layer, err := v.LayerFor(s.Layer, s.SortY)
	if err != nil {
		return preparedFrame{}, err
	}
	sx, sy, rotation := tiled.Transform(s)
	x, y := tiled.Center(s)
	state := SpriteState{
		AssetID:  id,
		X:        x,
		Y:        y,
		ScaleX:   sx,
		ScaleY:   sy,
		Rotation: rotation,
		Opacity:  s.Opacity,
		Layer:    layer,
	}
	if !finite(state.X) || !finite(state.Y) || !finite(state.ScaleX) || !finite(state.ScaleY) || !finite(state.Rotation) {
		return preparedFrame{}, ErrInvalidMap
	}
	return preparedFrame{state: state, sprite: s}, nil
}

// LayerFor uses the same inclusive layer/Y ordering for map and game actors.
// Choose a LayerStride larger than the complete Y-sort span of a layer.
// Equal keys retain the backend's entity ordering, including after residency.
func (v *View) LayerFor(layer int, sortY float64) (int32, error) {
	if layer < 0 || !finite(sortY) {
		return 0, ErrInvalidMap
	}
	value := float64(v.options.LayerBase) + float64(layer)*float64(v.options.LayerStride) + math.Trunc(sortY*v.options.SortScale)
	if !finite(value) || value < math.MinInt32 || value > math.MaxInt32 {
		return 0, ErrInvalidMap
	}
	return int32(value), nil
}

// Update consumes absolute elapsed milliseconds, not a delta. Bounds are world
// coordinates, normally including overscan around the current Camera2D view.
// The returned ready flag means the current bounds' residency pass completed.
// Moving bounds retain the rolling cursor so later map entries cannot starve.
// At most ResidencyBudget lifecycle calls and AnimationBudget changed-state
// updates are made per invocation. Previously successful calls remain committed
// if a later backend call fails, and can be retried with the same clock/bounds.
func (v *View) Update(bounds Bounds, elapsedMS int64) (bool, error) {
	if v.closing {
		return false, ErrClosed
	}
	if !bounds.valid() || elapsedMS < 0 {
		return false, ErrInvalidBounds
	}
	if !v.hasBounds || bounds != v.bounds {
		v.bounds = bounds
		v.hasBounds = true
		v.remaining = len(v.entries)
		v.pending = false
		v.evicting = true
	}
	changes, err := v.evictInvisible()
	if err != nil {
		return false, err
	}
	visited := 0
	for !v.evicting && v.remaining > 0 && changes < v.options.ResidencyBudget && visited < len(v.entries) {
		e := &v.entries[v.cursor]
		visible := bounds.intersects(e.bounds)
		if visible && e.handle == 0 {
			if v.live < v.options.MaxLiveSprites {
				state := e.stateAt(elapsedMS)
				handle, err := v.backend.Create(state)
				if err != nil {
					return false, err
				}
				if handle == 0 {
					return false, ErrInvalidHandle
				}
				e.handle, e.last = handle, state
				v.live++
				changes++
			} else {
				v.pending = true
			}
		}
		v.cursor = (v.cursor + 1) % len(v.entries)
		v.remaining--
		visited++
	}
	if !v.evicting && v.remaining == 0 && v.pending {
		v.remaining = len(v.entries)
		v.pending = false
		if v.live == v.options.MaxLiveSprites {
			return false, ErrLiveLimit
		}
	}
	if err := v.animate(elapsedMS); err != nil {
		return false, err
	}
	return !v.evicting && v.remaining == 0, nil
}

// Finish retiring the previous view before admitting its replacement. Backends
// can have a tighter capacity than the union of two individually valid views.
func (v *View) evictInvisible() (int, error) {
	changes := 0
	for v.evicting && v.remaining > 0 && changes < v.options.ResidencyBudget {
		e := &v.entries[v.cursor]
		if e.handle != 0 && !v.bounds.intersects(e.bounds) {
			if err := v.backend.Destroy(e.handle); err != nil {
				return changes, err
			}
			e.handle = 0
			v.live--
			changes++
		}
		v.cursor = (v.cursor + 1) % len(v.entries)
		v.remaining--
	}
	if v.evicting && v.remaining == 0 {
		v.evicting = false
		v.remaining = len(v.entries)
	}
	return changes, nil
}

func (v *View) animate(milliseconds int64) error {
	changes := 0
	for visited := 0; visited < len(v.effects) && changes < v.options.AnimationBudget; visited++ {
		i := v.effects[v.animationCursor]
		e := &v.entries[i]
		if e.handle != 0 {
			state := e.stateAt(milliseconds)
			if state != e.last {
				if err := v.backend.Update(e.handle, state); err != nil {
					return err
				}
				e.last = state
				changes++
			}
		}
		v.animationCursor = (v.animationCursor + 1) % len(v.effects)
	}
	return nil
}

func (e *entry) stateAt(milliseconds int64) SpriteState {
	frame := e.base
	if i := tiled.FrameAt(e.timing, milliseconds); i >= 0 {
		frame = e.frames[i]
	}
	state := frame.state
	if e.amplitude != 0 {
		// Quantized 15 Hz breeze updates; animation timing remains exact.
		bucket := milliseconds/1000*15 + milliseconds%1000*15/1000
		state.X, state.Y, state.ScaleX, state.ScaleY, state.Rotation = WindTransform(
			frame.sprite,
			float64(bucket)/15,
			e.amplitude,
			e.period,
		)
	}
	return state
}

// Close retires at most ResidencyBudget sprites per call. It is idempotent;
// call until ready before releasing the backend's mounted textures. No future
// Update is accepted once Close has begun.
func (v *View) Close() (bool, error) {
	v.closing = true
	changes := 0
	for v.closeCursor < len(v.entries) && changes < v.options.ResidencyBudget {
		e := &v.entries[v.closeCursor]
		if e.handle != 0 {
			if err := v.backend.Destroy(e.handle); err != nil {
				return false, err
			}
			e.handle = 0
			v.live--
			changes++
		}
		v.closeCursor++
	}
	return v.closeCursor == len(v.entries), nil
}

func (v *View) LiveCount() int { return v.live }

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func (b Bounds) valid() bool {
	return finite(b.MinX) && finite(b.MinY) && finite(b.MaxX) && finite(b.MaxY) && b.MinX <= b.MaxX && b.MinY <= b.MaxY
}
func (b Bounds) intersects(other Bounds) bool {
	return b.MinX <= other.MaxX && b.MaxX >= other.MinX && b.MinY <= other.MaxY && b.MaxY >= other.MinY
}
func (b Bounds) union(other Bounds) Bounds {
	return Bounds{min(b.MinX, other.MinX), min(b.MinY, other.MinY), max(b.MaxX, other.MaxX), max(b.MaxY, other.MaxY)}
}
func frameBounds(frame preparedFrame) Bounds {
	s := frame.state
	w := float64(frame.sprite.SourceWidth) * math.Abs(s.ScaleX) / 2
	h := float64(frame.sprite.SourceHeight) * math.Abs(s.ScaleY) / 2
	x := math.Abs(math.Cos(s.Rotation))*w + math.Abs(math.Sin(s.Rotation))*h
	y := math.Abs(math.Sin(s.Rotation))*w + math.Abs(math.Cos(s.Rotation))*h
	return Bounds{s.X - x, s.Y - y, s.X + x, s.Y + y}
}

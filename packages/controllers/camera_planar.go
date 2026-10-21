package controllers

import "math"

// ComposeCamera is the optional framing replacement for Follow. It keeps input,
// movement, zoom and picking on this Planar controller. Supply the current focus
// used by ScreenToGround and accepted player position, once per 60 Hz update.
// width/height describe the projection rectangle, in the same coordinates used
// for picking, not CSS pixels. Zones are relative to that supplied rectangle.
// A fixed isometric view damps screen-right/down rather than world X/Y.
func (c *Planar) ComposeCamera(composer *CameraComposer, current, accepted Vec2, width, height float64, snap bool) (Vec2, error) {
	if composer == nil || !finite(width) || !finite(height) || width <= 0 || height <= 0 || width > 1e6 || height > 1e6 ||
		!cameraVector(current) || !cameraVector(accepted) {
		return current, ErrCameraInput
	}
	goal := Vec2{accepted.X + c.options.FollowOffset.X, accepted.Y + c.options.FollowOffset.Y}
	view := Vec2{width / c.options.Zoom, height / c.options.Zoom}
	if c.isometric {
		view = Vec2{c.options.Zoom * width / height, c.options.Zoom}
	}
	// Commit history only after both projection and the returned world focus pass.
	next := *composer
	focus, err := next.Compose(c.cameraPlane(current), c.cameraPlane(goal), view, 1.0/60, snap)
	if err != nil {
		return current, err
	}
	if c.isometric {
		sine, cosine := math.Sin(c.options.Yaw), math.Cos(c.options.Yaw)
		down := -focus.Y / math.Sin(-c.options.Pitch)
		focus = Vec2{focus.X*cosine + down*sine, -focus.X*sine + down*cosine}
	}
	if !cameraVector(focus) || math.Abs(focus.X) > 1000000 || math.Abs(focus.Y) > 1000000 {
		return current, ErrCameraInput
	}
	*composer = next
	return focus, nil
}

func (c *Planar) cameraPlane(p Vec2) Vec2 {
	if !c.isometric {
		return p
	}
	sine, cosine := math.Sin(c.options.Yaw), math.Cos(c.options.Yaw)
	return Vec2{p.X*cosine - p.Y*sine, -(p.X*sine + p.Y*cosine) * math.Sin(-c.options.Pitch)}
}

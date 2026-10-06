package main

import (
	"math"

	"example.com/world-camera/.karty/engine"
)

// markerFollower follows the accepted FPS eye. Identical feedback emits no
// command; isometric orbit changes do not move the player marker.
type markerFollower struct {
	actor    engine.WorldActorRef
	lastPose engine.Transform3D
	hasPose  bool
}

func (follower *markerFollower) Bind(actor engine.WorldActorRef) {
	*follower = markerFollower{actor: actor}
}
func (follower *markerFollower) Reset() { *follower = markerFollower{} }
func (follower *markerFollower) Follow(pose engine.Transform3D) {
	if follower.actor.ID == 0 || follower.hasPose && follower.lastPose == pose {
		return
	}
	const behind = .16
	yaw, pitch := float64(pose.Yaw), float64(pose.Pitch)
	position := pose.Position
	position.X -= behind * float32(math.Sin(yaw)*math.Cos(pitch))
	position.Y -= behind * float32(math.Cos(yaw)*math.Cos(pitch))
	position.Z -= behind * float32(math.Sin(pitch))
	follower.actor.SetTransform(engine.NewTransform3D(position))
	follower.lastPose, follower.hasPose = pose, true
}
func cameraTransform(settings engine.CameraSettings) engine.Transform3D {
	pose := engine.NewTransform3D(settings.Position)
	pose.Yaw, pose.Pitch = settings.Yaw, settings.Pitch
	return pose
}

// spinnerAnimation is the only actor behavior that advances every frame.
type spinnerAnimation struct {
	actor     engine.WorldActorRef
	lastFrame uint64
	updated   bool
}

func (animation *spinnerAnimation) Bind(actor engine.WorldActorRef) {
	*animation = spinnerAnimation{actor: actor}
}
func (animation *spinnerAnimation) Reset() { *animation = spinnerAnimation{} }
func (animation *spinnerAnimation) Update(frame uint64) {
	if animation.actor.ID == 0 || animation.updated && animation.lastFrame == frame {
		return
	}
	transform := engine.NewTransform3D(engine.NewVec3D(6, .65, .33))
	transform.Yaw = float32(frame%314) * .02
	animation.actor.SetTransform(transform)
	animation.lastFrame, animation.updated = frame, true
}

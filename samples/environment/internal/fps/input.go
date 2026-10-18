// Package fps contains the sample's adapters and world-specific movement rules.
package fps

import (
	"math"

	"github.com/karty-game/controllers"
)

type Control = controllers.Control
type Input = controllers.FirstPerson
type Motion = controllers.Motion

const (
	Forward     = controllers.Forward
	Backward    = controllers.Backward
	Left        = controllers.Left
	Right       = controllers.Right
	LookLeft    = controllers.LookLeft
	LookRight   = controllers.LookRight
	LookUp      = controllers.LookUp
	LookDown    = controllers.LookDown
	RunLeft     = controllers.RunLeft
	RunRight    = controllers.RunRight
	SwimUp      = controllers.SwimUp
	SwimDown    = controllers.SwimDown
	StickRadius = float32(96)
)

func New(width, height float32) Input {
	options := controllers.DefaultFirstPersonOptions()
	options.Width, options.Height = width, height
	options.StickRadius, options.StickInsetY = StickRadius, 140
	controller, err := controllers.NewFirstPerson(options)
	if err != nil {
		panic(err)
	}
	return *controller
}

func hypot(x, y float32) float32 { return float32(math.Hypot(float64(x), float64(y))) }

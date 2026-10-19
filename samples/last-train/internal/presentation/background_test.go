package presentation

import (
	"testing"

	"github.com/karty-game/dialogue"
)

func TestArchitecturalBackgroundStaysFixedAcrossAmbientMotionAndFraming(t *testing.T) {
	for _, scene := range []string{"platform", "carriage", "dining"} {
		t.Run(scene, func(t *testing.T) {
			motion := dialogue.NewStage("mara")
			motion.SetScene(scene)
			initial := ArchitecturalBackground(motion.Snapshot().Background)
			ambientChanged := false
			for tick := 0; tick < 960; tick++ {
				if tick == 180 {
					if err := motion.SetFraming("close"); err != nil {
						t.Fatal(err)
					}
				}
				if tick == 480 {
					motion.SetReducedMotion(true)
				}
				if tick == 720 {
					motion.SetReducedMotion(false)
				}
				motion.Step()
				raw := motion.Snapshot().Background
				ambientChanged = ambientChanged || raw != initial
				if got := ArchitecturalBackground(raw); got != initial {
					t.Fatalf("%s architecture moved at update %d: %+v", scene, tick, got)
				}
			}
			if !ambientChanged {
				t.Fatal("fixture did not exercise animated stage input")
			}
		})
	}
}

func TestArchitecturalBackgroundPreservesVisibility(t *testing.T) {
	for _, alpha := range []float32{0, .5, 1} {
		got := ArchitecturalBackground(dialogue.StageTransform{X: .6, Y: .7, Height: 1.2, Rotation: .1, Alpha: alpha})
		if got.Alpha != alpha {
			t.Fatal("fixed framing changed scene visibility", got)
		}
	}
}

package dialogue

import "testing"

func TestStageEntrancesEnsembleAndExit(t *testing.T) {
	s := NewStage("mara", "elias", "iris")
	if err := s.SetPortrait("mara"); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Actors[0].Alpha != 0 {
		t.Fatal("instant entrance")
	}
	for range 15 {
		s.Step()
	}
	mid := s.Snapshot()
	if mid.Actors[0].Alpha <= 0 || mid.Actors[0].Alpha >= 1 || mid.Actors[1].Alpha != 0 {
		t.Fatalf("invalid entrance: %+v", mid.Actors)
	}
	for range 16 {
		s.Step()
	}
	if got := s.Snapshot().Actors[0].Alpha; got != 1 {
		t.Fatalf("entrance did not settle: %v", got)
	}
	s.SetEnsemble(true)
	for range 31 {
		s.Step()
	}
	f := s.Snapshot()
	for i := range 3 {
		if f.Actors[i].Alpha != 1 {
			t.Fatalf("ensemble actor %d hidden", i)
		}
	}
	if !(f.Actors[0].X < f.Actors[1].X && f.Actors[1].X < f.Actors[2].X) {
		t.Fatal("ensemble actors at same anchor")
	}
	s.SetEnsemble(false)
	_ = s.SetPortrait("")
	for range 25 {
		s.Step()
	}
	for i := range 3 {
		if s.Snapshot().Actors[i].Alpha != 0 {
			t.Fatalf("actor %d did not leave", i)
		}
	}
}
func TestStageReducedMotionSettlesAndRemainsStill(t *testing.T) {
	s := NewStage("mara")
	_ = s.SetPortrait("mara")
	_ = s.SetFraming("close")
	_ = s.Gesture("mara", "recoil")
	s.Step()
	s.SetReducedMotion(true)
	before := s.Snapshot()
	if before.Actors[0].Alpha != 1 || before.Background.Height != 1.16 || s.ActiveGesture("mara") != "" {
		t.Fatal("motion preference failed to settle transient state")
	}
	for range 120 {
		s.Step()
	}
	after := s.Snapshot()
	before.Tick = after.Tick
	if before != after {
		t.Fatal("reduced motion transforms changed")
	}
	_ = s.Gesture("mara", "nod")
	if s.ActiveGesture("mara") != "" {
		t.Fatal("reduced motion started gesture")
	}
	s.SetReducedMotion(false)
	s.Step()
	if s.Snapshot().Background == after.Background {
		t.Fatal("motion failed to resume")
	}
}
func TestStageAuthoredFramingReactionAndAmbient(t *testing.T) {
	s := NewStage("iris")
	s.SetScene("platform")
	_ = s.SetPortrait("iris")
	s.Settle()
	wide := s.Snapshot()
	for range 60 {
		s.Step()
	}
	if moving := s.Snapshot(); wide.Rain[0] == moving.Rain[0] || moving.Rain[0].Alpha == 0 {
		t.Fatal("platform rain did not advance")
	}
	_ = s.SetFraming("close")
	for range 180 {
		s.Step()
	}
	close := s.Snapshot()
	if close.Background.Height <= wide.Background.Height || close.Actors[0].Height <= wide.Actors[0].Height {
		t.Fatal("close frame did not approach scene and actor")
	}
	_ = s.Gesture("iris", "recoil")
	for range 18 {
		s.Step()
	}
	if s.Snapshot().Actors[0].X-close.Actors[0].X < .02 {
		t.Fatal("recoil did not move actor")
	}
	for range 18 {
		s.Step()
	}
	if s.ActiveGesture("iris") != "" {
		t.Fatal("one-shot gesture failed to finish")
	}
	s.SetScene("carriage")
	s.Step()
	if s.Snapshot().Rain[0].Alpha != 0 {
		t.Fatal("rain drawn inside carriage")
	}
	for _, p := range s.Snapshot().Lights {
		if p.Alpha < 0 || p.Alpha > .1 {
			t.Fatal("unbounded lamp intensity")
		}
	}
}
func TestStageRejectsUnknownDirection(t *testing.T) {
	s := NewStage("mara", "mara", "")
	if s.Snapshot().ActorCount != 1 {
		t.Fatal("duplicate actor registration")
	}
	if s.SetPortrait("typo") == nil || s.SetFraming("typo") == nil || s.Gesture("mara", "typo") == nil || s.Gesture("typo", "nod") == nil {
		t.Fatal("unknown stage direction accepted")
	}
}
func TestStageSnapshotAndStepAllocateNothing(t *testing.T) {
	s := NewStage("mara", "elias", "iris")
	_ = s.SetPortrait("mara")
	if n := testing.AllocsPerRun(100, func() { s.Step(); _ = s.Snapshot() }); n != 0 {
		t.Fatalf("per-update allocations: %v", n)
	}
}

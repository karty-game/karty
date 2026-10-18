package presentation

import "testing"

func TestTravelCoversSceneBeforeReplacingItAndLocksInputThroughExit(t *testing.T) {
	var s State
	s.Travel("dining", "II", "The last lit lamp")
	for n := 1; n <= coverTicks+cardTicks+uncoverTicks; n++ {
		scene, resume := s.Step()
		if (scene == "dining") != (n == coverTicks) {
			t.Fatalf("scene swap at update %d: %q", n, scene)
		}
		if s.Cover != (n < coverTicks+cardTicks) {
			t.Fatalf("curtain at %d: %v", n, s.Cover)
		}
		if s.Busy != (n < coverTicks+cardTicks+uncoverTicks) || resume != (n == coverTicks+cardTicks+uncoverTicks) {
			t.Fatalf("premature resume at %d", n)
		}
	}
	if scene, resume := s.Step(); scene != "" || resume {
		t.Fatal("resumed twice")
	}
	s.Pause(18)
	for n := 0; n < 17; n++ {
		if scene, resume := s.Step(); scene != "" || resume {
			t.Fatal("pause ended early")
		}
	}
	if _, resume := s.Step(); !resume {
		t.Fatal("pause never ended")
	}
	s.Opening()
	if !s.Title || s.Busy || s.Cover {
		t.Fatal(s)
	}
	s.Begin()
	if s.Title || s.Busy {
		t.Fatal(s)
	}
}

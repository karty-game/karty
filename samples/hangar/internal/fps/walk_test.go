package fps

import (
	"math"
	"strings"
	"testing"
)

func TestStepUpAnticipatesPlayerFootprint(t *testing.T) {
	for _, test := range []struct {
		name                  string
		x, y, z, dx, dy, want float32
	}{
		{"arrival threshold", 6, 9.68, 1.7, 0, .055, 2.05},
		{"loading doorway", 19.68, 2, 1.9, .055, 0, 2.25},
		{"original control stairs", .32, 22, 1.7, -.055, 0, 2.05},
		{"west lift entrance stays level", 19.68, 13.8, 1.9, .055, 0, 1.9},
		{"east acid stairs", 46.32, 13.8, 1.9, -.055, 0, 2.25},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := StepUp(test.x, test.y, test.z, test.x+test.dx, test.y+test.dy)
			if math.Abs(float64(got-test.want)) > .0001 {
				t.Fatalf("leading footprint step: got %v want %v", got, test.want)
			}
		})
	}
}

func TestStepUpPreservesGravityAndBlocksUnsafeLifts(t *testing.T) {
	for _, test := range []struct {
		name            string
		x, y, z, dx, dy float32
	}{
		{"standing still", 19.8, 13.8, 1.9, 0, 0},
		{"airborne approach", 19.68, 13.8, 2.04, .055, 0},
		{"descending control stairs", -5.7, 22, 2.9, .11, 0},
		{"off high ledge", 27, 23, 1.9, -.11, 0},
		{"window sill exceeds step height", 31, 15.23, 5.1, 0, .055},
		{"against outer stair wall", 19.68, 12.28, 1.9, .055, -.055},
		{"outside world", 100, 100, 1.7, .055, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := StepUp(test.x, test.y, test.z, test.x+test.dx, test.y+test.dy); got != test.z {
				t.Fatalf("unsafe positional correction: %v -> %v", test.z, got)
			}
		})
	}
}

func TestStepUpNarrowTreadsAndStraddledSupport(t *testing.T) {
	// The east acid tread is narrower than the .6-metre player footprint.
	// Support remains the lower tread beneath the feet while the next tread
	// intersects the footprint. Looking only at its highest floor would fail.
	walker := Walker{active: true, floor: .4, dx: -.11, lastX: 46.07, lastY: 13.8}
	if got := walker.StepUp(45.96, 13.8, 2.25, 45.85, 13.8); math.Abs(float64(got-2.45)) > .0001 {
		t.Fatalf("narrow tread step = %v", got)
	}
}

func TestStepUpRejectsLowHeadroomAndSolidOverlap(t *testing.T) {
	// Mutate only the relevant compiled cell in this serial unit test, then
	// restore it. Source regeneration and normal route tests use authored data.
	found := false
	for index, cell := range walkCells {
		if cell.name != "acid-control-east-step-0#0" {
			continue
		}
		found = true
		saved := walkCells[index].ceiling
		walkCells[index].ceiling = 2
		got := StepUp(46.32, 13.8, 1.9, 46.265, 13.8)
		walkCells[index].ceiling = saved
		if got != 1.9 {
			t.Fatalf("headroom allowed lift: %v", got)
		}
		break
	}
	if !found {
		t.Fatal("compiled headroom fixture cell was not found")
	}
	if clearSolids(8, 24, 0, 1.9) {
		t.Fatal("reactor collision solid was ignored")
	}
}

func TestStepUpRequiresOpenAuthoredPortal(t *testing.T) {
	found := false
	for index, cell := range walkCells {
		if cell.name != "acid-control-west-access#0" {
			continue
		}
		for wallIndex, wall := range cell.walls {
			if wall.a.x != 20 || wall.b.x != 20 || wall.portal < 0 {
				continue
			}
			found = true
			walkCells[index].walls[wallIndex].portal = -1
			got := StepUp(19.68, 13.8, 1.9, 19.735, 13.8)
			walkCells[index].walls[wallIndex].portal = wall.portal
			if got != 1.9 {
				t.Fatalf("closed wall allowed lift into adjacent floor: %v", got)
			}
		}
	}
	if !found {
		t.Fatal("compiled directed portal fixture was not found")
	}
}

func TestStepUpAscendsCompleteAuthoredRoutes(t *testing.T) {
	for _, route := range []struct {
		name                              string
		x, y, z, dx, dy, distance, height float32
	}{
		{"control deck", .4, 22, 1.7, -.055, 0, 7, 2.9},

		{"east acid room", 46.5, 13.8, 1.9, -.11, 0, 8, 5.1},
	} {
		t.Run(route.name, func(t *testing.T) {
			x, y, z := route.x, route.y, route.z
			var walker Walker
			for travelled := float32(0); travelled < route.distance; travelled += max(abs32(route.dx), abs32(route.dy)) {
				z = walker.StepUp(x, y, z, x+route.dx, y+route.dy)
				x, y = x+route.dx, y+route.dy
				// Supply accepted gravity/ground poses to exercise the helper
				// across the authored graph; WASM qualifies physical rates/contacts.
				z = settleTestPose(x, y, z)
			}
			for range 30 {
				z = walker.StepUp(x, y, z, x, y)
				z = settleTestPose(x, y, z)
			}
			if math.Abs(float64(z-route.height)) > .0001 {
				t.Fatalf("complete route height: got %v want %v", z, route.height)
			}
		})
	}
}

func settleTestPose(x, y, z float32) float32 {
	z -= .01
	for index, cell := range walkCells {
		if cellContains(cell, x, y) && cell.floor <= z-playerHeight+groundTolerance {
			return max(z, visitFootprint(index, x, y, z-playerHeight).floor+playerHeight)
		}
	}
	return z
}

func TestWalkerTransferCancelsSafely(t *testing.T) {
	for _, test := range []struct {
		name            string
		x, y, z, dx, dy float32
	}{
		{"stop", 46.2, 13.8, 2.25, 0, 0},
		{"reverse", 46.2, 13.8, 2.25, .055, 0},
		{"sideways", 46.2, 13.8, 2.25, 0, -.055},
		{"lost target support", 46.6, 13.8, 2.25, -.055, 0},
		{"airborne", 46.2, 13.8, 2.3, -.055, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			walker := Walker{active: true, floor: .4, dx: -.055, lastX: test.x + .055, lastY: test.y}
			if got := walker.StepUp(test.x, test.y, test.z, test.x+test.dx, test.y+test.dy); got != test.z || walker.active {
				t.Fatalf("phase cancellation changed height or remained active: z=%v state=%+v", got, walker)
			}
		})
	}
	completed := Walker{active: true, floor: 1.2, dx: -.055, lastX: -6.345, lastY: 22}
	if got := completed.StepUp(-6.4, 22, 2.93, -6.455, 22); got != 2.93 || completed.active {
		t.Fatalf("completed transfer remained active: z=%v state=%+v", got, completed)
	}
	var walker Walker
	z := walker.StepUp(46.32, 13.8, 1.9, 46.265, 13.8)
	for range 13 {
		z = walker.StepUp(46.265, 13.8, z, 46.21, 13.8)
	}
	if walker.active {
		t.Fatal("blocked authoritative XY held phase indefinitely")
	}
	for range 30 {
		if got := walker.StepUp(46.265, 13.8, z, 46.21, 13.8); got != z || walker.active {
			t.Fatal("stalled phase rearmed without grounded rise")
		}
	}
}

func TestWalkerMaintainsOnlySupportedGapAndTrueFloorLimit(t *testing.T) {
	walker := Walker{active: true, floor: .4, dx: -.055, lastX: 46.3, lastY: 13.8}
	if got := walker.StepUp(46.2, 13.8, 2.27, 46.145, 13.8); got != 2.27 {
		t.Fatalf("holding lowered overshoot pose: %v", got)
	}
	for index, cell := range walkCells {
		if cell.name != "acid-control-east-step-1#0" {
			continue
		}
		saved := walkCells[index].floor
		walkCells[index].floor = .7 // .30 above active .4, even though lifted feet=.55.
		walker = Walker{active: true, floor: .4, dx: -.11, lastX: 46.07, lastY: 13.8}
		got := walker.StepUp(45.96, 13.8, 2.25, 45.85, 13.8)
		walkCells[index].floor = saved
		if got != 2.25 || walker.active {
			t.Fatalf("clearance enlarged actual floor step limit: z=%v state=%+v", got, walker)
		}
		return
	}
	t.Fatal("compiled tall riser fixture was not found")
}

func TestWalkerActiveTransferDoesNotAllocate(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() {
		walker := Walker{active: true, floor: .4, dx: -.055, lastX: 46.3, lastY: 13.8}
		walker.StepUp(46.2, 13.8, 2.13, 46.145, 13.8)
	}); got != 0 {
		t.Fatalf("active transfer allocations = %v", got)
	}
}

func TestWalkerSlowTouchAndTransferDeadline(t *testing.T) {
	walker := Walker{active: true, floor: .4, dx: -.00065, lastX: 46.2 + .00065, lastY: 13.8}
	walker.StepUp(46.2, 13.8, 2.13, 46.2-.00065, 13.8)
	if !walker.active || walker.stalled != 0 {
		t.Fatalf("slow touch was treated as stalled: %+v", walker)
	}
	walker = Walker{active: true, floor: .4, dx: -.055, lastX: 46.3, lastY: 13.8, age: 119}
	if got := walker.StepUp(46.2, 13.8, 2.13, 46.145, 13.8); got != 2.13 || walker.active {
		t.Fatalf("expired transfer persisted: z=%v state=%+v", got, walker)
	}
}

func TestWalkerRetainsSupportAtActualSolverEdge(t *testing.T) {
	for _, test := range []struct {
		name                   string
		x, y, z, dx, dy, floor float32
	}{

		{"east acid", 46.300029755, 13.8, 2.149447, -.11, 0, .4},
	} {
		t.Run(test.name, func(t *testing.T) {
			walker := Walker{active: true, floor: test.floor, dx: test.dx, dy: test.dy, lastX: test.x - test.dx, lastY: test.y - test.dy}
			got := walker.StepUp(test.x, test.y, test.z, test.x+test.dx, test.y+test.dy)
			want := test.floor + playerHeight + stepClearance
			if math.Abs(float64(got-want)) > .00001 || !walker.active {
				t.Fatalf("solver contact gap cancelled supported transfer: z=%v state=%+v", got, walker)
			}
		})
	}
}

func TestWalkerSettlesWithoutTeleportAndRetainsNextRiserEligibility(t *testing.T) {
	walker := Walker{active: true, floor: .2, dx: -.11, lastX: -.29, lastY: 22}
	// The entire footprint is now on the one-metre control tread, while the
	// running-speed contact gap remains above ordinary grounding tolerance.
	if got := walker.StepUp(-.4, 22, 2.04, -.51, 22); got != 2.04 || !walker.active {
		t.Fatalf("settling must retain bounded witness without Z setter: z=%v state=%+v", got, walker)
	}
	if got := walker.StepUp(-.65, 22, 2.02, -.76, 22); math.Abs(float64(got-2.25)) > .00001 || !walker.active || walker.floor != .4 {
		t.Fatalf("next real riser lost eligibility during gravity settling: z=%v state=%+v", got, walker)
	}
	// On a flat completed tread, actual gravity closes the gap and retires the
	// witness; it cannot keep an unsupported height alive after landing.
	walker = Walker{active: true, floor: .2, dx: -.11, lastX: -.29, lastY: 22}
	if got := walker.StepUp(-.4, 22, 1.92, -.51, 22); got != 1.92 || walker.active {
		t.Fatalf("grounded completed step retained witness: z=%v state=%+v", got, walker)
	}
}

func TestWalkerAnticipatesActualSpeculativeRiserContact(t *testing.T) {
	// Actual WASM stopped here: the body front is .068351 metre short of the
	// next .2 riser, inside static CCD reach .02+.055=.075. Motion plus contact
	// lookahead must find that floor before horizontal contact can stop it.
	walker := Walker{active: true, floor: 1.8, dx: -.055, lastX: 43.423351, lastY: 13.8}
	got := walker.StepUp(43.368351, 13.8, 3.648467, 43.313351, 13.8)
	if math.Abs(float64(got-3.85)) > .00001 || !walker.active || walker.floor != 2 {
		t.Fatalf("speculative contact prevented next true riser: z=%v state=%+v", got, walker)
	}
}

func TestWalkerResumesGroundedUpperCornerOnlyUphill(t *testing.T) {
	var walker Walker
	if got := walker.StepUp(.29, 22, 1.9, .235, 22); math.Abs(float64(got-2.05)) > .00001 || !walker.active {
		t.Fatalf("actual grounded corner remained blocked: z=%v state=%+v", got, walker)
	}
	walker.Reset()
	if got := walker.StepUp(.29, 22, 1.9, .345, 22); got != 1.9 || walker.active {
		t.Fatalf("descending corner received lift: z=%v state=%+v", got, walker)
	}
	// A dynamic obstacle that prevents progress cannot repeatedly restart the
	// same corner transfer after gravity settles it. Input zero explicitly arms
	// a retry; the unchanged physical box still owns obstacle/crate collision.
	walker.StepUp(.29, 22, 1.9, .235, 22)
	for range 13 {
		walker.StepUp(.29, 22, 2.05, .235, 22)
	}
	if !walker.blocked {
		t.Fatal("stalled target was not suppressed")
	}
	for range 30 {
		if got := walker.StepUp(.29, 22, 1.9, .235, 22); got != 1.9 || walker.active {
			t.Fatal("stalled grounded corner rearmed without input reset")
		}
	}
	if got := walker.StepUp(.29, 22, 1.7, .235, 22); got != 1.7 || walker.active {
		t.Fatal("falling onto lower ground retried the failed ordinary step")
	}
	walker.StepUp(.29, 22, 1.9, .29, 22)
	if got := walker.StepUp(.29, 22, 1.9, .235, 22); math.Abs(float64(got-2.05)) > .00001 || !walker.active {
		t.Fatalf("input reset did not permit supported corner retry: z=%v state=%+v", got, walker)
	}
}

func TestWalkerBlockedInitialLiftDoesNotRepeatAfterLanding(t *testing.T) {
	var walker Walker
	walker.StepUp(.34, 22, 1.7, .285, 22)
	// The move was blocked before reaching the virtual contact footprint.
	walker.StepUp(.34, 22, 2.05, .285, 22)
	if !walker.blocked || walker.active {
		t.Fatalf("unreached initial target was not suppressed: %+v", walker)
	}
	if got := walker.StepUp(.34, 22, 1.7, .285, 22); got != 1.7 || walker.active {
		t.Fatal("blocked initial lift restarted after gravity returned to lower floor")
	}
}

func TestWalkerCornerRearmRetainsActualHeightLimit(t *testing.T) {
	for index, cell := range walkCells {
		if cell.name != "step-0#0" {
			continue
		}
		saved := walkCells[index].floor
		walkCells[index].floor = .3
		var walker Walker
		got := walker.StepUp(.29, 22, 2, .235, 22)
		walkCells[index].floor = saved
		if got != 2 || walker.active {
			t.Fatalf("corner rearm climbed actual .30 floor rise: z=%v state=%+v", got, walker)
		}
		return
	}
	t.Fatal("compiled corner floor fixture was not found")
}

func TestWalkerActualCrossingReleasesSuppressedTarget(t *testing.T) {
	walker := Walker{blocked: true, blockedFloor: .2}
	if got := walker.StepUp(-.4, 22, 1.9, -.455, 22); got != 1.9 || walker.blocked {
		t.Fatalf("actual full-foot crossing did not retire failed target: z=%v state=%+v", got, walker)
	}
	if got := walker.StepUp(-.7, 22, 1.9, -.755, 22); math.Abs(float64(got-2.25)) > .00001 || !walker.active {
		t.Fatalf("following real riser remained suppressed: z=%v state=%+v", got, walker)
	}
}

func abs32(value float32) float32 { return max(value, -value) }

func TestStepUpDoesNotAllocate(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() { StepUp(19.68, 13.8, 1.9, 19.735, 13.8) }); got != 0 {
		t.Fatalf("per-frame allocations = %v", got)
	}
}

// The physical host owns slope ascent/descent. Cartridge step support must
// recognize the authored planes without converting them into artificial risers.
func TestPoolRampsPreserveHostVerticalMotion(t *testing.T) {
	for _, route := range []struct{ x, y, dx, dy float32 }{
		{40.5, 22, .11, 0}, {40.5, 22, -.11, 0},
		{23, 26.5, 0, .11}, {23, 26.5, 0, -.11},
	} {
		for _, cell := range walkCells {
			if !cellContains(cell, route.x, route.y) {
				continue
			}
			z := cell.floorAt(route.x, route.y) + playerHeight + .2
			var walker Walker
			if got := walker.StepUp(route.x, route.y, z, route.x+route.dx, route.y+route.dy); got != z || walker.active {
				t.Fatalf("slope received a positional step: %v -> %v", z, got)
			}
		}
	}
}

func TestPoolRampPlanesMeetBasinAndLandings(t *testing.T) {
	for _, route := range []struct {
		name                         string
		x0, y0, x1, y1, top, ceiling float32
	}{
		{"trench-stair-", 40, 22, 46, 22, .2, 3.2},
		{"west-trench-stair-", 23, 26, 23, 32, 0, 3.1},
	} {
		count := 0
		for _, cell := range walkCells {
			if !strings.HasPrefix(cell.name, route.name) {
				continue
			}
			count++
			if abs32(cell.floorAt(route.x0, route.y0)+3.5) > .00001 || abs32(cell.floorAt(route.x1, route.y1)-route.top) > .00001 ||
				abs32(cell.ceilingAt(route.x1, route.y1)-route.ceiling) > .00001 {
				t.Fatalf("discontinuous ramp plane: %+v", cell)
			}
			for _, wall := range cell.walls {
				if cell.ceilingAt(wall.a.x, wall.a.y)-cell.floorAt(wall.a.x, wall.a.y) < playerHeight+.3 {
					t.Fatalf("ramp lacks player headroom: %s", cell.name)
				}
			}
		}
		if count != 6 {
			t.Fatalf("%s: %d strips", route.name, count)
		}
	}
}

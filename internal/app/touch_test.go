package app

import "testing"

// A finger comes down on a menu row, lighting it; the row runs only when the
// finger lifts, and that tap still lands although nothing is lit by then.
func TestMenuRowTapRunsOnLift(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1}
	r := menuRows[2]
	x, y := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	g.updateOptionsMenu(mouseState{x: x, y: y, down: true, pressed: true})
	if g.mode != modeOptions || g.optHover != 2 {
		t.Fatalf("touch down: mode %d hover %d, want the row lit and no run",
			g.mode, g.optHover)
	}
	g.updateOptionsMenu(mouseState{
		x: x, y: y, clicked: true, released: true, lifted: true,
	})
	if g.mode != modeLoad {
		t.Errorf("mode = %d, want the tap to open the load screen", g.mode)
	}
}

// A lifted finger leaves no row lit.
func TestLiftedFingerLightsNoRow(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1, started: true}
	r := menuRows[1]
	g.updateOptionsMenu(mouseState{x: r.Min.X + 4, y: r.Min.Y + 4, lifted: true})
	if g.optHover != -1 {
		t.Errorf("optHover = %d, want none", g.optHover)
	}
}

// A slot-screen button goes in under the finger and runs on the lift, which
// is also the tap's click.
func TestSlotButtonTapRunsOnLift(t *testing.T) {
	g := slotScreen(modeSave)
	x, y := cancelButton.Min.X+4, cancelButton.Min.Y+4
	g.updateSlotScreen(mouseState{x: x, y: y, down: true, pressed: true})
	if g.btnDown != 1 {
		t.Fatalf("btnDown = %d, want the cancel button held", g.btnDown)
	}
	g.updateSlotScreen(mouseState{
		x: x, y: y, clicked: true, released: true, lifted: true,
	})
	if g.mode != modeOptions {
		t.Errorf("mode = %d, want the cancel to run", g.mode)
	}
}

// A slider follows a finger from touch-down to lift.
func TestSliderFollowsAFinger(t *testing.T) {
	g := &Game{
		mode: modeOptions, optHover: -1, optDrag: -1, audio: &fakeAudio{},
	}
	tr := sliderTracks[2]
	y := (tr.Min.Y + tr.Max.Y) / 2
	g.updateOptionsMenu(mouseState{
		x: tr.Min.X, y: y, down: true, pressed: true,
	})
	if g.optDrag != 2 || g.speed != 0 {
		t.Fatalf("grab: drag %d speed %.2f, want slider 2 at 0",
			g.optDrag, g.speed)
	}
	g.updateOptionsMenu(mouseState{x: tr.Max.X, y: y, pressed: true})
	if g.speed != 1 {
		t.Errorf("speed = %.2f, want the finger's end of the track, 1", g.speed)
	}
	g.updateOptionsMenu(mouseState{
		x: tr.Max.X, y: y, clicked: true, released: true, lifted: true,
	})
	if g.optDrag != -1 {
		t.Error("lifting the finger must let the slider go")
	}
}

// A finger drag carries the view against its step and parks the camera
// there, clamped to the scene.
func TestDragCameraParksTheView(t *testing.T) {
	g := camGame(t, 200)
	g.camTarget = 50 // a walk's aim in flight is dropped
	g.dragCamera(-30)
	if g.camXf != 230 || g.camTarget != 230 {
		t.Errorf("view %.0f target %.0f, want both at 230",
			g.camXf, g.camTarget)
	}
	g.dragCamera(500)
	if g.camX != 0 || g.camTarget != 0 {
		t.Errorf("camX %d target %.0f, want the left end", g.camX, g.camTarget)
	}
}

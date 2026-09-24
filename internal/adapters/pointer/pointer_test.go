package pointer

import "testing"

const tick = 1.0 / 60

// fingerAt is a sample with the followed finger down at (x, y).
func fingerAt(x, y int) Sample { return Sample{Finger: true, FingerX: x, FingerY: y} }

// run steps the tracker through samples and reports the clicks it saw.
func run(t *Tracker, samples ...Sample) (clicks int) {
	for _, s := range samples {
		t.Step(s, tick)
		if t.Clicked() {
			clicks++
		}
	}
	return clicks
}

// A tap clicks on release, where the finger was, and not before.
func TestTapClicksOnRelease(t *testing.T) {
	var tr Tracker
	tr.Step(fingerAt(100, 50), tick)
	if !tr.Down() || tr.Clicked() || !tr.Hover() || !tr.Touch() {
		t.Fatalf("touch down: down=%v clicked=%v hover=%v touch=%v",
			tr.Down(), tr.Clicked(), tr.Hover(), tr.Touch())
	}
	tr.Step(fingerAt(103, 52), tick) // within the slop
	tr.Step(Sample{}, tick)
	if !tr.Clicked() || !tr.Up() {
		t.Fatal("lifting the finger must click")
	}
	if x, y := tr.Pos(); x != 103 || y != 52 {
		t.Errorf("Pos = (%d,%d), want where the finger lifted (103,52)", x, y)
	}
	if tr.Hover() {
		t.Error("a lifted finger hovers nowhere")
	}
}

// A drag the caller claims is no tap; the first step carries the whole way
// from the touch point, so the scroll does not lag the finger.
func TestClaimedDragIsNoTap(t *testing.T) {
	var tr Tracker
	tr.Step(fingerAt(100, 50), tick)
	tr.Step(fingerAt(115, 50), tick)
	if dx, dy := tr.TakeDrag(); dx != 15 || dy != 0 {
		t.Errorf("first drag step = (%d,%d), want (15,0)", dx, dy)
	}
	tr.Step(fingerAt(110, 50), tick)
	if dx, _ := tr.TakeDrag(); dx != -5 {
		t.Errorf("next drag step = %d, want -5", dx)
	}
	if clicks := run(&tr, Sample{}); clicks != 0 {
		t.Error("a claimed drag must not click on release")
	}
}

// A drag nobody claims still ends in a click where the finger lifted: a
// minigame carries the piece under the finger and drops it there.
func TestUnclaimedDragClicks(t *testing.T) {
	var tr Tracker
	clicks := run(&tr, fingerAt(100, 50), fingerAt(160, 90), Sample{})
	if clicks != 1 {
		t.Fatalf("clicks = %d, want 1", clicks)
	}
	if x, y := tr.Pos(); x != 160 || y != 90 {
		t.Errorf("Pos = (%d,%d), want (160,90)", x, y)
	}
}

// Held still for LongPress the finger right-clicks once; a caller that takes
// it claims the gesture, and one that does not leaves the tap to the release.
func TestLongPressRightClicks(t *testing.T) {
	for _, take := range []bool{true, false} {
		var tr Tracker
		rights := 0
		tr.Step(fingerAt(100, 50), tick)
		for i := 0; i < 60; i++ {
			tr.Step(fingerAt(101, 50), tick)
			if take && tr.TakeRightClick() {
				rights++
			}
		}
		if take && rights != 1 {
			t.Errorf("right clicks = %d, want exactly 1", rights)
		}
		clicks := run(&tr, Sample{})
		if want := map[bool]int{true: 0, false: 1}[take]; clicks != want {
			t.Errorf("take=%v: clicks on release = %d, want %d",
				take, clicks, want)
		}
	}
}

// A drag never turns into a long press.
func TestDragIsNoLongPress(t *testing.T) {
	var tr Tracker
	tr.Step(fingerAt(100, 50), tick)
	tr.Step(fingerAt(140, 50), tick)
	for i := 0; i < 60; i++ {
		tr.Step(fingerAt(140, 50), tick)
		if tr.TakeRightClick() {
			t.Fatal("a dragging finger must not right-click")
		}
	}
}

// The mouse clicks on press, as the original does, and hovers always.
func TestMouseClicksOnPress(t *testing.T) {
	var tr Tracker
	tr.Step(Sample{MouseX: 10, MouseY: 10}, tick)
	tr.Step(Sample{MouseX: 20, MouseY: 30}, tick)
	if !tr.Mouse() || !tr.Hover() || tr.Touch() {
		t.Fatalf("mouse=%v hover=%v touch=%v, want a live mouse",
			tr.Mouse(), tr.Hover(), tr.Touch())
	}
	tr.Step(Sample{MouseX: 20, MouseY: 30, Left: true}, tick)
	if !tr.Clicked() || !tr.Down() || !tr.Pressed() {
		t.Error("the left button going down must click")
	}
	tr.Step(Sample{MouseX: 20, MouseY: 30}, tick)
	if tr.Clicked() || !tr.Up() {
		t.Error("the release is no second click")
	}
	tr.Step(Sample{MouseX: 20, MouseY: 30, Right: true}, tick)
	if !tr.TakeRightClick() {
		t.Error("the right button must right-click")
	}
}

// A cursor that never moved is not a mouse yet; a finger takes over from the
// mouse, and moving the mouse takes it back.
func TestFingerAndMouseTakeTurns(t *testing.T) {
	var tr Tracker
	tr.Step(Sample{}, tick)
	tr.Step(Sample{}, tick)
	if tr.Mouse() {
		t.Error("an untouched cursor must not count as a mouse")
	}
	tr.Step(Sample{MouseX: 5, MouseY: 5}, tick)
	if !tr.Mouse() {
		t.Fatal("a moved cursor is a mouse")
	}
	run(&tr, Sample{
		MouseX: 5, MouseY: 5, Finger: true, FingerX: 300,
		FingerY: 200,
	}, Sample{MouseX: 5, MouseY: 5})
	if tr.Mouse() || !tr.Touch() {
		t.Error("after a tap the finger is the pointer")
	}
	if x, _ := tr.Pos(); x != 300 {
		t.Errorf("Pos x = %d, want the finger's 300, not the parked cursor", x)
	}
	tr.Step(Sample{MouseX: 6, MouseY: 5}, tick)
	if !tr.Mouse() || tr.Touch() {
		t.Error("moving the mouse makes it the pointer again")
	}
}

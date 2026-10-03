package crt

import (
	"math"
	"testing"

	"github.com/shpaker/kinescope"
)

const tick = 1.0 / 60

// newTube is a tube switched on and warmed up: the picture whole.
func newTube(t *testing.T, o Options) *Tube {
	t.Helper()
	tube, err := New(true, o)
	if err != nil {
		t.Fatal(err)
	}
	for range 40 {
		tube.Update(tick, false)
	}
	return tube
}

// The tube's shaders compile: Kage is checked on the CPU, no window needed.
func TestShadersCompile(t *testing.T) {
	newTube(t, Defaults)
}

// The pointer bends the way the picture does: the middle stays put, the
// bend is symmetric, a corner shows frame pixels further out, a flat tube in
// a window bends nothing, and full screen's case makes the picture smaller.
func TestWarp(t *testing.T) {
	const w, h = 640, 480
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	tube := newTube(t, Defaults)
	for _, full := range []bool{false, true} {
		if x, y := tube.Warp(320, 240, w, h, full); !near(x, 320) ||
			!near(y, 240) {
			t.Errorf("full %v: the middle moved to %v,%v", full, x, y)
		}
	}
	x1, y1 := tube.Warp(100, 60, w, h, false)
	x2, y2 := tube.Warp(540, 420, w, h, false)
	if !near(x1+x2, w) || !near(y1+y2, h) {
		t.Errorf("not symmetric: %v,%v and %v,%v", x1, y1, x2, y2)
	}
	if x1 >= 100 || y1 >= 60 {
		t.Errorf("a corner's pointer did not bow out: %v,%v", x1, y1)
	}
	flat := newTube(t, Options{Case: true})
	if x, y := flat.Warp(100, 60, w, h, false); !near(x, 100) || !near(y, 60) {
		t.Errorf("a flat tube bent 100,60 to %v,%v", x, y)
	}
	if x, _ := flat.Warp(0, 240, w, h, true); x >= 0 {
		t.Errorf("the case left the frame's edge on the picture: %v", x)
	}
	bare := newTube(t, Options{})
	if x, _ := bare.Warp(0, 240, w, h, true); !near(x, 0) {
		t.Errorf("without its case the picture still shrank: %v", x)
	}
}

// The glitches come as often as the options say, and never when they are
// set to 0.
func TestGlitchSchedule(t *testing.T) {
	every := setup(Defaults, 1).Schedules["glitches"]
	if every.Mean != 90 || every.Spread != 30 || len(every.Episodes) != 4 {
		t.Errorf("glitches %+v", every)
	}
	if _, ok := setup(Options{}, 1).Schedules["glitches"]; ok {
		t.Error("a glitch schedule with glitches off")
	}
}

// The dials reach the tube; the case rests until full screen.
func TestDials(t *testing.T) {
	tube := newTube(t, Defaults)
	tube.Update(tick, false)
	x := tube.tv.Value(kinescope.CurvatureX)
	if math.Abs(float64(x)-0.045) > 1e-6 {
		t.Errorf("curvature x %v", x)
	}
	if m := tube.tv.Value(kinescope.CabinetMargin); m != 0 {
		t.Errorf("the case shows in a window: %v", m)
	}
	tube.setFull(true)
	if m := tube.tv.Value(kinescope.CabinetMargin); m != caseMargin {
		t.Errorf("the case in full screen %v, want %v", m, caseMargin)
	}
}

// The ripple of a scene change shivers and dies out within half a second;
// a tube set against ripples keeps still.
func TestRipple(t *testing.T) {
	tube := newTube(t, Defaults)
	tube.Ripple()
	tube.Update(tick, false)
	if s := tube.tv.Value(kinescope.TearStrength); s <= 0 {
		t.Errorf("no tear in a ripple: %v", s)
	}
	for range 30 {
		tube.Update(tick, false)
	}
	if s := tube.tv.Value(kinescope.TearStrength); s != 0 {
		t.Errorf("still rippling: %v", s)
	}

	still := newTube(t, Options{Ripple: false})
	still.Ripple()
	still.Update(tick, false)
	if s := still.tv.Value(kinescope.TearStrength); s != 0 {
		t.Errorf("rippled with ripples off: %v", s)
	}
}

// Power off folds the picture away and the tube goes dark.
func TestPowerOff(t *testing.T) {
	tube := newTube(t, Defaults)
	tube.PowerOff()
	for range 60 {
		tube.Update(tick, false)
	}
	if !tube.Dark() {
		t.Error("never went dark")
	}
}

// A nil tube is a switched-off one.
func TestNilTubeIsOff(t *testing.T) {
	var tube *Tube
	tube.Update(tick, false)
	tube.Ripple()
	tube.PowerOff()
	tube.Toggle()
	if tube.On() || tube.Dark() {
		t.Error("a nil tube claims to be on or dark")
	}
}

package crt

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

const tick = 1.0 / 60

// Both passes compile: Kage is checked on the CPU, no window needed.
func TestShadersCompile(t *testing.T) {
	for name, src := range map[string][]byte{
		"crt.kage": crtKage, "glow.kage": glowKage,
	} {
		if _, err := ebiten.NewShader(src); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The pointer bends the way the picture does: the middle stays put, the
// bend is symmetric, a corner shows frame pixels further out, a flat tube in
// a window bends nothing, and full screen's case makes the picture smaller.
func TestWarp(t *testing.T) {
	const w, h = 640, 480
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	tube := &Tube{opts: Defaults}
	for _, withCase := range []bool{false, true} {
		if x, y := tube.Warp(320, 240, w, h, withCase); !near(x, 320) ||
			!near(y, 240) {
			t.Errorf("case %v: the middle moved to %v,%v", withCase, x, y)
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
	flat := &Tube{opts: Options{Case: true}}
	if x, y := flat.Warp(100, 60, w, h, false); !near(x, 100) || !near(y, 60) {
		t.Errorf("a flat tube bent 100,60 to %v,%v", x, y)
	}
	if x, _ := flat.Warp(0, 240, w, h, true); x >= 0 {
		t.Errorf("the case left the frame's edge on the picture: %v", x)
	}
	bare := &Tube{}
	if x, _ := bare.Warp(0, 240, w, h, true); !near(x, 0) {
		t.Errorf("without its case the picture still shrank: %v", x)
	}
}

// A random glitch comes a minute or two after the last one began, with the
// default 90 seconds between them, and never when they are set to 0.
func TestGlitchesKeepQuietAMinuteOrTwo(t *testing.T) {
	c := newClock(rand.New(rand.NewPCG(1, 2)), 90)
	starts, last := 0, 0.0
	for range 60 * 60 * 10 {
		was := c.g
		c.tick(tick, false)
		if was != calm || c.g == calm {
			continue
		}
		if gap := c.now - last; gap < 60 || gap > 120+tick {
			t.Errorf("glitch %d came after %.1f s", starts, gap)
		}
		starts, last = starts+1, c.now
	}
	if starts < 5 || starts > 10 {
		t.Errorf("%d glitches in ten minutes", starts)
	}
	never := newClock(rand.New(rand.NewPCG(1, 2)), 0)
	for range 60 * 60 * 10 {
		if never.tick(tick, false); never.g != calm {
			t.Fatal("a glitch with glitches off")
		}
	}
}

// With a mouse button down a glitch that falls due waits for the release.
func TestHeldMouseDefersAGlitch(t *testing.T) {
	c := newClock(rand.New(rand.NewPCG(3, 4)), 90)
	for c.now < c.next+5 {
		c.tick(tick, true)
	}
	if c.g != calm {
		t.Fatalf("glitch %d started during a drag", c.g)
	}
	c.tick(tick, false)
	if c.g == calm {
		t.Error("no glitch after the release")
	}
}

// The ripple of a scene change shivers and dies out within half a second.
func TestRippleDiesOut(t *testing.T) {
	c := newClock(rand.New(rand.NewPCG(5, 6)), 90)
	c.start(ripple)
	c.tick(tick, false)
	if s := c.shape(); s[0] <= 0 || s[2] <= 0 {
		t.Errorf("ripple shape %v", s)
	}
	for range 30 {
		c.tick(tick, false)
	}
	if c.g != calm || c.shape() != [4]float32{} {
		t.Errorf("still rippling: %d %v", c.g, c.shape())
	}
}

// Power on shows a dot, a line and the picture; power off folds it back
// and the tube goes dark.
func TestPower(t *testing.T) {
	c := newClock(rand.New(rand.NewPCG(7, 8)), 90)
	if w, h, flash := c.power(); w >= 0.1 || h >= 0.1 || flash <= 0 {
		t.Errorf("cold tube: %v %v %v", w, h, flash)
	}
	for c.warm < warmUp {
		c.tick(tick, false)
	}
	if w, h, flash := c.power(); w != 1 || h != 1 || flash != 0 {
		t.Errorf("warm tube: %v %v %v", w, h, flash)
	}
	c.dying = true
	for !c.dark() {
		if c.cold > coolDown+tick {
			t.Fatal("never went dark")
		}
		c.tick(tick, false)
	}
	if w, h, flash := c.power(); w != 0 || h != 0 || flash != 0 {
		t.Errorf("dead tube: %v %v %v", w, h, flash)
	}
}

// A tube set against ripples keeps still on a change of scene.
func TestRippleCanBeOff(t *testing.T) {
	tube := &Tube{opts: Options{Ripple: false}}
	tube.Ripple()
	if tube.g != calm {
		t.Error("rippled with ripples off")
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

package app

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
)

// testCycle builds a cycle whose movie has n frames and whose script carries the
// given frames. Animation.OK only counts frames, so nil images are enough and no
// graphics context is needed — creating real ones would need a running game.
func testCycle(n int, frames ...*types.Frame) *walkCycle {
	return &walkCycle{
		anim: &adapters.Animation{
			Frames: make([]*ebiten.Image, n),
			BBox:   make([][2]int, n),
		},
		frames: frames,
	}
}

// step is one script frame with a delay and optional event keywords.
func step(delay int, kws ...string) *types.Frame {
	f := &types.Frame{Delay: delay}
	for _, kw := range kws {
		f.Events = append(f.Events, types.Command{Kw: kw})
	}
	return f
}

// running returns a walker parked on a cycle, as enter would leave it.
func running(c *walkCycle) *walker {
	return &walker{prefix: "RG", chr: "ROBY", cur: c}
}

func kwsOf(cs []types.Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Kw
	}
	return out
}

func sameKws(t *testing.T, got []types.Command, want ...string) {
	t.Helper()
	k := kwsOf(got)
	if len(k) != len(want) {
		t.Fatalf("events = %v, want %v", k, want)
	}
	for i := range want {
		if k[i] != want[i] {
			t.Fatalf("events = %v, want %v", k, want)
		}
	}
}

// A cycle key is the resource base name the movie and the script share, and an
// index off the end has no cycle at all — the chain is finished.
func TestWalkerCycleKey(t *testing.T) {
	w := &walker{prefix: "RG", cycles: []string{"56", "66", "65"}}
	if got := w.cycleKey(1); got != "RG_66" {
		t.Errorf("cycleKey(1) = %q, want RG_66", got)
	}
	if got := w.cycleKey(-1); got != "" {
		t.Errorf("cycleKey(-1) = %q, want empty", got)
	}
	if got := w.cycleKey(3); got != "" {
		t.Errorf("cycleKey(3) = %q, want empty", got)
	}
	f := &walker{prefix: "FG", cycles: []string{"52"}}
	if got := f.cycleKey(0); got != "FG_52" {
		t.Errorf("cycleKey(0) = %q, want FG_52", got)
	}
}

// Negative delays mark ambient frames in the scripts, but their magnitude is
// still the duration, so a -457 frame must last as long as a 457 one. A missing
// or zero delay would stall the walk forever, hence the fallback.
func TestFrameDelayUsesMagnitudeAndFallsBack(t *testing.T) {
	const fallback = 0.09
	cases := []struct {
		name  string
		w     *walker
		frame int
		want  float64
	}{
		{"plain delay", running(testCycle(2, step(100), step(120))), 0, 0.1},
		{"next frame", running(testCycle(2, step(100), step(120))), 1, 0.12},
		{"negative is ambient", running(testCycle(1, step(-457))), 0, 0.457},
		{"zero falls back", running(testCycle(1, step(0))), 0, fallback},
		{"no cycle falls back", &walker{}, 0, fallback},
		{"no script frames", running(testCycle(3)), 0, fallback},
		// The movie may outlast its script; the last authored delay then holds
		// instead of indexing past the end.
		{"past the script", running(testCycle(4, step(100))), 3, 0.1},
	}
	for _, c := range cases {
		c.w.frame = c.frame
		if got := c.w.frameDelay(); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: frameDelay = %v, want %v", c.name, got, c.want)
		}
	}
}

// A walk cycle never loops: its frames march the figure across one cell, so
// replaying them would walk the hero a second cell with no cell shift behind it.
// The last frame still gets its full delay on screen, and only the tick after
// that reports done so the caller can enter the next cycle.
func TestWalkerAdvanceNeverLoops(t *testing.T) {
	w := running(testCycle(3, step(100), step(100), step(100)))
	w.advance(0.25) // 2 steps: frames 1 and 2
	if w.frame != 2 || w.done {
		t.Fatalf("frame = %d done = %v, want the last frame still playing",
			w.frame, w.done)
	}
	if evs := w.advance(0.1); evs != nil || w.frame != 2 || !w.done {
		t.Fatalf("frame = %d done = %v evs = %v, want done on frame 2",
			w.frame, w.done, evs)
	}
	if w.walking() {
		t.Fatal("a played-out cycle must stop reporting itself as walking")
	}
	// Past the end the walker is inert: the chain, not the cycle, moves on.
	if evs := w.advance(1); evs != nil || w.frame != 2 {
		t.Fatalf("frame = %d evs = %v after done", w.frame, evs)
	}
}

// advance returns the events of every frame it *entered*. Frame 0's events are
// the caller's business (enter hands them over when the cycle starts), so
// advance must not fire them again — the authored "Shift char,X,+1" lives there
// and a repeat would move the hero an extra cell per cycle.
func TestWalkerAdvanceFiresEnteredFramesOnce(t *testing.T) {
	// 125ms and its exact halves, so no rounding decides a frame boundary.
	const delay, half = 125, 0.0625
	w := running(testCycle(3,
		step(delay, "shift", "sound"),
		step(delay, "sound"),
		step(delay, "set"),
	))
	if evs := w.advance(half); evs != nil { // not a full frame yet
		t.Fatalf("partial tick fired %v", kwsOf(evs))
	}
	sameKws(t, w.advance(half), "sound") // enters frame 1
	if evs := w.advance(half); evs != nil {
		t.Fatalf("second partial tick refired %v", kwsOf(evs))
	}
	sameKws(t, w.advance(half), "set") // enters frame 2
	if evs := w.advance(2 * half); evs != nil {
		t.Fatalf("the tick that ends the cycle fired %v", kwsOf(evs))
	}
	if !w.done {
		t.Fatal("the cycle must be done after its last frame's delay")
	}
}

// One long tick may cross several frames; each one's events still come back
// exactly once and in order, so a dropped frame cannot swallow a cell shift.
func TestWalkerAdvanceCollectsEveryCrossedFrame(t *testing.T) {
	w := running(testCycle(4,
		step(100, "shift"),
		step(100, "sound"),
		step(100, "set"),
		step(100, "text"),
	))
	sameKws(t, w.advance(0.35), "sound", "set", "text")
	if w.frame != 3 || w.done {
		t.Fatalf("frame = %d done = %v, want the last frame playing",
			w.frame, w.done)
	}
}

// A movie with more frames than its script simply has silent frames: the walk
// must keep playing them instead of panicking on the missing script entry.
func TestWalkerAdvanceToleratesShortScript(t *testing.T) {
	w := running(testCycle(4, step(100), step(100, "sound")))
	sameKws(t, w.advance(0.11), "sound")
	if evs := w.advance(0.21); evs != nil {
		t.Fatalf("frames past the script fired %v", kwsOf(evs))
	}
	if w.frame != 3 {
		t.Fatalf("frame = %d, want 3", w.frame)
	}
}

// walking() gates every update: no cycle means the chain is spent, and a movie
// with no frames is a failed load that must not be advanced either.
func TestWalkerWalkingNeedsAPlayableCycle(t *testing.T) {
	if (&walker{}).walking() {
		t.Error("a walker with no cycle must not be walking")
	}
	w := running(testCycle(2, step(100)))
	if !w.walking() {
		t.Error("a fresh cycle must be walking")
	}
	w.done = true
	if w.walking() {
		t.Error("a played-out cycle must not be walking")
	}
	empty := running(&walkCycle{anim: &adapters.Animation{}})
	if evs := empty.advance(1); evs != nil || empty.frame != 0 {
		t.Fatalf("frameless movie advanced: frame = %d evs = %v",
			empty.frame, evs)
	}
	if empty.anim().OK() {
		t.Error("a frameless animation must not report OK")
	}
	if (&walker{}).anim() != nil {
		t.Error("an idle walker has no animation to draw")
	}
}

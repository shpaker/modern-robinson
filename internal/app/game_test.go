package app

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// The two edge exits are objects like any other, so their names are what the
// container is searched for. "gorght" is spelled without the i in the shipped
// resources; correcting it here would silently break every right-hand exit.
func TestExitKeyNamesTheEdgeObjects(t *testing.T) {
	if got := exitKey(true); got != "goleft" {
		t.Errorf("exitKey(true) = %q, want goleft", got)
	}
	if got := exitKey(false); got != "gorght" {
		t.Errorf("exitKey(false) = %q, want gorght", got)
	}
}

// INT0..INT4 are cutscene bridges driven by their own objects, with no
// controllable character on stage. Only those five: a scene wrongly taken for an
// intro would hide the hero for good, and a missed one would draw him standing
// in the middle of the opening cutscene.
func TestIsIntroScene(t *testing.T) {
	cases := map[string]bool{
		"INT0":   true,
		"INT4":   true,
		"int2":   true, // the scripts name scenes in mixed case
		"INT5":   false,
		"INT9":   false,
		"INT":    false, // too short to carry an index
		"INTRO":  false, // four letters is the whole rule
		"INT10":  false,
		"SCENA0": false,
		"":       false,
	}
	for name, want := range cases {
		if got := isIntroScene(name); got != want {
			t.Errorf("isIntroScene(%q) = %v, want %v", name, got, want)
		}
	}
}

// Hit testing is half-open: the max edge belongs to the next zone, so two
// hotspots that share a border cannot both claim the same pixel.
func TestPointInIsHalfOpen(t *testing.T) {
	r := image.Rect(10, 20, 30, 40)
	cases := []struct {
		x, y int
		want bool
	}{
		{10, 20, true},  // the min corner is inside
		{29, 39, true},  //
		{30, 39, false}, // the max edge is outside
		{29, 40, false},
		{9, 20, false},
		{10, 19, false},
		{20, 30, true},
	}
	for _, c := range cases {
		if got := pointIn(r, c.x, c.y); got != c.want {
			t.Errorf(
				"pointIn(%v,%d,%d) = %v, want %v",
				r,
				c.x,
				c.y,
				got,
				c.want,
			)
		}
	}
}

// A bar box is x0,y0,x1,y1 — corners, not a size — and half-open like the
// hotspots, so adjacent buttons do not overlap on their shared edge.
func TestInBoxIsHalfOpenCorners(t *testing.T) {
	b := [4]int{4, 8, 12, 16}
	cases := []struct {
		x, y int
		want bool
	}{
		{4, 8, true},
		{11, 15, true},
		{12, 15, false},
		{11, 16, false},
		{3, 8, false},
		{4, 7, false},
	}
	for _, c := range cases {
		if got := inBox(b, c.x, c.y); got != c.want {
			t.Errorf("inBox(%v,%d,%d) = %v, want %v", b, c.x, c.y, got, c.want)
		}
	}
}

func TestClampAndMinMaxInt(t *testing.T) {
	cases := []struct {
		v, lo, hi, want int
	}{
		{5, 0, 10, 5},
		{-1, 0, 10, 0},
		{11, 0, 10, 10},
		{0, 0, 0, 0},
		{-5, -10, -1, -5},
	}
	for _, c := range cases {
		if got := clampInt(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clampInt(%d,%d,%d) = %d, want %d",
				c.v, c.lo, c.hi, got, c.want)
		}
	}
	if got := minInt(3, -3); got != -3 {
		t.Errorf("minInt(3,-3) = %d, want -3", got)
	}
	if got := minInt(2, 2); got != 2 {
		t.Errorf("minInt(2,2) = %d, want 2", got)
	}
	if got := maxInt(3, -3); got != 3 {
		t.Errorf("maxInt(3,-3) = %d, want 3", got)
	}
}

// Script arguments arrive as raw text with whatever spacing the author left, and
// a non-numeric one must read as 0 rather than break the frame: the interpreter
// has no way to report an error mid-cutscene.
func TestAtoiArg(t *testing.T) {
	cases := map[string]int{
		"7":     7,
		" 7 ":   7,
		"-1":    -1,
		"+2":    2,
		"":      0,
		"Roby":  0,
		"1.5":   0, // not an int: the scripts never mean a fraction here
		"  -12": -12,
	}
	for in, want := range cases {
		if got := atoiArg(in); got != want {
			t.Errorf("atoiArg(%q) = %d, want %d", in, got, want)
		}
	}
}

// HideChar/ShowChar with no argument means the hero; Friday's variants are
// routed elsewhere, so naming him here must not match.
func TestRobyTarget(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"Roby"}, true},
		{[]string{"roby"}, true},
		{[]string{"Frid"}, false},
		{[]string{"Frid", "1"}, false},
	}
	for _, c := range cases {
		if got := robyTarget(c.args); got != c.want {
			t.Errorf("robyTarget(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

// The minigames pick pieces by their pixels, but never outside the bitmap: the
// bounds test has to come first, or a click next to a piece reads a neighbour's
// memory. Reading an actual pixel needs a running graphics context, so only the
// guards are exercised here.
func TestOpaqueAtGuardsBounds(t *testing.T) {
	if opaqueAt(nil, 0, 0) {
		t.Error("a missing sprite must never answer a click")
	}
	img := ebiten.NewImage(4, 4)
	outside := [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 4}, {4, 4}}
	for _, p := range outside {
		if opaqueAt(img, p[0], p[1]) {
			t.Errorf("opaqueAt(%d,%d) is outside a 4x4 sprite", p[0], p[1])
		}
	}
}

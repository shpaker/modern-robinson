package app

import "testing"

// HEAD.MV is a 3x3 table of standing poses, not an animation, so the pose is
// chosen by where the cursor sits relative to the hero's eyes. Screen y grows
// downwards, which is why a cursor *below* the eyes selects the *down* row (0)
// and one above selects the up row (2) — getting that backwards makes the hero
// look away from the mouse.
func TestHeadFrameFollowsCursor(t *testing.T) {
	const far = 200 // well outside both dead zones
	cases := []struct {
		name   string
		dx, dy int
		want   int
	}{
		{"down left", -far, far, 0},
		{"down", 0, far, 1},
		{"down right", far, far, 2},
		{"left", -far, 0, 3},
		{"straight ahead", 0, 0, 4},
		{"right", far, 0, 5},
		{"up left", -far, -far, 6},
		{"up", 0, -far, 7},
		{"up right", far, -far, 8},
	}
	for _, c := range cases {
		if got := headFrame(c.dx, c.dy); got != c.want {
			t.Errorf(
				"headFrame(%d,%d) = %d, want %d (%s)",
				c.dx,
				c.dy,
				got,
				c.want,
				c.name,
			)
		}
	}
}

// The frame index is row*3+col, so every cell of the table is reachable and no
// two offsets collide: lookAtCursor indexes the nine movie frames with it.
func TestHeadFrameCoversTheNineSlots(t *testing.T) {
	const far = 200
	seen := map[int]bool{}
	for _, dy := range []int{far, 0, -far} {
		for _, dx := range []int{-far, 0, far} {
			f := headFrame(dx, dy)
			if f < 0 || f >= headCols*headCols {
				t.Fatalf("headFrame(%d,%d) = %d, outside the table", dx, dy, f)
			}
			if seen[f] {
				t.Fatalf("frame %d reached twice, at (%d,%d)", f, dx, dy)
			}
			seen[f] = true
		}
	}
	if len(seen) != headCols*headCols {
		t.Fatalf("reached %d poses, want %d", len(seen), headCols*headCols)
	}
}

// The dead zones keep the head still for small cursor movement, and their edge
// belongs to the straight-ahead pose: the comparisons are strict, so exactly
// headDeadX/headDeadY away still looks forward and one pixel further turns.
func TestHeadFrameDeadZoneEdges(t *testing.T) {
	const mid = 4 // row 1, col 1: straight ahead
	cases := []struct {
		dx, dy int
		want   int
	}{
		{-headDeadX, 0, mid},      // on the edge: still straight
		{-headDeadX - 1, 0, 3},    // one past: turns left
		{headDeadX, 0, mid},       //
		{headDeadX + 1, 0, 5},     // turns right
		{0, -headDeadY, mid},      //
		{0, -headDeadY - 1, 7},    // turns up
		{0, headDeadY, mid},       //
		{0, headDeadY + 1, 1},     // turns down
		{headDeadX, headDeadY, 4}, // both edges at once
	}
	for _, c := range cases {
		if got := headFrame(c.dx, c.dy); got != c.want {
			t.Errorf(
				"headFrame(%d,%d) = %d, want %d",
				c.dx,
				c.dy,
				got,
				c.want,
			)
		}
	}
}

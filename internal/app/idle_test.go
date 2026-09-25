package app

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// robyLookBox is the LookBox both ROBY.CHR and FRID.CHR carry.
var robyLookBox = [4]int{-10, -10, 50, 50}

// HEAD.MV is a 3x3 table of standing poses, not an animation, so the pose is
// chosen by where the cursor sits relative to the hero's cell anchor. Screen y
// grows downwards, which is why a cursor *below* the box selects the *down* row
// (0) and one above selects the up row (2) — getting that backwards makes the
// hero look away from the mouse.
func TestHeadFrameFollowsCursor(t *testing.T) {
	const far = 200 // well outside the LookBox
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
		if got := headFrame(robyLookBox, c.dx, c.dy); got != c.want {
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
			f := headFrame(robyLookBox, dx, dy)
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

// The LookBox edge belongs to the turned pose: the engine turns the head on
// <= left/top and >= right/bottom (0x40dee2..0x40dfd8), so the edge itself
// already looks aside and one pixel inside still looks forward.
func TestHeadFrameLookBoxEdges(t *testing.T) {
	cases := []struct {
		dx, dy int
		want   int
	}{
		{-10, 0, 3}, // on the left edge: turns left
		{-9, 0, 4},  // one inside: straight
		{49, 0, 4},  //
		{50, 0, 5},  // turns right
		{0, -10, 7}, // turns up
		{0, -9, 4},  //
		{0, 49, 4},  //
		{0, 50, 1},  // turns down
		{-9, -9, 4}, // one inside both edges at once
		{49, 49, 4}, //
		{-10, -10, 6},
		{50, -10, 8},
		{-10, 50, 0},
		{50, 50, 2},
	}
	for _, c := range cases {
		if got := headFrame(robyLookBox, c.dx, c.dy); got != c.want {
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

// An empty LookBox never picks the straight pose: left is checked before right
// and up before down, as in the engine, so the anchor itself looks up-left.
func TestHeadFrameEmptyLookBox(t *testing.T) {
	var box [4]int
	if got := headFrame(box, 0, 0); got != 6 {
		t.Errorf("headFrame(empty, 0,0) = %d, want 6", got)
	}
	if got := headFrame(box, 1, 1); got != 2 {
		t.Errorf("headFrame(empty, 1,1) = %d, want 2", got)
	}
}

// The LookBox hangs off the hero's cell anchor on screen: the grid anchor minus
// the camera scroll. On SCENA0's grid cell 3,4 anchors at 493,195, which a
// camera at 384 puts at 109,195.
func TestLookAtCursorAroundTheCellAnchor(t *testing.T) {
	sc := &types.Scene{
		LeftTopGrid: [2]int{15, 150},
		GridSize:    [2]int{144, 36},
		GridShift:   [2]int{46, -99},
		GridLength:  [2]int{8, 5},
	}
	g := &Game{
		grid:    use_cases.NewGrid(sc, 1024, 400),
		idle:    &adapters.Animation{Frames: make([]*ebiten.Image, 9)},
		lookBox: robyLookBox,
		camX:    384,
	}
	g.placeRoby([2]int{3, 4})
	cases := []struct {
		mx, my int
		want   int
	}{
		{99, 215, 3},
		{100, 215, 4},
		{158, 215, 4},
		{159, 215, 5},
		{129, 185, 7},
		{129, 186, 4},
		{129, 244, 4},
		{129, 245, 1},
		{99, 185, 6},
		{159, 245, 2},
	}
	for _, c := range cases {
		g.lookAtCursor(c.mx, c.my)
		if g.frameI != c.want {
			t.Errorf(
				"cursor %d,%d: frame %d, want %d",
				c.mx,
				c.my,
				g.frameI,
				c.want,
			)
		}
	}

	// A standing loop that is not the nine-pose table keeps its frame.
	g.idle = &adapters.Animation{Frames: make([]*ebiten.Image, 1)}
	g.frameI = 0
	g.lookAtCursor(99, 185)
	if g.frameI != 0 {
		t.Errorf("one-frame loop: frame %d, want 0", g.frameI)
	}
}

// A script of the hero's own holds his head (+0x238) even while its movie
// waits for Friday's walk and he is drawn from the pose table; Friday's script
// leaves his head to the cursor.
func TestHeadHoldsDuringHisOwnScript(t *testing.T) {
	g := &Game{
		gs:       types.NewGameState(),
		mode:     modePlay,
		audio:    &fakeAudio{},
		idle:     &adapters.Animation{Frames: make([]*ebiten.Image, 9)},
		lookBox:  robyLookBox,
		fridPath: [][2]int{{1, 1}}, // her walk keeps either movie paused
	}

	g.act = &actionPlay{started: true, wait: waitFrid}
	g.frameI = -1 // not a pose: only a held head keeps it
	_ = g.Update()
	if g.frameI != -1 {
		t.Errorf("his own script: head turned to %d, want held", g.frameI)
	}

	g.act = &actionPlay{frid: true, started: true, wait: waitFrid}
	_ = g.Update()
	if g.frameI == -1 {
		t.Error("Friday's script held the hero's head")
	}
}

// loadCharacter takes the LookBox from ROBY.CHR along with the standing loop.
func TestLoadCharacterReadsTheLookBox(t *testing.T) {
	g := &Game{
		res:    repositories.NewResources(testutil.GameRoot(t)),
		parser: repositories.SceneParser{},
	}
	g.loadCharacter()
	if g.lookBox != robyLookBox {
		t.Errorf("lookBox = %v, want %v", g.lookBox, robyLookBox)
	}
	if !g.idle.OK() {
		t.Fatal("no standing loop")
	}
	if n := len(g.idle.Frames); n != headCols*headCols {
		t.Fatalf("standing loop has %d frames, want %d", n, headCols*headCols)
	}
}

package app

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// fridGame puts Friday on stage with the nine-pose table, standing at SCENA0's
// cell 2,3 anchor (349,159), so her LookBox spans 339..399 by 149..209.
func fridGame() *Game {
	g := &Game{
		gs:       types.NewGameState(),
		fridIdle: &adapters.Animation{Frames: make([]*ebiten.Image, 9)},
		fridBox:  robyLookBox,
		fridPos:  [2]float64{349, 159},
	}
	g.gs.SetVar("FridIs", 1)
	return g
}

// Standing Friday picks her pose by the cursor around the box: a column per
// side, a row per height, as for the hero. There is no timer: the same cursor
// keeps the same pose.
func TestFridHeadFollowsCursor(t *testing.T) {
	g := fridGame()
	cases := []struct {
		mx, my int
		want   int
	}{
		{300, 100, 6},
		{370, 100, 7},
		{450, 100, 8},
		{300, 180, 3},
		{370, 180, 4},
		{450, 180, 5},
		{300, 260, 0},
		{370, 260, 1},
		{450, 260, 2},
	}
	for _, c := range cases {
		for range 3 {
			g.fridLook(c.mx, c.my)
			if g.fridFrame != c.want {
				t.Errorf(
					"cursor %d,%d: frame %d, want %d",
					c.mx,
					c.my,
					g.fridFrame,
					c.want,
				)
				break
			}
		}
	}
}

// The box edge already turns her head, one pixel inside it does not.
func TestFridHeadBoxEdges(t *testing.T) {
	g := fridGame()
	cases := []struct {
		mx, my int
		want   int
	}{
		{339, 180, 3}, // on the left edge: turns left
		{340, 180, 4},
		{398, 180, 4},
		{399, 180, 5}, // turns right
		{370, 149, 7}, // turns up
		{370, 150, 4},
		{370, 208, 4},
		{370, 209, 1}, // turns down
	}
	for _, c := range cases {
		g.fridLook(c.mx, c.my)
		if g.fridFrame != c.want {
			t.Errorf(
				"cursor %d,%d: frame %d, want %d",
				c.mx,
				c.my,
				g.fridFrame,
				c.want,
			)
		}
	}
}

// The box hangs off her anchor on screen, so the camera scroll moves it along.
func TestFridHeadScrolls(t *testing.T) {
	g := fridGame()
	g.camX = 200
	g.fridPos = [2]float64{549, 159}
	g.fridLook(370, 180)
	if g.fridFrame != 4 {
		t.Errorf("inside the scrolled box: frame %d, want 4", g.fridFrame)
	}
	g.fridLook(300, 180)
	if g.fridFrame != 3 {
		t.Errorf("left of the scrolled box: frame %d, want 3", g.fridFrame)
	}
}

// Walking, her own script, being off stage or a standing movie that is not the
// pose table hold her pose; the hero's script does not.
func TestFridHeadGates(t *testing.T) {
	const held, turned = 4, 6 // the cursor at 300,100 looks up-left
	cases := []struct {
		name string
		set  func(g *Game)
		want int
	}{
		{"walking", func(g *Game) { g.fridPath = [][2]int{{3, 3}} }, held},
		{
			"her own script",
			func(g *Game) { g.act = &actionPlay{frid: true} },
			held,
		},
		{"hidden", func(g *Game) { g.fridHidden = true }, held},
		{"not in the game", func(g *Game) { g.gs.SetVar("FridIs", 0) }, held},
		{
			"one-frame loop",
			func(g *Game) {
				g.fridIdle = &adapters.Animation{
					Frames: make([]*ebiten.Image, 1),
				}
			},
			held,
		},
		{"hero's script", func(g *Game) { g.act = &actionPlay{} }, turned},
	}
	for _, c := range cases {
		g := fridGame()
		g.fridFrame = held
		c.set(g)
		g.fridLook(300, 100)
		if g.fridFrame != c.want {
			t.Errorf("%s: frame %d, want %d", c.name, g.fridFrame, c.want)
		}
	}
}

// Update aims her head every tick in place of the old 114 ms roll: a cursor
// that does not move leaves the pose as it is.
func TestFridHeadStaysWithTheCursor(t *testing.T) {
	g := fridGame()
	g.mode = modePlay
	g.audio = &fakeAudio{}
	mx, my := ebiten.CursorPosition()
	want := headFrame(robyLookBox, mx-349, my-159)
	for i := range 30 {
		_ = g.Update()
		if g.fridFrame != want {
			t.Fatalf("tick %d: frame %d, want %d", i, g.fridFrame, want)
		}
	}
}

// FRHEAD is a pose table like HEAD: nine frames of one Delay and no events,
// drawn from the Shift its script names.
func TestFridHeadIsPoseTable(t *testing.T) {
	res := repositories.NewResources(testutil.GameRoot(t))
	c := res.SceneContainer("FRID")
	if c == nil {
		t.Fatal("FRID container not found")
	}
	raw, err := c.ExtractName("FRHEAD.FS")
	if err != nil {
		t.Fatal(err)
	}
	fs := repositories.SceneParser{}.ParseFrameScript(string(raw))
	if !strings.EqualFold(fs.MovieName, fridMovie) {
		t.Errorf("movie %q, want %q", fs.MovieName, fridMovie)
	}
	if len(fs.Frames) != headCols*headCols {
		t.Fatalf("%d frames, want %d", len(fs.Frames), headCols*headCols)
	}
	for i, fr := range fs.Frames {
		if fr.Delay != 114 || len(fr.Events) > 0 {
			t.Errorf(
				"frame %d: delay %d, %d events",
				i,
				fr.Delay,
				len(fr.Events),
			)
		}
	}
	if ngb, _ := res.MovieFrames(fridMovie); len(ngb) != headCols*headCols {
		t.Errorf(
			"%s has %d frames, want %d",
			fridMovie,
			len(ngb),
			headCols*headCols,
		)
	}
	if s := res.MovieShift(fridMovie); s != [2]int{99, 89} || s != fs.Shift {
		t.Errorf("shift %v, script %v, want [99 89]", s, fs.Shift)
	}
}

// Entering a scene reads her LookBox from FRID.CHR, and it hangs off her cell
// anchor like the hero's.
func TestFridLookBoxFromChar(t *testing.T) {
	g := saveGame(t, memSaves{})
	g.loadScene("SCENA0", &[2]int{2, 3}, "", "")
	if g.fridBox != robyLookBox {
		t.Fatalf("fridBox = %v, want %v", g.fridBox, robyLookBox)
	}
	g.gs.SetVar("FridIs", 1)
	g.fridHidden = false
	g.placeFrid([2]int{3, 2})
	x, y := g.grid.ToScreen(3, 2)
	ax := x - g.camX
	cases := []struct {
		dx, dy int
		want   int
	}{
		{-40, -40, 6},
		{20, 20, 4},
		{100, 100, 2},
	}
	for _, c := range cases {
		g.fridLook(ax+c.dx, y+c.dy)
		if g.fridFrame != c.want {
			t.Errorf(
				"anchor%+d%+d: frame %d, want %d",
				c.dx,
				c.dy,
				g.fridFrame,
				c.want,
			)
		}
	}
}

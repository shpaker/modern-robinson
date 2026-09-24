package app

import (
	"testing"
)

// camGame is SCENA0's 1024-wide slice with the camera parked at x: 384 pixels
// of scroll to play with.
func camGame(t *testing.T, x float64) *Game {
	t.Helper()
	g, _ := clickGame(t, poolScript)
	g.sc = scena0()
	g.w, g.h = g.sc.Size[0], g.sc.Size[1]
	g.gs.UI["mouse"] = true
	g.camXf, g.camTarget, g.camX = x, x, int(x)
	return g
}

// One engine frame's worth of time, so a step is the authored pixel count.
const engineFrame = 1 / enginePace

// The cursor on an edge of the scene pushes the view 5 px an engine frame, its
// target along with it, and stops short of the scene's ends (0x418d04).
func TestEdgeScrollPushesTheView(t *testing.T) {
	cases := []struct {
		name   string
		from   float64
		mx, my int
		mouse  bool
		want   float64
	}{
		{"right edge", 100, 639, 200, true, 105},
		{"left edge", 100, 0, 200, true, 95},
		{"inner band", 100, 10, 200, true, 95},
		{"mid screen", 100, 320, 200, true, 100},
		{"over the bar", 100, 639, 450, true, 100},
		{"mouse off", 100, 639, 200, false, 100},
		{"past the right end", 382, 639, 200, true, 382},
		{"past the left end", 3, 0, 200, true, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := camGame(t, c.from)
			g.gs.UI["mouse"] = c.mouse
			g.edgeScroll(c.mx, c.my, engineFrame)
			if g.camXf != c.want || g.camTarget != c.want {
				t.Errorf("view %.0f target %.0f, want both %.0f",
					g.camXf, g.camTarget, c.want)
			}
		})
	}
}

// A click on the floor aims the camera at the point clicked, not at the hero,
// and the scene's ends clamp the aim (WalkTo 0x40f14e).
func TestClickAimsTheCameraAtThePoint(t *testing.T) {
	g := camGame(t, 0)
	wx, wy := g.grid.Corner(3, 0)
	g.click(wx+2, wy+2)
	if want := float64(wx + 2 - ViewW/2); g.camTarget != want {
		t.Errorf("target %.0f, want the click centred at %.0f",
			g.camTarget, want)
	}

	g = camGame(t, 300)
	wx, wy = g.grid.Corner(6, 0)
	g.click(wx+2-g.camX, wy+2)
	if g.camTarget != 384 {
		t.Errorf("target %.0f, want the scene's right end 384", g.camTarget)
	}
}

// An Aproach aims the camera at its goal cell's corner.
func TestAproachAimsTheCameraAtTheCell(t *testing.T) {
	const src = "MovieName Rohanpoo.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 142;\nAproach Roby,4,1;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {4, 1}}, "Roby")
	g.sc = scena0()
	g.w, g.h = g.sc.Size[0], g.sc.Size[1]
	g.cell = [2]int{1, 0}
	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0)
	x, _ := g.grid.Corner(4, 1)
	if want := float64(x - ViewW/2); g.camTarget != want {
		t.Errorf("target %.0f, want the goal corner centred at %.0f",
			g.camTarget, want)
	}
}

// The camera eases to its target and stays there while the hero is elsewhere.
func TestCameraDoesNotFollowTheHero(t *testing.T) {
	g := camGame(t, 0)
	g.camTarget = 200
	g.pos = [2]float64{900, 200}
	for i := 0; i < 600; i++ {
		g.followCamera(engineFrame)
	}
	if g.camX != 200 {
		t.Errorf("camX = %d, want the target 200 whatever the hero does",
			g.camX)
	}
}

// ShiftScreen pans whole cells from where the view stands now (0x415690).
func TestShiftScreenPansFromTheView(t *testing.T) {
	g := camGame(t, 100)
	g.shiftScreen([]string{"1", "0"})
	if g.camTarget != 244 {
		t.Errorf("target %.0f, want one cell on: 244", g.camTarget)
	}
	g.camXf = 244
	g.shiftScreen([]string{"-1", "0"})
	if g.camTarget != 100 {
		t.Errorf("target %.0f, want one cell back: 100", g.camTarget)
	}
	g.shiftScreen([]string{"-1", "0"})
	if g.camTarget != 100 {
		t.Errorf("target %.0f, want the move from the view, not the target",
			g.camTarget)
	}
}

// GoScene's cell opens the view with the scene's left edge on its corner,
// clamped to the scene (0x41b03d).
func TestShowCellOpensOnTheCorner(t *testing.T) {
	g := camGame(t, 0)
	g.showCell(1, 2)
	if x, _ := g.grid.Corner(1, 2); g.camX != x {
		t.Errorf("camX = %d, want the corner %d", g.camX, x)
	}
	g.showCell(6, 2)
	if g.camX != 384 || g.camTarget != 384 {
		t.Errorf("camX %d target %.0f, want the right end 384",
			g.camX, g.camTarget)
	}
}

// Set moves the hero, not the view.
func TestSetLeavesTheView(t *testing.T) {
	g := camGame(t, 120)
	g.setCharCoord([]string{"Roby", "X", "6"})
	if g.camX != 120 || g.camTarget != 120 {
		t.Errorf("camX %d target %.0f, want the view left at 120",
			g.camX, g.camTarget)
	}
}

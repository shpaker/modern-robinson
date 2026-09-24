package app

import (
	"image"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// clickGame is a playable slice of Game: SCENA0's grid, one scripted object
// ("pool" at (5,1), so a click can start an action) and a routing recorder.
func clickGame(t *testing.T, script string) (*Game, *recGrid) {
	t.Helper()
	g, rec := actGame(
		scena0(), script, map[string][2]int{"pool": {5, 1}}, "Roby",
	)
	g.audio = &fakeAudio{}
	g.res = actRes{}
	g.cell = [2]int{1, 0}
	// One hotspot over the pool's cell corner, so a click can reach it.
	cx, cy := g.grid.Corner(5, 1)
	g.hotspots = []hotspot{{
		key:  "pool",
		rect: image.Rect(cx, cy, cx+20, cy+20),
		ob:   &types.SceneObject{Name: "pool", Text: 1},
	}}
	return g, rec
}

const poolScript = "MovieName Rohanpoo.mv;\nTotalFrames 2;\n" +
	"Frame 0,1;\nDelay 142;\nFrame 1,1;\nDelay 142;\nEnd;"

// SetMouse OFF makes the scene deaf: no object action starts and no walk is
// routed — the engine swallows the click before the hit test (0x402e86).
func TestSetMouseOffDeafensTheScene(t *testing.T) {
	g, rec := clickGame(t, poolScript)
	g.gs.UI["mouse"] = false

	hx, hy := g.grid.Corner(5, 1)
	g.click(hx+2, hy+2) // on the pool's hotspot
	if g.act != nil {
		t.Error("an action started through a switched-off mouse")
	}
	wx, wy := g.grid.Corner(2, 0)
	g.click(wx+2, wy+2) // on a walkable cell
	if rec.paths != 0 {
		t.Errorf("routed %d walks, want none while the mouse is off", rec.paths)
	}

	g.gs.UI["mouse"] = true
	g.click(hx+2, hy+2)
	if g.act == nil {
		t.Error("the same click must land once the mouse is back on")
	}
}

// The bar lives under SetMouse OFF — only LockBar gates it. The disk button
// still opens the menu; with LockBar it goes dead.
func TestSetMouseOffKeepsTheBarAlive(t *testing.T) {
	bar := shippedBar(t)
	g := &Game{bar: bar, gs: types.NewGameState(), mode: modePlay}
	g.gs.UI["mouse"] = false
	mx := (bar.SaveBox[0] + bar.SaveBox[2]) / 2
	my := (bar.SaveBox[1] + bar.SaveBox[3]) / 2

	g.click(mx, my)
	if g.mode != modeOptions {
		t.Errorf("mode = %d, want the disk button alive (modeOptions)", g.mode)
	}

	g.mode = modePlay
	g.gs.UI["barlock"] = true
	g.click(mx, my)
	if g.mode != modePlay {
		t.Errorf("mode = %d, want LockBar to keep the bar dead", g.mode)
	}
}

// A click during a movie means "skip" and works even under SetMouse OFF: the
// engine takes it before it looks at the flag (0x402e1c).
func TestSetMouseOffKeepsTheSkip(t *testing.T) {
	fs := repositories.SceneParser{}.ParseFrameScript(
		"MovieName Int1.mv;\nTotalFrames 2;\n" +
			"Frame 0,1;\nDelay 140;\nInterrupt ON;\n" +
			"Frame 1,1;\nDelay 140;\nSetVar Seen,1;\nEnd;",
	)
	g, _ := skipGame(t, fs)
	g.gs.UI["mouse"] = false

	g.click(100, 100)
	if g.act != nil {
		t.Fatal("the click must fast-forward the cutscene")
	}
	if g.gs.Var("Seen") != 1 {
		t.Error("the skipped script's state must still land")
	}
	if !g.gs.UI["mouse"] {
		t.Error("the finished movie must hand the mouse back")
	}
}

// Starting a movie turns the mouse off and finishing it turns it back on, as
// the engine does around every script (0x40e936/0x40ebea) — half the scripts
// end on SetMouse OFF and rely on exactly this.
func TestActionTogglesTheMouse(t *testing.T) {
	g, _ := clickGame(t, poolScript)
	if !g.startObjectAction("pool") {
		t.Fatal("the pool's script must start")
	}
	if g.gs.UI["mouse"] {
		t.Error("a starting movie must take the mouse")
	}
	for i := 0; i < 100 && g.act != nil; i++ {
		g.updateAction(1)
	}
	if g.act != nil {
		t.Fatal("the movie must have played out")
	}
	if !g.gs.UI["mouse"] {
		t.Error("the finished movie must hand the mouse back")
	}
}

// SetMouse OFF silences the hover captions along with the clicks (0x4153e9).
func TestSetMouseOffSilencesHover(t *testing.T) {
	g, _ := clickGame(t, poolScript)
	g.texts = []string{"", "лужа"}
	hx, hy := g.grid.Corner(5, 1)

	g.updateHover(hx+2, hy+2)
	if g.hover != "лужа" {
		t.Fatalf("hover = %q, want the caption with the mouse on", g.hover)
	}
	g.gs.UI["mouse"] = false
	g.updateHover(hx+2, hy+2)
	if g.hover != "" {
		t.Errorf("hover = %q, want silence with the mouse off", g.hover)
	}
}

// A click on the acting character's own cell aims the held item at himself:
// the five-letter script runs (ROHAT puts the hat on). With the bare hand the
// click is eaten whole — no script and no walk (engine 0x40f110).
func TestClickOnOwnCellRunsTheSelfScript(t *testing.T) {
	const selfScript = "MovieName Rohat.mv;\nTotalFrames 1;\n" +
		"Frame 0,1;\nDelay 142;\nEnd;"
	g, rec := clickGame(t, poolScript)
	pack := g.sceneC.(*fsPack)
	pack.files["ROHAT.FS"] = []byte(selfScript)
	g.gs.Active = "hat"
	x, y := g.grid.Corner(g.cell[0], g.cell[1])

	g.click(x+2, y+2)
	if g.act == nil || g.act.fs.MovieName != "Rohat.mv" {
		t.Fatal("the five-letter self script must be playing")
	}
	if rec.paths != 0 {
		t.Error("a self action walks nowhere")
	}
	for i := 0; i < 100 && g.act != nil; i++ {
		g.updateAction(1) // play it out, which hands the mouse back
	}

	// The bare hand on himself: eaten, no walk either.
	g.gs.Active = "hand"
	g.click(x+2, y+2)
	if g.act != nil || rec.paths != 0 {
		t.Error("the bare hand on himself must do nothing at all")
	}

	// An item the scene has no self script for: eaten too, not a walk.
	g.gs.Active = "axe"
	g.click(x+2, y+2)
	if g.act != nil || rec.paths != 0 {
		t.Error("a scriptless self click must be eaten, not walked")
	}

	// A click anywhere else still walks.
	wx, wy := g.grid.Corner(2, 1)
	g.click(wx+2, wy+2)
	if rec.paths != 1 {
		t.Errorf("routed %d walks, want the ordinary click to walk", rec.paths)
	}
}

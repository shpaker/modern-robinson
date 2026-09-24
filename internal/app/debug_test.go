package app

import (
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// debugGame is a bare play scene for the debug layers: a 4x3 grid of 10px
// cells with its top-right corner walled off, mouse on.
func debugGame() *Game {
	sc := &types.Scene{
		GridSize:   [2]int{10, 10},
		GridLength: [2]int{4, 3},
		ClosedVert: [][2]int{{3, 0}},
	}
	g := &Game{grid: use_cases.NewGrid(sc, 0, 0), gs: types.NewGameState()}
	g.interp.Observe = g.trace
	return g
}

func logTexts(g *Game) []string {
	out := make([]string, len(g.dbg.log))
	for i, l := range g.dbg.log {
		out[i] = fmt.Sprintf("%s x%d", l.text, l.n)
	}
	return out
}

// The log keeps the last commands, folds a repeat into a count, and leaves out
// the sounds and walk-cycle Z steps that would flood it.
func TestCommandLog(t *testing.T) {
	g := debugGame()
	g.sceneName = "SCENA0"
	g.logCommand(types.Command{Kw: "Sound", Args: []string{"step", "1"}})
	g.logCommand(types.Command{Kw: "Set", Args: []string{"Roby", "Z", "6"}})
	g.logCommand(types.Command{Kw: "Set", Args: []string{"Roby", "X", "2"}})
	g.logCommand(types.Command{Kw: "Set", Args: []string{"Roby", "X", "2"}})
	want := []string{"[SCENA0] set Roby,X,2 x2"}
	if got := logTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("log = %v, want %v", got, want)
	}
	for i := 0; i < 2*debugLogLines; i++ {
		g.logCommand(types.Command{Kw: "SetVar", Args: []string{"n", fmt.Sprint(i)}})
	}
	if len(g.dbg.log) != debugLogLines {
		t.Fatalf("log holds %d lines, want %d", len(g.dbg.log), debugLogLines)
	}
	if last := g.dbg.log[debugLogLines-1].text; !strings.HasSuffix(last, ",19") {
		t.Fatalf("last line = %q, want the newest command", last)
	}
}

// State commands never reach applyEffect; the interpreter's Observe hook is how
// they get into the trace and the log at all.
func TestStateCommandsReachTheLog(t *testing.T) {
	g := debugGame()
	g.applyEvents([]types.Command{
		{Kw: "SetVar", Args: []string{"CrabNeed", "1"}},
		{Kw: "AddItem", Args: []string{"axe"}},
	})
	want := []string{"[] setvar CrabNeed,1 x1", "[] additem axe x1"}
	if got := logTexts(g); !reflect.DeepEqual(got, want) {
		t.Fatalf("log = %v, want %v", got, want)
	}
}

// A changed variable lights up for a while, a zeroed one included; a new state
// (a load, a new run) only resets the baseline.
func TestStateChangesLightUp(t *testing.T) {
	g := debugGame()
	g.gs.SetVar("TreeIs", 1)
	g.noteStateChanges(0)
	if len(g.dbg.flash) != 0 {
		t.Fatalf("the first look lit %v", g.dbg.flash)
	}
	g.gs.SetVar("TreeIs", 0)
	g.gs.SetCharVar("rohangol", "r1hangol")
	g.noteStateChanges(0.1)
	vars, side, _, lit := g.statePanel()
	if !lit["treeis=0"] || !lit["rohangol=r1hangol"] {
		t.Fatalf("lit = %v (vars %v, side %v)", lit, vars, side)
	}
	g.noteStateChanges(debugFlash)
	if _, _, _, lit := g.statePanel(); len(lit) != 0 {
		t.Fatalf("still lit after %v s: %v", debugFlash, lit)
	}
	g.gs = types.NewGameState()
	g.gs.SetVar("Loaded", 5)
	g.noteStateChanges(0.1)
	if len(g.dbg.flash) != 0 {
		t.Fatalf("a swapped-in state lit %v", g.dbg.flash)
	}
}

// The preview tells a click the way click() takes it: a zone gets the script
// composed for it, the hero's own cell with the hand is eaten, and anything
// else walks to the nearest free cell.
func TestPreviewClick(t *testing.T) {
	g := debugGame()
	g.gs.UI["mouse"] = true
	g.gs.Active, g.gs.ActiveChar = "hand", "Roby"
	g.sceneC = newPack("ROHANCRB.FS")
	g.hotspots = []hotspot{{key: "crb", rect: image.Rect(0, 20, 10, 30), z: 7}}
	g.cell = [2]int{1, 1}

	if pv := g.previewClick(5, 25); pv.hot == nil ||
		pv.what != "crb z=7 -> ROHANCRB" {
		t.Errorf("zone: %q", pv.what)
	}
	if pv := g.previewClick(15, 15); pv.walk || pv.what != "self: hand, eaten" {
		t.Errorf("own cell: %q", pv.what)
	}
	pv := g.previewClick(35, 5) // the walled-off corner
	if !pv.walk || pv.goal != [2]int{2, 0} || pv.cell != [2]int{3, 0} {
		t.Fatalf("corner: %+v", pv)
	}
	want := [][2]int{{1, 1}, {2, 0}}
	if !reflect.DeepEqual(pv.route, want) {
		t.Errorf("route = %v, want %v", pv.route, want)
	}
	g.gs.UI["mouse"] = false
	if pv := g.previewClick(35, 5); pv.walk || pv.what != "mouse OFF" {
		t.Errorf("SetMouse OFF: %q", pv.what)
	}
}

// A route keeps every planned cell; what is left of it starts past the cell
// the walker stands on, and before the first step lands that is all of it.
func TestRemainingRoute(t *testing.T) {
	path := [][2]int{{1, 0}, {2, 0}, {3, 0}}
	if got := remaining([2]int{0, 0}, path); len(got) != 3 {
		t.Errorf("before the first step: %v", got)
	}
	if got := remaining([2]int{2, 0}, path); !reflect.DeepEqual(
		got, [][2]int{{3, 0}},
	) {
		t.Errorf("halfway: %v", got)
	}
	if got := remaining([2]int{3, 0}, path); len(got) != 0 {
		t.Errorf("arrived: %v", got)
	}
}

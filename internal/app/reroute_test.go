package app

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// rerouting builds a hero part-way through a walk: cycles is the chain, idx the
// cycle on screen (entered, so its frame 0 already moved the cell) and cell
// where that left him. Nothing is loaded; rerouteWalk only edits the chain.
func rerouting(cycles []string, idx int, cell [2]int) (*Game, *walker) {
	sc := scena0()
	g := &Game{grid: use_cases.NewGrid(sc, sc.Size[0], sc.Size[1])}
	g.cell = cell
	g.roby = walker{
		prefix: "RG", chr: "ROBY", cycles: cycles, idx: idx,
		cur: testCycle(2, step(90), step(90)), frame: 1,
	}
	return g, &g.roby
}

func sameChain(t *testing.T, w *walker, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(w.cycles, want) {
		t.Errorf("cycles = %v, want %v", w.cycles, want)
	}
}

// A new goal mid-step never cuts the cycle on screen: its frames carry the
// motion, and a fresh 5·d start would snap the figure back to the cell anchor.
// RG_66 has already moved him onto (2,0) and commits to one more step east, so
// the new route starts from (3,0) and turns through RG_62.
func TestRerouteMidStepTurnsAfterTheCycle(t *testing.T) {
	g, w := rerouting([]string{"56", "66", "66", "65"}, 1, [2]int{2, 0})
	cur := w.cur
	route, ok := g.rerouteWalk(
		w, g.cell, [2]int{3, 2}, [][2]int{{2, 0}, {3, 0}, {4, 0}}, g.grid.Path,
	)
	if !ok {
		t.Fatal("rerouteWalk = false, want the walk changed")
	}
	sameChain(t, w, "56", "66", "62", "22", "25")
	if w.idx != 1 || w.cur != cur || w.frame != 1 {
		t.Errorf("idx=%d frame=%d: the cycle on screen must play on",
			w.idx, w.frame)
	}
	want := [][2]int{{3, 0}, {3, 1}, {3, 2}}
	if !reflect.DeepEqual(route, want) {
		t.Errorf("route = %v, want %v", route, want)
	}
}

// The start cycle 5·d moves no cell yet, but commits to d all the same.
func TestRerouteDuringTheStart(t *testing.T) {
	g, w := rerouting([]string{"56", "66", "65"}, 0, [2]int{1, 0})
	route, ok := g.rerouteWalk(
		w, g.cell, [2]int{2, 2}, [][2]int{{2, 0}, {3, 0}}, g.grid.Path,
	)
	if !ok {
		t.Fatal("rerouteWalk = false, want the walk changed")
	}
	sameChain(t, w, "56", "62", "22", "25")
	if want := [][2]int{{2, 0}, {2, 1}, {2, 2}}; !reflect.DeepEqual(
		route, want,
	) {
		t.Errorf("route = %v, want %v", route, want)
	}
}

// The brake ends standing on the anchor, so the new route waits for it and
// starts from rest.
func TestRerouteDuringTheBrakeStartsAfterIt(t *testing.T) {
	g, w := rerouting([]string{"56", "66", "65"}, 2, [2]int{3, 0})
	route, ok := g.rerouteWalk(
		w, g.cell, [2]int{1, 0}, [][2]int{{2, 0}, {3, 0}}, g.grid.Path,
	)
	if !ok {
		t.Fatal("rerouteWalk = false, want the walk changed")
	}
	sameChain(t, w, "56", "66", "65", "54", "44", "45")
	if want := [][2]int{{2, 0}, {1, 0}}; !reflect.DeepEqual(route, want) {
		t.Errorf("route = %v, want %v", route, want)
	}
}

// The goal the walk already has changes nothing (WalkTo 0x40f514), and neither
// does a goal with no path to it: the old walk simply goes on.
func TestRerouteKeepsTheWalkForTheSameOrNoGoal(t *testing.T) {
	old := []string{"56", "66", "65"}
	route := [][2]int{{2, 0}, {3, 0}}
	for _, goal := range [][2]int{{3, 0}, {7, 0}} {
		g, w := rerouting(append([]string(nil), old...), 1, [2]int{2, 0})
		got, ok := g.rerouteWalk(w, g.cell, goal, route, g.grid.Path)
		if ok {
			t.Errorf("goal %v: rerouteWalk = true, want the walk kept", goal)
		}
		sameChain(t, w, old...)
		if !reflect.DeepEqual(got, route) {
			t.Errorf("goal %v: route = %v, want %v", goal, got, route)
		}
	}
}

// A click on his own cell only finishes the step he is committed to, then he
// brakes there (WalkTo 0x40f331), rather than turning back.
func TestRerouteToOwnCellFinishesTheStep(t *testing.T) {
	g, w := rerouting([]string{"56", "66", "66", "65"}, 1, [2]int{2, 0})
	route, ok := g.rerouteWalk(
		w, g.cell, [2]int{2, 0}, [][2]int{{2, 0}, {3, 0}, {4, 0}}, g.grid.Path,
	)
	if !ok {
		t.Fatal("rerouteWalk = false, want the walk changed")
	}
	sameChain(t, w, "56", "66", "65")
	if want := [][2]int{{3, 0}}; !reflect.DeepEqual(route, want) {
		t.Errorf("route = %v, want %v", route, want)
	}
}

// stepY is the authored row step a walk cycle fires on its frame 0.
func stepY(char string, d int) types.Command {
	return types.Command{
		Kw:   "shift",
		Args: []string{char, "Y", strconv.Itoa(d)},
	}
}

// turnableCycles adds to walkableCycles the hero's cycles for turning south
// off an eastbound walk and walking on down.
func turnableCycles(g *Game) {
	walkableCycles(g)
	g.cycleCache["RG_62"] = fakeCycle(stepX("Roby", 1))
	g.cycleCache["RG_22"] = fakeCycle(stepY("Roby", 1))
	g.cycleCache["RG_25"] = fakeCycle(stepY("Roby", 1))
}

// Played out, a rerouted walk ends on the new goal having walked every cell of
// the way: the committed step first, then the new route.
func TestReroutedWalkArrivesOnTheNewGoal(t *testing.T) {
	g, _ := actGame(scena0(), "", nil, "Roby")
	turnableCycles(g)
	g.cell = [2]int{1, 0}
	evs, ok := g.startWalk(&g.roby, g.cell, [][2]int{{2, 0}, {3, 0}, {4, 0}})
	if !ok {
		t.Fatal("startWalk = false, want the walk started")
	}
	g.path = [][2]int{{2, 0}, {3, 0}, {4, 0}}
	g.applyWalkEvents(evs)
	g.updateWalk(0.18) // into RG_66: he now stands on (2,0)
	if g.cell != [2]int{2, 0} || g.roby.cycleKey(g.roby.idx) != "RG_66" {
		t.Fatalf("cell=%v cycle=%s, want (2,0) on RG_66",
			g.cell, g.roby.cycleKey(g.roby.idx))
	}
	cur, frame := g.roby.cur, g.roby.frame
	g.rerouteRoby([2]int{3, 2})
	if g.roby.cur != cur || g.roby.frame != frame {
		t.Fatal("the reroute restarted the cycle on screen")
	}
	for i := 0; i < 100 && g.roby.walking(); i++ {
		g.updateWalk(0.09)
	}
	if g.cell != [2]int{3, 2} {
		t.Errorf("cell = %v, want the new goal (3,2)", g.cell)
	}
}

// A click on an object mid-walk no longer stops the hero dead on his cell:
// frame 0 runs at once and its Aproach reroutes the walk, so the step on screen
// plays on, and the movie waits until he arrives.
func TestObjectClickMidWalkReroutesTheWalk(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 90;\nAproach Roby, 4,0;\n" +
		"Frame 1,1;\nDelay 90;\nSetVar probe,1;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	walkableCycles(g)
	g.cell = [2]int{1, 0}
	evs, _ := g.startWalk(&g.roby, g.cell, [][2]int{{2, 0}, {3, 0}})
	g.path = [][2]int{{2, 0}, {3, 0}}
	g.applyWalkEvents(evs)
	g.updateWalk(0.09) // mid RG_56
	cur, frame := g.roby.cur, g.roby.frame

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	if !g.roby.walking() || g.roby.cur != cur || g.roby.frame != frame {
		t.Fatal("the click must not cut the step on screen")
	}
	g.updateAction(0)
	if g.act == nil || g.act.wait != waitRoby {
		t.Fatalf("act = %+v, want the movie waiting for the hero", g.act)
	}
	for i := 0; i < 200 && g.act != nil; i++ {
		if g.act.wait == waitRoby && g.gs.Var("probe") != 0 {
			t.Fatal("frame 1 fired while the hero was still walking")
		}
		g.updateWalk(0.09)
		g.updateAction(0.09)
	}
	if g.cell != [2]int{4, 0} {
		t.Errorf("cell = %v, want the Aproach goal (4,0)", g.cell)
	}
	if g.gs.Var("probe") != 1 {
		t.Error("frame 1 must fire once he has arrived")
	}
}

// A script with no Aproach, started mid-walk, runs its frame 0 at once and then
// holds: ROBY.EXE ticks a movie through its owner only while he stands. The
// hero finishes the walk he was on and the movie plays where it ends.
func TestActionHoldsWhileItsOwnerWalks(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 90;\nSetVar first,1;\n" +
		"Frame 1,1;\nDelay 90;\nSetVar probe,1;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	walkableCycles(g)
	g.cell = [2]int{1, 0}
	evs, _ := g.startWalk(&g.roby, g.cell, [][2]int{{2, 0}, {3, 0}})
	g.path = [][2]int{{2, 0}, {3, 0}}
	g.applyWalkEvents(evs)

	g.startObjectAction("pool")
	g.updateAction(0)
	if g.gs.Var("first") != 1 {
		t.Error("frame 0 must run at once")
	}
	if g.act == nil || g.act.wait != waitRoby {
		t.Fatalf("act = %+v, want the movie held for the walking hero", g.act)
	}
	for i := 0; i < 200 && g.act != nil; i++ {
		if g.roby.walking() && g.gs.Var("probe") != 0 {
			t.Fatal("frame 1 fired while the hero was still walking")
		}
		g.updateWalk(0.09)
		g.updateAction(0.09)
	}
	if g.cell != [2]int{3, 0} || g.gs.Var("probe") != 1 {
		t.Errorf("cell=%v probe=%d, want the walk finished, then frame 1",
			g.cell, g.gs.Var("probe"))
	}
}

// Skipping such a movie lands the walking hero where his walk ends, just as
// it does for a movie paused on its own Aproach.
func TestSkipLandsAWalkingOwner(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 90;\nSetVar first,1;\n" +
		"Frame 1,1;\nDelay 90;\nSetVar probe,1;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.audio = &fakeAudio{}
	g.gs.UI["interrupt"] = true
	walkableCycles(g)
	g.cell = [2]int{1, 0}
	evs, _ := g.startWalk(&g.roby, g.cell, [][2]int{{2, 0}, {3, 0}})
	g.path = [][2]int{{2, 0}, {3, 0}}
	g.applyWalkEvents(evs)

	g.startObjectAction("pool")
	g.updateAction(0)
	if !g.skipCutscene() {
		t.Fatal("skipCutscene = false, want the skip taken")
	}
	if g.cell != [2]int{3, 0} || g.roby.walking() {
		t.Errorf("cell=%v walking=%v, want him landed on (3,0)",
			g.cell, g.roby.walking())
	}
	if g.gs.Var("probe") != 1 {
		t.Error("the skipped frames' state must still land")
	}
}

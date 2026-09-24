package app

import (
	"reflect"
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// An object's .OB walls and step fences come and go with the object: the crab
// blocks its two cells while it sits there and opens them when it is caught —
// without that, the ford it lies on stays walled off for good.
func TestObjectBlockingAppliesAndLifts(t *testing.T) {
	sc := &types.Scene{
		GridSize:   [2]int{10, 10},
		GridLength: [2]int{4, 4},
	}
	g := &Game{grid: use_cases.NewGrid(sc, 0, 0), gs: types.NewGameState()}
	crab := &sceneObj{
		ref: types.ObjectRef{Name: "crb", GX: 2, GY: 1},
		ob: &types.SceneObject{
			Name:       "crb",
			ClosedVert: [][2]int{{0, -1}, {0, 0}},
			ClosedDir:  [][3]int{{-1, 0, 6}},
		},
	}
	g.sceneObjs = []*sceneObj{crab}

	g.applyObjectBlocking()
	if g.grid.Valid(2, 0) || g.grid.Valid(2, 1) {
		t.Fatal("the crab's cells must be blocked while it is on stage")
	}
	if p := g.grid.Path([2]int{1, 1}, [2]int{1, 0}); len(p) == 0 {
		t.Fatal("cells beside the crab must stay reachable")
	}

	g.hideObject("crb")
	if !g.grid.Valid(2, 0) || !g.grid.Valid(2, 1) {
		t.Error("the caught crab must open its cells")
	}
	if p := g.grid.Path([2]int{1, 1}, [2]int{2, 1}); len(p) != 2 {
		t.Errorf("path onto the freed cell = %v, want the direct step", p)
	}
}

// SCENA1's back row lies behind the oak, and the scene fences the ways into
// it: no diagonal out of (5,1), none of (4,1)'s upward steps, and the liana
// walls off (4,1) itself. Walked from the right edge to the oak's cell, the hero
// goes up at the edge and along the back row behind the trunk. Cutting the
// diagonal instead plays RG_74, whose Z 6 draws him over the trunk until its
// frame 12 drops him behind it.
func TestScena1WalkToTheOakGoesAlongTheBackRow(t *testing.T) {
	g := sceneFor(t, "SCENA1")
	for _, s := range g.sceneObjs {
		if !s.ref.Flag {
			s.removed = true // on stage at the start are only the starred ones
		}
	}
	g.applyObjectBlocking()
	if g.grid.Valid(4, 1) {
		t.Error("the liana must wall off (4,1)")
	}
	got := g.grid.Path([2]int{5, 1}, [2]int{2, 0})
	want := [][2]int{{5, 1}, {5, 0}, {4, 0}, {3, 0}, {2, 0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Path((5,1)->(2,0)) = %v, want %v", got, want)
	}
}

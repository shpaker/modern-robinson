package app

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// emptyPacks is a resource layer with no minigame assets. Embedding the
// interface satisfies the type while leaving every other method nil, so a test
// that needs more than this fails loudly instead of silently reading real files.
type emptyPacks struct{ interfaces.IResources }

func (emptyPacks) ScreenPack(string) (map[string]*types.NGB, types.Palette) {
	return map[string]*types.NGB{}, types.Palette{}
}

func (emptyPacks) ScreenPalettes(string) []types.Palette { return nil }

// The launcher scripts put StartGame and the branches that test its result on
// one frame, so the frame has to be suspended and finished later. This drives
// that seam: the tail must be held back while the game runs, then re-judged
// against the result the minigame wrote — the success branch is unreachable
// otherwise, which is what left the hut unbuilt however well it was solved.
func TestStartGameSuspendsAndResumesTheFrame(t *testing.T) {
	frame := []types.Command{
		{Kw: "StartGame", Args: []string{"1", "House", "Find7"}},
		{Kw: "If", Args: []string{"House", "1"}},
		{Kw: "SetVar", Args: []string{"Built", "1"}},
		{Kw: "EndIf"},
		{Kw: "If", Args: []string{"House", "0"}},
		{Kw: "SetVar", Args: []string{"GaveUp", "1"}},
		{Kw: "EndIf"},
	}

	g := &Game{
		res:    emptyPacks{},
		parser: repositories.SceneParser{},
		gs:     types.NewGameState(),
	}
	g.applyEvents(frame)

	// No assets, so the game cannot open: startMinigame lets the quest through
	// with a win and must run the held-back tail itself, since nothing else
	// will.
	if g.mg != nil {
		t.Fatal("no assets: no minigame should be running")
	}
	if g.gs.Var("Built") != 1 {
		t.Errorf("Built = %d, want the success branch to have run",
			g.gs.Var("Built"))
	}
	if g.gs.Var("GaveUp") != 0 {
		t.Errorf("GaveUp = %d, want the failure branch skipped",
			g.gs.Var("GaveUp"))
	}
	if g.mgResume != nil {
		t.Error("the suspended tail must be cleared once it has run")
	}
}

// Giving up takes the other branch, and it must be decided after the game ends
// rather than before it starts.
func TestMinigameResultChoosesTheBranch(t *testing.T) {
	for _, tc := range []struct {
		name          string
		result        int
		built, gaveUp int
	}{
		{"solved", 1, 1, 0},
		{"gave up", 0, 0, 1},
	} {
		g := &Game{
			res:    emptyPacks{},
			parser: repositories.SceneParser{},
			gs:     types.NewGameState(),
		}
		// The frame suspends here; nothing after StartGame may run yet.
		g.mgVar = "House"
		g.mgResume = []types.Command{
			{Kw: "If", Args: []string{"House", "1"}},
			{Kw: "SetVar", Args: []string{"Built", "1"}},
			{Kw: "EndIf"},
			{Kw: "If", Args: []string{"House", "0"}},
			{Kw: "SetVar", Args: []string{"GaveUp", "1"}},
			{Kw: "EndIf"},
		}
		g.mg = stubGame{result: tc.result}

		if !g.updateMinigame(0.1) {
			t.Fatalf("%s: the minigame must own the frame", tc.name)
		}
		if g.mg != nil {
			t.Fatalf("%s: a finished minigame must be cleared", tc.name)
		}
		if got := g.gs.Var("Built"); got != tc.built {
			t.Errorf("%s: Built = %d, want %d", tc.name, got, tc.built)
		}
		if got := g.gs.Var("GaveUp"); got != tc.gaveUp {
			t.Errorf("%s: GaveUp = %d, want %d", tc.name, got, tc.gaveUp)
		}
	}
}

// stubGame finishes on its first update with a fixed result.
type stubGame struct{ result int }

func (s stubGame) update(*Game, float64) (bool, int) { return true, s.result }

func (stubGame) draw(*Game, *ebiten.Image) {}

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// clickRow presses the mouse at the centre of main-menu row i.
func clickRow(g *Game, i int) {
	r := menuRows[i]
	g.updateOptionsMenu((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2, true)
}

// Until a run starts, "continue" and "save" are unavailable, as in the
// original (ROBY.PDF p.24): the rows neither highlight nor click.
func TestStartMenuKeepsContinueAndSaveDead(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1}
	for _, row := range []int{1, 3} {
		clickRow(g, row)
		if g.optHover != -1 {
			t.Errorf("row %d highlights at the start menu", row)
		}
		if g.mode != modeOptions {
			t.Errorf("row %d clicked through: mode = %d", row, g.mode)
		}
	}
}

// "Restore" is how a save is reached from the start menu, so it must work
// before any run: it opens the load screen, and cancelling backs out to the
// menu rather than into a play that does not exist yet.
func TestStartMenuOpensTheLoadScreen(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1}
	clickRow(g, 2)
	if g.mode != modeLoad {
		t.Fatalf("mode = %d, want modeLoad (%d)", g.mode, modeLoad)
	}
	g.updateSlotScreen(cancelButton.Min.X+1, cancelButton.Min.Y+1, true)
	if g.mode != modeOptions {
		t.Errorf("cancel: mode = %d, want back on the menu", g.mode)
	}
}

// "New game" from the start menu takes the same covered path as a restart from
// play: the loading screen sits out the intro bridge.
func TestStartMenuNewGameEntersLoading(t *testing.T) {
	g := &Game{
		res:        noScenes{},
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		cycleCache: map[string]*walkCycle{},
		mode:       modeOptions,
		optHover:   -1,
		optDrag:    -1,
	}
	clickRow(g, 0)
	if g.mode != modeLoading {
		t.Errorf("mode = %d, want modeLoading (%d)", g.mode, modeLoading)
	}
	if g.started {
		t.Error("a run not yet handed to play must not count as started")
	}
}

// Once a run is underway the same two rows come alive again.
func TestMenuContinueAndSaveComeAliveInARun(t *testing.T) {
	g := &Game{mode: modeOptions, optHover: -1, optDrag: -1, started: true}
	clickRow(g, 1)
	if g.mode != modePlay {
		t.Errorf("continue: mode = %d, want modePlay (%d)", g.mode, modePlay)
	}
	g.mode = modeOptions
	clickRow(g, 3)
	if g.mode != modeSave {
		t.Errorf("save: mode = %d, want modeSave (%d)", g.mode, modeSave)
	}
}

// Esc has no play to fall back to before the first run: it keeps the menu up
// and only backs the slot screens out into it.
func TestEscBeforeARunStaysOnTheMenu(t *testing.T) {
	g := &Game{mode: modeOptions}
	g.toggleOptions()
	if g.mode != modeOptions {
		t.Errorf("menu: mode = %d, want to stay on modeOptions", g.mode)
	}
	g.mode = modeLoad
	g.toggleOptions()
	if g.mode != modeOptions {
		t.Errorf("load screen: mode = %d, want modeOptions", g.mode)
	}
	g.started = true
	g.toggleOptions()
	if g.mode != modePlay {
		t.Errorf("in a run: mode = %d, want modePlay (%d)", g.mode, modePlay)
	}
}

// slotDir points the save files at a test directory while every container
// lookup still bows out early.
type slotDir struct {
	noScenes
	root string
}

func (d slotDir) Root() string { return d.root }

// Restoring a slot is the other way a run starts, and it must light up
// "continue" and "save" just like playing through the intro does.
func TestLoadSlotMarksTheRunStarted(t *testing.T) {
	dir := t.TempDir()
	sav := filepath.Join(dir, "robinson03.sav")
	if err := os.WriteFile(sav, []byte(`{"scene":"SCENA0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{
		res:        slotDir{root: dir},
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		cycleCache: map[string]*walkCycle{},
	}
	if !g.loadSlot(3) {
		t.Fatal("the slot must load")
	}
	if !g.started {
		t.Error("a restored run must count as started")
	}
}

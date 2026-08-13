package app

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
)

// noScenes is a resource layer that finds no containers at all, so loadScene,
// seedStartup and loadCharacter all bow out early and a Game can be restarted
// without touching real files.
type noScenes struct{ interfaces.IResources }

func (noScenes) SceneContainer(string) interfaces.IContainer { return nil }

// Leaving the title used to drop straight into play, which drew the INT0 bridge
// — a black room with a character sprite standing in it — for as long as the
// intro movie took to decode. The title now hands over to the loading screen,
// which sits out the scene it was raised on.
func TestTitleHandsOverToLoading(t *testing.T) {
	g := &Game{mode: modeTitle, modeT: 61, sceneName: "INT0"}
	if !g.updateScreens(1.0 / 60) {
		t.Fatal("the boot screens must still own the frame")
	}
	if g.mode != modeLoading {
		t.Errorf("mode = %d, want modeLoading (%d)", g.mode, modeLoading)
	}
	if g.loadingFrom != "INT0" {
		t.Errorf("loadingFrom = %q, want the scene sat out", g.loadingFrom)
	}
}

// The loading screen holds the frame until the bridge scene has chained onward,
// which is the whole point: hand over any earlier and the bridge shows.
func TestLoadingOwnsTheFrameUntilTheSceneChanges(t *testing.T) {
	cases := []struct {
		name  string
		mode  int
		scene string
		owns  bool
		want  int
	}{
		{"still on the bridge", modeLoading, "INT0", true, modeLoading},
		{"bridge crossed", modeLoading, "INT1", true, modePlay},
		{"not loading at all", modePlay, "INT1", false, modePlay},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := &Game{mode: c.mode, sceneName: c.scene, loadingFrom: "INT0"}
			if got := g.updateLoading(1.0 / 60); got != c.owns {
				t.Errorf("updateLoading owns the frame = %v, want %v",
					got, c.owns)
			}
			if g.mode != c.want {
				t.Errorf("mode = %d, want %d", g.mode, c.want)
			}
		})
	}
}

// A broken GoScene (or a missing target container) leaves the scene name where
// it was; the screen must give up rather than hold the game forever.
func TestLoadingGivesUpOnABridgeThatNeverCrosses(t *testing.T) {
	g := &Game{
		mode:        modeLoading,
		sceneName:   "INT0",
		loadingFrom: "INT0",
		modeT:       30,
	}
	g.updateLoading(1.0 / 60)
	if g.mode != modePlay {
		t.Errorf("mode = %d, want the screen to time out into play", g.mode)
	}
}

// Starting a new game re-enters the same bridge, so it takes the same cover as
// the boot sequence — and does it after resetRun, with the old run's transition
// already cleared.
func TestRestartEntersLoading(t *testing.T) {
	g := &Game{
		res:        noScenes{},
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		cycleCache: map[string]*walkCycle{},
		mode:       modeOptions,
		sceneName:  "SCENA0",
		pending:    &types.Exit{Scene: "SCENA3"},
		fadeCurve:  []float64{1, 0.5},
	}
	g.restart()
	if g.mode != modeLoading {
		t.Errorf("mode = %d, want modeLoading (%d)", g.mode, modeLoading)
	}
	if g.pending != nil || g.fadeCurve != nil {
		t.Error("the old run's transition must not survive the restart")
	}
}

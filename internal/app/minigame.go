package app

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/minigame/catalog"
)

// A minigame takes over the whole 640x480 window while the adventure waits.
// StartGame gameId, resultVar, paramVar launches one; when it finishes, its
// result goes into resultVar and the scene's script resumes.

// startMinigame handles StartGame gameId,resultVar,paramVar.
func (g *Game) startMinigame(args []string) {
	if len(args) < 2 {
		return
	}
	g.mgVar = args[1]
	param := 0
	if len(args) >= 3 {
		param = g.gs.Var(args[2])
	}
	g.mg = nil
	if e, ok := catalog.Get(atoiArg(args[0])); ok {
		g.mg = e.New(minigame.NewHost(g.res, g.audio), param)
	}
	if g.mg == nil {
		// Assets missing: let the quest through rather than dead-end it, and
		// run the suspended tail straight away since nothing will resume it.
		g.gs.SetVar(g.mgVar, 1)
		if rest := g.mgResume; len(rest) > 0 {
			g.mgResume = nil
			g.applyEvents(rest)
		}
	}
}

// updateMinigame runs the active minigame; true while it owns the frame.
func (g *Game) updateMinigame(dt float64) bool {
	if g.mg == nil {
		return false
	}
	done, result := g.mg.Update(dt)
	if done {
		g.mg = nil
		if g.mgVar != "" {
			g.gs.SetVar(g.mgVar, result)
			g.traceState("minigame finished: %s=%d", g.mgVar, result)
		}
		// Resume the frame the StartGame suspended, now that the result is in.
		if rest := g.mgResume; len(rest) > 0 {
			g.mgResume = nil
			g.applyEvents(rest)
		}
	}
	return true
}

// drawMinigame paints the active minigame over everything else.
func (g *Game) drawMinigame(screen *ebiten.Image) {
	if g.mg == nil {
		return
	}
	g.mg.Draw(screen)
}

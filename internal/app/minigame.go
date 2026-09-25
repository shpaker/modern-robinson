package app

import (
	"strings"

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
		g.mg = e.New(earHost{minigame.NewHost(g.res, g.audio), g}, param)
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

// earHost is the Host a minigame gets in the adventure: it plays every sound
// as the plain one does, and lets a driver hear it (control.sounded). It
// holds the game, not its control: a puzzle ROBINSON_MINIGAME opens starts
// before a driver takes the hero.
type earHost struct {
	minigame.Host
	g *Game
}

var _ minigame.Host = earHost{}

func (h earHost) PlaySound(file string, ch int) {
	h.Host.PlaySound(file, ch)
	h.g.ctl.sounded(file)
}

// puzzleSounds is what the ear makes of each sound of the minigames, by file
// in lower case: the driver gets the word, the file stays in the game. The
// organ's notes are not here (soundLabel).
var puzzleSounds = map[string]string{
	// The hut.
	"h_take.wav":  "взял",
	"h_turn.wav":  "повернул",
	"h_back.wav":  "вернул",
	"h_good.wav":  "встало",
	"h_error.wav": "не туда",
	// The island chart.
	"m_take.wav": "взял",
	"m_turn.wav": "повернул",
	"m_put.wav":  "положил",
	"m_good.wav": "склеилось",
	// The translator.
	"r_take.wav":  "взял",
	"r_put.wav":   "поставил",
	"r_back.wav":  "вернул",
	"r_all.wav":   "стёр",
	"r_error.wav": "ошибка",
	// The checkers.
	"move.wav":     "ход",
	"movelady.wav": "ход дамкой",
	"eat.wav":      "съел",
	"lady.wav":     "дамка",
	// The balloon.
	"stnbalon.wav": "посадка",
	// A puzzle solved.
	"final0.wav": "победа",
	"final1.wav": "победа",
	"final3.wav": "победа",
	"final5.wav": "победа",
}

// soundLabel is the word for a sound a puzzle played: the table's, a note for
// each of the organ's pipes, and a plain sound for anything else — never its
// file.
func soundLabel(file string) string {
	f := strings.ToLower(file)
	if w, ok := puzzleSounds[f]; ok {
		return w
	}
	if strings.HasPrefix(f, "pipe") {
		return "нота"
	}
	return "звук"
}

// updateMinigame runs the active minigame; true while it owns the frame.
func (g *Game) updateMinigame(dt float64) bool {
	if g.mg == nil {
		return false
	}
	if g.ctl.stands() {
		return true // a driver answers slower than a player: it waits for him
	}
	if done, result := g.mg.Update(dt); done {
		g.finishMinigame(result)
	}
	return true
}

// finishMinigame hands the screen back to the adventure: the result goes into
// the quest variable, and the frame the StartGame suspended resumes. Giving up
// is result 0, as Esc inside every game.
func (g *Game) finishMinigame(result int) {
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

// drawMinigame paints the active minigame over everything else.
func (g *Game) drawMinigame(screen *ebiten.Image) {
	if g.mg == nil {
		return
	}
	g.mg.Draw(screen)
}

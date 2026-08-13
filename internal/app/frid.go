package app

import (
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
)

// Friday (Frid) is the second protagonist. He never walks on his own: scripts
// place him (Set Frid,X/Y/Z), show and hide him (ShowChar/HideChar), and move
// him between scenes with the 7-arg GoScene. When he is the active character
// (SetActive Frid) object clicks resolve FRHAN* scripts instead of ROHAN*.

// fridInit resets Friday's per-scene presentation state (STARTUP.INF declares
// him at ZCoord 7; INT0 hides him for the whole intro).
func (g *Game) fridInit() {
	if g.fridIdle == nil {
		g.fridIdle = adapters.LoadAnimation(g.res, "Frhead.mv")
		g.fridZ = 7 // STARTUP.INF: Frid, 7, 0, 0, *
		// Debug/test aid: place Friday explicitly ("gx,gy").
		if v := os.Getenv("ROBINSON_FRID"); v != "" {
			if x, y, ok := strings.Cut(v, ","); ok {
				g.fridCell = [2]int{atoiArg(x), atoiArg(y)}
				g.fridHidden = false
			}
		}
	}
	g.fridFrame = 0
	g.fridT = 0
	g.fridSync() // his screen position follows the cell he was placed on
}

// fridVisible reports whether Friday should be drawn in the current scene.
func (g *Game) fridVisible() bool {
	return !g.fridHidden && g.gs.Var("FridIs") == 1 && g.fridIdle != nil &&
		g.fridIdle.OK()
}

// updateFrid advances Friday's idle loop.
func (g *Game) updateFrid(dt float64) {
	if !g.fridVisible() || len(g.fridPath) > 0 {
		return
	}
	g.fridT += dt
	if g.fridT >= 0.114 { // FRHEAD.FS: Delay 114 per frame
		g.fridT = 0
		g.fridFrame = (g.fridFrame + 1) % len(g.fridIdle.Frames)
	}
}

// drawFrid renders Friday's idle at his grid cell.
func (g *Game) drawFrid(screen *ebiten.Image) {
	if !g.fridVisible() {
		return
	}
	if g.act != nil && g.act.frid && g.act.started && g.act.wait == waitNone {
		// Her own action movie stands in for the idle (see drawAction) — but a
		// movie paused for an Aproach walk steps aside, so she draws her own
		// walk cycles on the way over.
		return
	}
	a, fi := g.fridIdle, g.fridFrame
	if len(g.fridPath) > 0 {
		if w := g.fridWalk.anim(); w.OK() {
			a, fi = w, g.fridWalk.frame
		}
	}
	fi %= len(a.Frames)
	drawAnim(screen, a, fi, g.fridPos[0]-float64(g.camX), g.fridPos[1])
}

// fridEffect applies a Frid-targeted command (Set/Aproach/Show/Hide); returns
// false when the command is not about Friday.
func (g *Game) fridEffect(kw string, args []string) bool {
	if len(args) == 0 || !strings.EqualFold(args[0], "Frid") {
		return false
	}
	switch kw {
	case "set":
		if len(args) >= 3 {
			n := atoiArg(args[2])
			switch strings.ToUpper(args[1]) {
			case "X":
				g.fridCell[0] = n
				g.fridSync()
			case "Y":
				g.fridCell[1] = n
				g.fridSync()
			case "Z":
				g.fridZ = n
			}
		}
	case "shift":
		// The authored cell step of her walk cycles (and a script nudge). It
		// re-anchors her without fridSync: syncing would clear fridPath on the
		// very first step, ending the walk as far as the rest of the engine
		// (walk pose, a paused action movie) can tell.
		if len(args) >= 3 {
			n := atoiArg(args[2])
			switch strings.ToUpper(args[1]) {
			case "X":
				g.fridCell[0] += n
				g.fridSnapPos()
			case "Y":
				g.fridCell[1] += n
				g.fridSnapPos()
			}
		}
	case "aproach", "approach":
		// Friday walks to the target with his own four-direction cycles;
		// scripts, not the player, decide where he goes.
		switch len(args) {
		case 3:
			g.fridWalkTo(atoiArg(args[1]), atoiArg(args[2]))
		case 4:
			if cx, cy, ok := g.objCell(args[1]); ok {
				g.fridWalkTo(cx+atoiArg(args[2]), cy+atoiArg(args[3]))
			}
		}
	case "hidechar":
		g.fridHidden = true
	case "showchar":
		g.fridHidden = false
	default:
		return false
	}
	return true
}

// runFridEntry executes Friday's arrival script silently: only his state
// commands matter (Set Frid...), no movie is rendered for the hidden helper.
func (g *Game) runFridEntry(name string) {
	raw, err := g.sceneC.ExtractName(strings.ToUpper(name) + ".FS")
	if err != nil {
		return
	}
	fs := g.parser.ParseFrameScript(string(raw))
	for _, fr := range fs.Frames {
		for _, ev := range fr.Events {
			g.fridEffect(strings.ToLower(ev.Kw), ev.Args)
		}
	}
}

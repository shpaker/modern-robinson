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
}

// fridVisible reports whether Friday should be drawn in the current scene.
func (g *Game) fridVisible() bool {
	return !g.fridHidden && g.gs.Var("FridIs") == 1 && g.fridIdle != nil &&
		g.fridIdle.OK()
}

// updateFrid advances Friday's idle loop.
func (g *Game) updateFrid(dt float64) {
	if !g.fridVisible() {
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
	fi := g.fridFrame % len(g.fridIdle.Frames)
	frame, anch := g.fridIdle.Frames[fi], g.fridIdle.Anchors[fi]
	px, py := g.grid.ToScreen(g.fridCell[0], g.fridCell[1])
	x := float64(px-g.camX) - float64(anch[0])
	y := float64(py) - float64(anch[1])
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	screen.DrawImage(frame, op)
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
			case "Y":
				g.fridCell[1] = n
			case "Z":
				g.fridZ = n
			}
		}
	case "shift":
		if len(args) >= 3 {
			n := atoiArg(args[2])
			switch strings.ToUpper(args[1]) {
			case "X":
				g.fridCell[0] += n
			case "Y":
				g.fridCell[1] += n
			}
		}
	case "aproach", "approach":
		// Friday teleports to the approach target (no free-walk of his own).
		switch len(args) {
		case 3:
			g.fridCell = [2]int{atoiArg(args[1]), atoiArg(args[2])}
		case 4:
			if cx, cy, ok := g.objCell(args[1]); ok {
				g.fridCell = [2]int{
					cx + atoiArg(args[2]),
					cy + atoiArg(args[3]),
				}
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

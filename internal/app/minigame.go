package app

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// A minigame takes over the whole 640x480 window while the adventure waits.
// StartGame gameId, resultVar, paramVar launches one; when it finishes, its
// result goes into resultVar and the scene's script resumes.
type minigame interface {
	// update advances the game; done reports it finished, result is the value
	// written into the quest variable (1 = solved, 0 = given up / failed).
	update(g *Game, dt float64) (done bool, result int)
	draw(g *Game, screen *ebiten.Image)
}

// startMinigame handles StartGame gameId,resultVar,paramVar.
func (g *Game) startMinigame(args []string) {
	if len(args) < 2 {
		return
	}
	id := atoiArg(args[0])
	g.mgVar = args[1]
	g.mgParam = 0
	if len(args) >= 3 {
		g.mgParam = g.gs.Var(args[2])
	}
	switch id {
	case 5:
		g.mg = newCryptGame(g)
	}
	if g.mg == nil {
		// Not reimplemented yet: let the quest through rather than dead-end it.
		g.gs.SetVar(g.mgVar, 1)
		g.msg, g.msgT = "Мини-игра пока пропускается", 2.5
	}
}

// updateMinigame runs the active minigame; true while it owns the frame.
func (g *Game) updateMinigame(dt float64) bool {
	if g.mg == nil {
		return false
	}
	done, result := g.mg.update(g, dt)
	if done {
		g.mg = nil
		if g.mgVar != "" {
			g.gs.SetVar(g.mgVar, result)
		}
	}
	return true
}

// drawMinigame paints the active minigame over everything else.
func (g *Game) drawMinigame(screen *ebiten.Image) {
	if g.mg == nil {
		return
	}
	g.mg.draw(g, screen)
	g.drawCursor(screen)
}

// packImages decodes a whole top-level pack into Ebiten images, forcing
// full-screen backdrops opaque.
func (g *Game) packImages(pack string) map[string]*ebiten.Image {
	sprites, pal := g.res.ScreenPack(pack)
	out := make(map[string]*ebiten.Image, len(sprites))
	for name, n := range sprites {
		if n == nil {
			continue
		}
		rgba := n.RGBA(pal)
		if n.Width == ViewW && n.Height == ViewH {
			for i := 0; i < n.Width*n.Height; i++ {
				rgba[i*4+3] = 255
			}
		}
		img := ebiten.NewImage(n.Width, n.Height)
		img.WritePixels(rgba)
		out[name] = img
	}
	return out
}

// blitAt draws a pack image at (x, y).
func blitAt(screen *ebiten.Image, img *ebiten.Image, x, y int) {
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

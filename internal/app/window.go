package app

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// The window: its own keys and the pointer on the frame. Ebiten maps the
// mouse onto the 640x480 frame in a straight line, and in a window that is the
// whole story. Full screen leaves black fields beside the frame, where the
// pointer would leave it: the cursor the game draws would vanish and the scene
// would stop scrolling at its edge. The original ran on a 640x480 screen the
// pointer could not leave, so the game holds it to the frame the same way.

// toggleFullscreen is F: the whole screen or the window, kept in config.yml
// for the next start, as the sliders are.
func (g *Game) toggleFullscreen() {
	on := !ebiten.IsFullscreen()
	ebiten.SetFullscreen(on)
	g.keepSwitch("fullscreen", on)
}

// aim is where the mouse points, on the frame or beside it. The menus and the
// minigames show the system arrow, which may well sit in a black field, and a
// click there must hit nothing.
func (g *Game) aim() (int, int) {
	x, y := ebiten.CursorPositionF()
	return int(math.Floor(x)), int(math.Floor(y))
}

// pointer is the mouse where the game draws the cursor itself: in full screen
// it rests on the frame's edge instead of vanishing beside it. A window's
// pointer outside it stays there, on no edge (edgeScroll).
func (g *Game) pointer() (int, int) {
	x, y := ebiten.CursorPositionF()
	x, y = holdToFrame(x, y, ebiten.IsFullscreen())
	return int(math.Floor(x)), int(math.Floor(y))
}

// holdToFrame keeps a position on the frame's pixels when hold is set.
func holdToFrame(x, y float64, hold bool) (float64, float64) {
	if !hold {
		return x, y
	}
	return clampF(x, 0, ViewW-1), clampF(y, 0, ViewH-1)
}

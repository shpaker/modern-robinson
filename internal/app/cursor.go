package app

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The mouse cursor, the way ROBY.EXE runs it. Its cursors are the executable's
// own Win32 resources (repositories.Cursors); every tick the engine picks one
// (0x418c90, called from OnIdle) and hands it to SetCursor, so the original's
// cursor is the system's. Ebiten can show no custom cursor, so the game's are
// drawn over the frame with the system cursor hidden. Where the original
// leaves the plain Windows arrow — the menus and the minigames — the system
// cursor is simply shown.

// cursorWait is the waiting clock: the engine sets it while the mouse is
// switched off (SetMouse OFF, which every movie does) and while a scene loads.
const cursorWait = "247"

// cursorHand is the bar's cursor, whoever is acting (AdvancedItems[0]).
const cursorHand = "HAND"

// cursorSprite is a cursor ready to draw: its image and the hotspot that sits
// on the pointer.
type cursorSprite struct {
	img        *ebiten.Image
	hotX, hotY int
}

// loadCursors turns the executable's cursors into sprites.
func (g *Game) loadCursors() {
	g.cursors = map[string]cursorSprite{}
	for name, c := range g.res.Cursors() {
		img := ebiten.NewImage(c.W, c.H)
		img.WritePixels(c.Pix)
		g.cursors[name] = cursorSprite{img: img, hotX: c.HotX, hotY: c.HotY}
	}
}

// playCursor is 0x418c90's pick during play.
//
// The scene reaches down to y <= ScreenSize.y, so the intro bridges, 480 tall,
// never get to the bar. There an object whose .OB Cursor is 1..4 shows ADV1..4,
// the exit arrows; anything else — bare ground, an ordinary object, the
// heroes — shows the cursor of the item selected in the bar, the hat while the
// hat is in hand. There is no separate "something here" cursor: the name on
// the bar is the only hint.
//
// Over the bar the cursor is the hand, Friday's turn included, and under
// LockBar the bar leaves the cursor as it was. SetMouse OFF turns either into
// the waiting clock, though LockBar is checked first.
func (g *Game) playCursor(mx, my int) string {
	if my > g.h {
		switch {
		case g.gs.UI["barlock"]:
			return g.cursorName
		case !g.gs.UI["mouse"]:
			return cursorWait
		}
		return cursorHand
	}
	if !g.gs.UI["mouse"] {
		return cursorWait
	}
	if hs := g.hotspotAt(mx+g.camX, my); hs != nil && hs.ob != nil &&
		hs.ob.Cursor >= 1 && hs.ob.Cursor <= 4 {
		return fmt.Sprintf("ADV%d", hs.ob.Cursor)
	}
	if g.gs.Active == "" {
		return cursorHand
	}
	return strings.ToUpper(g.gs.Active)
}

// cursorFor is what the pointer shows in the current mode: the name of one of
// the game's cursors, or none, and whether the system arrow stands in.
func (g *Game) cursorFor(mx, my int) (name string, system bool) {
	switch {
	case g.mode == modeLogo || g.mode == modeTitle:
		return "", false // the intro hides the cursor (0x40a209)
	case g.mode == modeLoading:
		return cursorWait, false
	case g.mode == modeOptions || g.mode == modeSave || g.mode == modeLoad:
		return "", true // the menu's IDC_ARROW (0x4065fd)
	case g.mg != nil:
		return "", true // MINIGAME.DLL sets IDC_ARROW itself
	case !g.gs.UI["cursor"]:
		return "", false // ShowCursor off
	}
	return g.playCursor(mx, my), false
}

// updateCursor makes this tick's pick. A cursor the executable did not provide
// falls back to the system one, so a copy of the game without ROBY.EXE still
// has a pointer.
func (g *Game) updateCursor() {
	name, system := g.cursorFor(ebiten.CursorPosition())
	if _, ok := g.cursors[name]; name != "" && !ok {
		system = true
	}
	g.cursorName = name
	if system != g.cursorSystem {
		g.cursorSystem = system
		if system {
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeHidden)
		}
	}
}

// drawCursor draws the picked cursor with its hotspot on the pointer.
func (g *Game) drawCursor(screen *ebiten.Image) {
	c, ok := g.cursors[g.cursorName]
	if g.cursorSystem || !ok {
		return
	}
	mx, my := ebiten.CursorPosition()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(mx-c.hotX), float64(my-c.hotY))
	screen.DrawImage(c.img, op)
}

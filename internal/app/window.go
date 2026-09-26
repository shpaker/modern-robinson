package app

import (
	"fmt"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters/crt"
)

// The window: its own keys, the CRT the frame is shown on, and the pointer on
// the frame. Ebiten maps the mouse onto the 640x480 frame in a straight line;
// the tube bends the picture, so the pointer takes the same bend and a click
// lands on what the player sees under it. Full screen leaves black fields
// beside the frame, where the pointer would leave it: the cursor the game
// draws would vanish and the scene would stop scrolling at its edge. The
// original ran on a 640x480 screen the pointer could not leave, so the game
// holds it to the frame the same way.

// toggleFullscreen is F: the whole screen or the window, kept in config.yml
// for the next start, as the sliders are.
func (g *Game) toggleFullscreen() {
	on := !ebiten.IsFullscreen()
	ebiten.SetFullscreen(on)
	g.keepSwitch("fullscreen", on)
}

// toggleTube is F3: the CRT on or off, kept in config.yml like F.
func (g *Game) toggleTube() {
	g.tube.Toggle()
	g.keepSwitch("crt", g.tube.On())
}

// newTube builds the CRT, a switched-off one should its shaders fail.
func newTube(cfg Config) *crt.Tube {
	t, err := crt.New(tubeOn(cfg), cfg.Tube)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crt: %v\n", err)
	}
	return t
}

// tubeOn is whether the game starts on the tube: as config.yml says (on,
// unless it says otherwise), but off in a headless run, whose scripted clicks
// and snapshots expect the bare frame. ROBINSON_CRT overrides both.
func tubeOn(cfg Config) bool {
	on := cfg.CRT && !headless
	if v := os.Getenv("ROBINSON_CRT"); v != "" {
		on = truthy(v)
	}
	return on
}

// DrawFinalScreen puts the frame on the screen: through the tube when it is
// on, the way Ebiten would otherwise.
func (g *Game) DrawFinalScreen(
	screen ebiten.FinalScreen,
	frame *ebiten.Image,
	geoM ebiten.GeoM,
) {
	if g.tube.On() {
		g.tube.Draw(screen, frame, geoM, ebiten.IsFullscreen())
		return
	}
	ebiten.DefaultDrawFinalScreen(screen, frame, geoM)
}

var _ ebiten.FinalScreenDrawer = (*Game)(nil)

// frameAt is the mouse on the frame, bent as the tube bends the picture, and
// whether the pointer is over the frame's place at all: inside the window,
// or in full screen on the picture's part of the screen.
func (g *Game) frameAt() (x, y float64, over bool) {
	x, y = ebiten.CursorPositionF()
	over = x >= 0 && x < ViewW && y >= 0 && y < ViewH
	if g.tube.On() {
		x, y = g.tube.Warp(x, y, ViewW, ViewH, ebiten.IsFullscreen())
	}
	return x, y, over
}

// aim is where the mouse points, on the frame or beside it. The menus and the
// minigames show the system arrow, which may well sit in a black field, and a
// click there must hit nothing.
func (g *Game) aim() (int, int) {
	x, y, _ := g.frameAt()
	return int(math.Floor(x)), int(math.Floor(y))
}

// pointer is the mouse where the game draws the cursor itself: over the
// frame's place — in full screen, anywhere — it rests on the frame's edge
// instead of vanishing past it, in a black field or a tube's rounded corner.
// A window's pointer outside it stays there, on no edge (edgeScroll).
func (g *Game) pointer() (int, int) {
	x, y, over := g.frameAt()
	x, y = holdToFrame(x, y, over || ebiten.IsFullscreen())
	return int(math.Floor(x)), int(math.Floor(y))
}

// holdToFrame keeps a position on the frame's pixels when hold is set.
func holdToFrame(x, y float64, hold bool) (float64, float64) {
	if !hold {
		return x, y
	}
	return clampF(x, 0, ViewW-1), clampF(y, 0, ViewH-1)
}

package app

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Screen modes: the boot sequence shows the studio logo, then the title, then
// hands over to play. Click (or any key) skips forward.
const (
	modeLogo = iota
	modeTitle
	modePlay
)

// loadScreens fetches the boot images from LOGO.DAT (LOGO = studio logo,
// ROBINSON = title). Missing packs skip straight to play.
func (g *Game) loadScreens() {
	load := func(name string) *ebiten.Image {
		n, pal := g.res.Screen("LOGO", name)
		if n == nil {
			return nil
		}
		img := ebiten.NewImage(n.Width, n.Height)
		rgba := n.RGBA(pal)
		for i := 0; i < n.Width*n.Height; i++ {
			rgba[i*4+3] = 255
		}
		img.WritePixels(rgba)
		return img
	}
	g.logoImg = load("LOGO")
	g.titleImg = load("ROBINSON")
	if g.logoImg == nil {
		g.mode = modePlay
		return
	}
	g.mode = modeLogo
	g.modeT = 0
}

// updateScreens advances the boot sequence; returns true while it owns the
// frame (play is paused underneath).
func (g *Game) updateScreens(dt float64) bool {
	if g.mode == modePlay {
		return false
	}
	g.modeT += dt
	skip := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		len(inpututil.AppendJustPressedKeys(nil)) > 0
	switch g.mode {
	case modeLogo:
		if skip || g.modeT > 2.5 {
			g.mode, g.modeT = modeTitle, 0
		}
	case modeTitle:
		if skip || g.modeT > 60 {
			g.mode = modePlay
		}
	}
	return true
}

// drawScreens renders the current boot screen with a short fade-in.
func (g *Game) drawScreens(screen *ebiten.Image) {
	img := g.logoImg
	if g.mode == modeTitle {
		img = g.titleImg
	}
	if img == nil {
		return
	}
	screen.DrawImage(img, nil)
	// quarter-second fade-in from black
	if g.modeT < 0.25 {
		a := 1 - g.modeT/0.25
		vector.FillRect(screen, 0, 0, float32(ViewW), float32(ViewH),
			rgba(0, 0, 0, uint8(a*255)), false)
	}
}

package app

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/types"
)

// Screen modes: the boot sequence shows the studio logo, then the title, then
// the main menu — the original opened on it, with "continue" and "save" inert
// until a run starts (ROBY.PDF p.23). "New game" enters the loading screen
// that covers the opening cutscene's decode and only then hands over to play;
// Esc reopens the same menu from play, branching to the save and load screens.
const (
	modeLogo = iota
	modeTitle
	modeLoading
	modePlay
	modeOptions
	modeSave
	modeLoad
)

// fadeStepTime is how long one .FAD step lasts; sixteen steps make ~0.3 s.
const fadeStepTime = 0.019

// clickedThisTick reports a fresh left-button press.
func clickedThisTick() bool {
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// serviceKey reports whether a key already has a job of its own (debug, quick
// save/load, the options menu) and so must not double as "skip".
func serviceKey(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyF1, ebiten.KeyF5, ebiten.KeyF9, ebiten.KeyEscape:
		return true
	}
	return false
}

// skipKeyPressed reports a fresh press of any key that means "get on with it".
func skipKeyPressed() bool {
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		if !serviceKey(k) {
			return true
		}
	}
	return false
}

// skipPressed reports the player asking to move past what is on screen, by
// click or by key. The boot screens and a skippable cutscene share it, so
// Enter works on both.
func skipPressed() bool {
	return clickedThisTick() || skipKeyPressed()
}

// loadScreens fetches the boot images from LOGO.DAT (LOGO = studio logo,
// ROBINSON = title, LOADING = the load break before the intro). Missing packs
// skip straight to the load break.
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
	g.loadingImg = load("LOADING")
	if g.logoImg == nil {
		g.leaveBoot()
		return
	}
	g.mode = modeLogo
	g.modeT = 0
}

// updateScreens advances the boot sequence; returns true while it owns the
// frame (play is paused underneath).
func (g *Game) updateScreens(dt float64) bool {
	if g.mode != modeLogo && g.mode != modeTitle {
		return false
	}
	g.modeT += dt
	skip := skipPressed()
	switch g.mode {
	case modeLogo:
		if skip || g.modeT > 2.5 {
			g.mode, g.modeT = modeTitle, 0
		}
	case modeTitle:
		if skip || g.modeT > 60 {
			g.leaveBoot()
		}
	}
	return true
}

// leaveBoot hands the boot screens over to the main menu, where the original
// started up (ROBY.PDF p.23). Without the menu backdrop there is nothing to
// click, so fall back to the load break: the intro bridge still has to run out
// of sight (it just draws black without an image).
func (g *Game) leaveBoot() {
	if g.optSprites["OPTIONS"] != nil {
		g.openMenu()
		return
	}
	g.enterLoading()
}

// enterLoading raises the loading screen and remembers the scene it is sitting
// out: the intro bridge (INT0) chains onward by itself, so a changed scene name
// is the signal that play can take the frame.
func (g *Game) enterLoading() {
	g.mode, g.modeT = modeLoading, 0
	g.loadingFrom = g.sceneName
}

// updateLoading runs the opening handover out of sight: only the scene objects
// (INT0's own driver script), the fade and the exit it queues tick — no input,
// no walking, no camera. The multi-second decode of the intro movie happens
// inside one of these ticks, and the loading screen is what stays on screen
// through it. Returns true while it owns the frame.
func (g *Game) updateLoading(dt float64) bool {
	if g.mode != modeLoading {
		return false
	}
	g.modeT += dt       // the screen's own fade-in runs on unscaled time
	dt *= 0.5 + g.speed // the hidden run keeps the play pace
	if !g.updateFade(dt) {
		for _, s := range g.sceneObjs {
			g.applyEvents(s.update(dt))
		}
		g.flushPending()
	}
	// The bridge has crossed — or the data is broken and never will, so bail
	// out rather than sit on the loading screen forever.
	if g.sceneName != g.loadingFrom || g.modeT > 30 {
		g.mode, g.started = modePlay, true
	}
	return true
}

// drawScreens renders the current boot screen with a short fade-in.
func (g *Game) drawScreens(screen *ebiten.Image) {
	img := g.logoImg
	switch g.mode {
	case modeTitle:
		img = g.titleImg
	case modeLoading:
		img = g.loadingImg
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

// startFade begins a fade-out that swaps to the pending exit at black and then
// fades the new scene in, using the current scene's own .FAD curve.
func (g *Game) startFade(to *types.Exit) {
	g.fadeCurve = g.res.SceneFade(g.sceneName)
	if len(g.fadeCurve) < 2 {
		// No fade table: swap immediately.
		g.loadScene(to.Scene, &[2]int{to.GX, to.GY}, to.Entry, to.EntryFrid)
		return
	}
	g.fadeTo = to
	g.fadeStep = 0
	g.fadeOut = true
	g.fadeT = 0
}

// updateFade advances an in-progress transition; returns true while one runs.
func (g *Game) updateFade(dt float64) bool {
	if g.fadeCurve == nil {
		return false
	}
	g.fadeT += dt
	for g.fadeT >= fadeStepTime {
		g.fadeT -= fadeStepTime
		switch {
		case g.fadeOut && g.fadeStep+1 < len(g.fadeCurve):
			g.fadeStep++
		case g.fadeOut:
			// Fully black: swap the scene and fade back in.
			to := g.fadeTo
			g.fadeTo = nil
			g.fadeOut = false
			if to != nil {
				g.loadScene(to.Scene, &[2]int{to.GX, to.GY},
					to.Entry, to.EntryFrid)
				if c := g.res.SceneFade(g.sceneName); len(c) >= 2 {
					g.fadeCurve = c
				}
				g.fadeStep = len(g.fadeCurve) - 1
			}
		case g.fadeStep > 0:
			g.fadeStep--
		default:
			g.fadeCurve = nil // fade-in finished
			return false
		}
	}
	return true
}

// drawFade darkens the frame to the current fade step's brightness.
func (g *Game) drawFade(screen *ebiten.Image) {
	if g.fadeCurve == nil || g.fadeStep <= 0 {
		return
	}
	b := g.fadeCurve[g.fadeStep]
	if b >= 1 {
		return
	}
	a := uint8(clampF(1-b, 0, 1) * 255)
	vector.FillRect(screen, 0, 0, float32(ViewW), float32(ViewH),
		rgba(0, 0, 0, a), false)
}

package app

import (
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/types"
)

// The main menu (the manual's "Главное меню игры", doubling as the options
// screen) and the save/load screens come from DATA/OPTIONS.DAT. Every rectangle
// below is the authored placement read from the bitmaps' own headers (the five
// menu rows are OPTS0-4, the slider knob is OPTS5, the save/load buttons are
// BUT1x/BUT2x), so the layout matches the original pixel for pixel.
var (
	// menuRows are the five buttons: new game, continue, load, save, quit.
	menuRows = [5]image.Rectangle{
		image.Rect(214, 62, 450, 96),
		image.Rect(214, 104, 450, 138),
		image.Rect(214, 147, 450, 181),
		image.Rect(214, 190, 450, 224),
		image.Rect(214, 233, 450, 267),
	}
	// sliderTracks are the sound, music and speed capsules; the knob (29px)
	// slides between trackX0 and trackX1.
	sliderTracks = [3]image.Rectangle{
		image.Rect(222, 294, 448, 323),
		image.Rect(222, 356, 448, 385),
		image.Rect(222, 418, 448, 447),
	}
	// saveButton and cancelButton are shared by the save and load screens.
	saveButton   = image.Rect(41, 436, 281, 471)
	cancelButton = image.Rect(362, 436, 602, 471)
)

// The slots are ROBY.EXE's own table (0x46d1f0): twelve 128x96 windows, the
// openings of the frames painted on SAVE.NGB and LOAD.NGB.
const (
	knobW      = 29
	slotCols   = 4
	slotRows   = 3
	slotW      = 128
	slotH      = 96
	slotX0     = 16
	slotY0     = 50
	slotPitchX = 160
	slotPitchY = 130
	slotCount  = slotCols * slotRows

	// slotDblClick is how soon a second press has to follow the first to count
	// as a double click: the 300 ms ROBY.EXE measures with GetTickCount.
	slotDblClick = 0.3
)

// slotRect is the on-screen rectangle of save slot i (0..11).
func slotRect(i int) image.Rectangle {
	c, r := i%slotCols, i/slotCols
	x, y := slotX0+c*slotPitchX, slotY0+r*slotPitchY
	return image.Rect(x, y, x+slotW, y+slotH)
}

// slotScreenSprite reports whether an OPTIONS.DAT bitmap belongs to the
// save/load screens rather than to the menu: the buttons and TEMP, the empty
// slot. Those screens ship their own palette (SAVE.COL, byte for byte the same
// as LOAD.COL and TEMP.COL) and it shares not one of its 256 entries with
// OPTIONS.COL, so a button painted with the menu's palette comes out a grey and
// yellow mess.
func slotScreenSprite(name string) bool {
	return strings.HasPrefix(name, "BUT") || name == "TEMP"
}

// loadOptions loads the options/save/load screens and their widgets.
func (g *Game) loadOptions() {
	// The save/load palette first: the buttons are painted with it, not with
	// the pack's own.
	var slotPal types.Palette
	g.optSprites = map[string]*ebiten.Image{}
	for _, name := range []string{"SAVE", "LOAD"} {
		n, p := g.res.Screen("OPTIONS", name)
		if n == nil {
			continue
		}
		slotPal = p
		img := ebiten.NewImage(n.Width, n.Height)
		rgba := n.RGBA(p)
		for i := 0; i < n.Width*n.Height; i++ {
			rgba[i*4+3] = 255
		}
		img.WritePixels(rgba)
		g.optSprites[name] = img
	}
	sprites, pal := g.res.ScreenPack("OPTIONS")
	for name, n := range sprites {
		if n == nil || g.optSprites[name] != nil {
			continue
		}
		p := pal
		if slotScreenSprite(name) {
			p = slotPal
		}
		img := ebiten.NewImage(n.Width, n.Height)
		rgba := n.RGBA(p)
		if n.Width == ViewW && n.Height == ViewH {
			for i := 0; i < n.Width*n.Height; i++ {
				rgba[i*4+3] = 255 // full-screen backdrops are opaque
			}
		}
		img.WritePixels(rgba)
		g.optSprites[name] = img
	}
	// The empty slot comes twice: as painted, and shaded the way the original
	// draws every slot but the chosen one — through TEMP.FAD.
	if n := sprites["TEMP"]; n != nil {
		dim := ebiten.NewImage(n.Width, n.Height)
		dim.WritePixels(n.RGBA(fadePalette(
			slotPal, g.res.ScreenFile("OPTIONS", "TEMP.FAD"),
		)))
		g.slotEmpty = [2]*ebiten.Image{g.optSprites["TEMP"], dim}
	}
}

// slotShadeRatio is the share of a .FAD table the original shades an unchosen
// slot with (vrtSetFadeRatio 0.4); NGI truncates it to a step.
const slotShadeRatio = 0.4

// fadeStep is the step of an n-step .FAD table the shade lands on: 6 of a
// scene's 16, 12 of TEMP.FAD's 32.
func fadeStep(n int) int { return int(slotShadeRatio * float64(n-1)) }

// fadeShade is how bright that step leaves a picture, read off a scene's fade
// curve. Without a table NGI draws the slot unshaded.
func fadeShade(curve []float64) float64 {
	if len(curve) < 2 {
		return 1
	}
	return curve[fadeStep(len(curve))]
}

// fadePalette is pal as a .FAD table's shade step leaves it: step k sends
// palette index i to fad[k*256+i], which is how the engine darkens an 8-bit
// picture. Without a table the palette stays as it is.
func fadePalette(pal types.Palette, fad []byte) types.Palette {
	n := len(fad) / 256
	if n == 0 {
		return pal
	}
	k := fadeStep(n) * 256
	var out types.Palette
	for i := range out {
		out[i] = pal[fad[k+i]]
	}
	return out
}

// updateOptions runs the options/save/load screens; returns true while one of
// them owns the frame.
func (g *Game) updateOptions(dt float64) bool {
	switch g.mode {
	case modeOptions, modeSave, modeLoad:
	default:
		return false
	}
	m := readMouse(g.aim())
	switch g.mode {
	case modeOptions:
		g.updateOptionsMenu(m.x, m.y, m.clicked)
	case modeSave, modeLoad:
		g.updateSlotScreen(m, dt)
	}
	return true
}

// updateOptionsMenu handles the five menu rows and the three sliders.
func (g *Game) updateOptionsMenu(mx, my int, click bool) {
	g.optHover = -1
	for i, r := range menuRows {
		// Until the first run starts, "continue" (1) and "save" (3) are dead,
		// as in the original (ROBY.PDF p.24): no highlight and no click.
		if !g.started && (i == 1 || i == 3) {
			continue
		}
		if pointIn(r, mx, my) {
			g.optHover = i
		}
	}
	// Dragging a slider keeps following the cursor until the button is up.
	if g.optDrag >= 0 {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			g.optDrag = -1
			g.keepLevels()
		} else {
			g.setSlider(g.optDrag, mx)
		}
		return
	}
	if !click {
		return
	}
	for i, t := range sliderTracks {
		if pointIn(t, mx, my) {
			g.optDrag = i
			g.setSlider(i, mx)
			return
		}
	}
	switch g.optHover {
	case 0: // начать новую игру
		g.restart()
	case 1: // продолжить игру
		g.mode = modePlay
	case 2: // восстановить игру
		g.openSlotScreen(modeLoad)
	case 3: // сохранить игру
		g.openSlotScreen(modeSave)
	case 4: // выход
		g.quit = true
	}
}

// openSlotScreen raises the save or load screen with nothing hovered, no
// button held over from whatever opened it and no press to pair a double click
// with.
func (g *Game) openSlotScreen(mode int) {
	g.mode, g.slotHover, g.btnDown, g.slotDblT = mode, -1, -1, 0
}

// setSlider maps a cursor x inside a track to 0..1 and applies the setting.
func (g *Game) setSlider(i, mx int) {
	t := sliderTracks[i]
	span := float64(t.Dx() - knobW)
	v := (float64(mx) - float64(t.Min.X) - knobW/2) / span
	v = clampF(v, 0, 1)
	switch i {
	case 0:
		g.volSound = v
		g.audio.SetVolume(v)
	case 1:
		g.volMusic = v
		g.audio.SetMusicVolume(v)
	case 2:
		g.speed = v
	}
}

// keepLevels writes the sliders to config.yml once the player lets go of
// one, so the game starts with them next time.
func (g *Game) keepLevels() {
	if err := g.cfg.SaveLevels(g.volSound, g.volMusic, g.speed); err != nil {
		fmt.Fprintf(os.Stderr, "settings: %v\n", err)
	}
}

// keepSwitch writes a setting one of the window's keys flipped (window.go) to
// config.yml, the way keepLevels keeps the sliders. A headless run's keys are
// a script's, no player's choice, and write nothing.
func (g *Game) keepSwitch(key string, on bool) {
	if headless {
		return
	}
	if err := g.cfg.SaveSwitch(key, on); err != nil {
		fmt.Fprintf(os.Stderr, "settings: %v\n", err)
	}
}

// slotButtonRect is the on-screen rectangle of slot-screen button i: 0 acts
// (save or restore), 1 cancels.
func slotButtonRect(i int) image.Rectangle {
	if i == 1 {
		return cancelButton
	}
	return saveButton
}

// slotButtonAt returns the slot-screen button under (x, y), or -1.
func slotButtonAt(x, y int) int {
	for i := 0; i < 2; i++ {
		if pointIn(slotButtonRect(i), x, y) {
			return i
		}
	}
	return -1
}

// slotButtons are the pressed-state bitmaps of the two buttons, per screen.
// Each pack matches its own backdrop shifted a pixel down and right, which is
// the button drawn pushed in: BUT20/BUT21 belong to the save screen,
// BUT10/BUT11 to the load screen.
func slotButtons(mode int) [2]string {
	if mode == modeLoad {
		return [2]string{"BUT10", "BUT11"}
	}
	return [2]string{"BUT20", "BUT21"}
}

// updateSlotScreen handles the twelve save/load thumbnails and the two buttons.
// A button acts on release over the button the press started on — the original
// ships its pushed-in state as a bitmap, and it has to stay on screen for as
// long as the player holds the mouse down. A double click on a slot does not
// wait for the release: the second press saves or restores at once, as the
// original's WM_LBUTTONDOWN handler does (0x406c95, 0x408590).
func (g *Game) updateSlotScreen(m mouseState, dt float64) {
	g.slotHover = -1
	for i := 0; i < slotCount; i++ {
		if pointIn(slotRect(i), m.x, m.y) {
			g.slotHover = i
		}
	}
	g.slotDblT -= dt
	switch {
	case m.clicked:
		dbl := g.slotDblT > 0
		g.slotDblT = slotDblClick
		g.btnDown = slotButtonAt(m.x, m.y)
		if g.slotHover >= 0 {
			g.slotSel = g.slotHover
			if dbl {
				g.pressSlotButton(0) // the same save or restore the button runs
			}
		}
	case m.released:
		btn := g.btnDown
		g.btnDown = -1
		if btn >= 0 && slotButtonAt(m.x, m.y) == btn {
			g.pressSlotButton(btn)
		}
	case !m.pressed:
		g.btnDown = -1 // the press ended somewhere we never saw it
	}
}

// pressSlotButton runs button i of the slot screen.
func (g *Game) pressSlotButton(i int) {
	if i == 1 {
		g.mode = modeOptions
		return
	}
	if g.slotSel < 0 {
		return
	}
	if g.mode == modeSave {
		g.saveSlot(g.slotSel)
		g.mode = modePlay
		return
	}
	if g.loadSlot(g.slotSel) {
		g.mode = modePlay
	}
}

// drawOptions renders whichever menu screen is active.
func (g *Game) drawOptions(screen *ebiten.Image) {
	switch g.mode {
	case modeOptions:
		g.blitOpt(screen, "OPTIONS", 0, 0)
		if g.optHover >= 0 {
			r := menuRows[g.optHover]
			g.blitOpt(
				screen,
				fmt.Sprintf("OPTS%d", g.optHover),
				r.Min.X,
				r.Min.Y,
			)
		}
		for i, v := range [3]float64{g.volSound, g.volMusic, g.speed} {
			t := sliderTracks[i]
			x := t.Min.X + int(v*float64(t.Dx()-knobW))
			g.blitOpt(screen, "OPTS5", x, t.Min.Y)
		}
	case modeSave, modeLoad:
		name := "SAVE"
		if g.mode == modeLoad {
			name = "LOAD"
		}
		g.blitOpt(screen, name, 0, 0)
		g.drawSlots(screen)
		// Both buttons are painted into each backdrop in their raised state;
		// the pack ships the pushed-in one as a bitmap per screen, so it only
		// goes up while the player holds that button down.
		cx, cy := g.aim()
		if btn := g.btnDown; btn >= 0 && slotButtonAt(cx, cy) == btn {
			r := slotButtonRect(btn)
			g.blitOpt(screen, slotButtons(g.mode)[btn], r.Min.X, r.Min.Y)
		}
	}
}

// drawSlots paints the twelve slots the way the original does (0x407392): the
// chosen one as it is, every other one shaded through a .FAD table — a saved
// picture through the scene's, an empty slot's marble (TEMP) through its own.
// There is no frame and no hover mark: the brightness is the selection.
func (g *Game) drawSlots(screen *ebiten.Image) {
	if g.slotDim == 0 {
		g.slotDim = fadeShade(g.res.SceneFade(g.sceneName))
	}
	for i := 0; i < slotCount; i++ {
		r := slotRect(i)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
		chosen := i == g.slotSel
		img := g.slotThumb(i)
		switch {
		case img != nil:
			if !chosen {
				k := float32(g.slotDim)
				op.ColorScale.Scale(k, k, k, 1)
			}
		case chosen:
			img = g.slotEmpty[0]
		default:
			img = g.slotEmpty[1]
		}
		if img == nil {
			continue
		}
		// TEMP is 129x98; the slot window clips it, as the original's does.
		win := img.SubImage(image.Rect(0, 0, slotW, slotH)).(*ebiten.Image)
		screen.DrawImage(win, op)
	}
}

// blitOpt draws a named OPTIONS.DAT bitmap at (x, y).
func (g *Game) blitOpt(screen *ebiten.Image, name string, x, y int) {
	img := g.optSprites[name]
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(img, op)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

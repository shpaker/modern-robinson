package app

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
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

const (
	knobW      = 29
	slotCols   = 4
	slotRows   = 3
	slotW      = 129
	slotH      = 98
	slotX0     = 9
	slotY0     = 45
	slotPitchX = 160
	slotPitchY = 130
	slotCount  = slotCols * slotRows
)

// slotRect is the on-screen rectangle of save slot i (0..11).
func slotRect(i int) image.Rectangle {
	c, r := i%slotCols, i/slotCols
	x, y := slotX0+c*slotPitchX, slotY0+r*slotPitchY
	return image.Rect(x, y, x+slotW, y+slotH)
}

// slotScreenSprite reports whether an OPTIONS.DAT bitmap belongs to the
// save/load screens rather than to the menu. Those screens ship their own
// palette (SAVE.COL, byte for byte the same as LOAD.COL) and it shares not one
// of its 256 entries with OPTIONS.COL, so a button painted with the menu's
// palette comes out a grey and yellow mess.
func slotScreenSprite(name string) bool {
	return strings.HasPrefix(name, "BUT")
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
	g.slotLit, g.slotShade = paletteEdges(slotPal)
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
}

// updateOptions runs the options/save/load screens; returns true while one of
// them owns the frame.
func (g *Game) updateOptions() bool {
	switch g.mode {
	case modeOptions, modeSave, modeLoad:
	default:
		return false
	}
	m := readMouse()
	switch g.mode {
	case modeOptions:
		g.updateOptionsMenu(m.x, m.y, m.clicked)
	case modeSave, modeLoad:
		g.updateSlotScreen(m)
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

// openSlotScreen raises the save or load screen with nothing hovered and no
// button held over from whatever opened it.
func (g *Game) openSlotScreen(mode int) {
	g.mode, g.slotHover, g.btnDown = mode, -1, -1
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
// long as the player holds the mouse down.
func (g *Game) updateSlotScreen(m mouseState) {
	g.slotHover = -1
	for i := 0; i < slotCount; i++ {
		if pointIn(slotRect(i), m.x, m.y) {
			g.slotHover = i
		}
	}
	switch {
	case m.clicked:
		g.btnDown = slotButtonAt(m.x, m.y)
		if g.slotHover >= 0 {
			g.slotSel = g.slotHover
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
		cx, cy := ebiten.CursorPosition()
		if btn := g.btnDown; btn >= 0 && slotButtonAt(cx, cy) == btn {
			r := slotButtonRect(btn)
			g.blitOpt(screen, slotButtons(g.mode)[btn], r.Min.X, r.Min.Y)
		}
	}
}

// drawSlots paints each slot's thumbnail (or its scene caption) and marks the
// hovered and selected ones.
func (g *Game) drawSlots(screen *ebiten.Image) {
	for i := 0; i < slotCount; i++ {
		r := slotRect(i)
		if th := g.slotThumb(i); th != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
			screen.DrawImage(th, op)
		}
		if meta := g.slotMeta(i); meta != "" {
			adapters.DrawText(screen, meta, float64(r.Min.X+4),
				float64(r.Max.Y-adapters.FontH-2), rgba(30, 24, 16, 255))
		}
		// The screens speak in bevels, so the slots do too: the hovered one
		// stands out raised, the chosen one sits pushed in, and deeper, so the
		// two never read alike.
		switch i {
		case g.slotSel:
			g.bevelRect(screen, r, 3, true)
		case g.slotHover:
			g.bevelRect(screen, r, 2, false)
		}
	}
}

// bevelRect frames a rectangle the way the screens' own buttons are shaded:
// t pixels of the palette's light tone along the top and left edges and its
// dark one along the others, swapped when sunken.
func (g *Game) bevelRect(
	dst *ebiten.Image, r image.Rectangle, t float32, sunken bool,
) {
	top, bottom := g.slotLit, g.slotShade
	if sunken {
		top, bottom = bottom, top
	}
	x, y := float32(r.Min.X), float32(r.Min.Y)
	w, h := float32(r.Dx()), float32(r.Dy())
	vector.FillRect(dst, x, y, w, t, top, false)
	vector.FillRect(dst, x, y, t, h, top, false)
	vector.FillRect(dst, x, y+h-t, w, t, bottom, false)
	vector.FillRect(dst, x+w-t, y, t, h, bottom, false)
}

// paletteEdges picks the two tones a slot frame is drawn with, so a frame we
// add ourselves stays inside the screen's own range: SAVE.COL (which LOAD.COL
// repeats byte for byte) is all sepia and gold, with not one red entry and
// four greys in 256. Not the very ends of that range, though — those are pure
// white and pure black, and either reads as a scratch on the page.
func paletteEdges(p types.Palette) (lit, shade color.Color) {
	tones := make([][4]byte, len(p))
	copy(tones, p[:])
	sort.Slice(tones, func(i, j int) bool {
		return luma(tones[i]) < luma(tones[j])
	})
	pick := func(percent int) color.Color {
		c := tones[percent*(len(tones)-1)/100]
		return rgba(c[0], c[1], c[2], 255)
	}
	return pick(90), pick(15)
}

// luma weighs a palette entry roughly the way the eye does.
func luma(c [4]byte) int { return int(c[0])*3 + int(c[1])*6 + int(c[2]) }

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

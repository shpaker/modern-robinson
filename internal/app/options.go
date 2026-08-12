package app

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
)

// The options, save and load screens come from DATA/OPTIONS.DAT. Every rectangle
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

// loadOptions loads the options/save/load screens and their widgets.
func (g *Game) loadOptions() {
	sprites, pal := g.res.ScreenPack("OPTIONS")
	g.optSprites = map[string]*ebiten.Image{}
	for name, n := range sprites {
		if n == nil {
			continue
		}
		img := ebiten.NewImage(n.Width, n.Height)
		rgba := n.RGBA(pal)
		if n.Width == ViewW && n.Height == ViewH {
			for i := 0; i < n.Width*n.Height; i++ {
				rgba[i*4+3] = 255 // full-screen backdrops are opaque
			}
		}
		img.WritePixels(rgba)
		g.optSprites[name] = img
	}
	// SAVE/LOAD ship their own palettes.
	for _, name := range []string{"SAVE", "LOAD"} {
		if n, p := g.res.Screen("OPTIONS", name); n != nil {
			img := ebiten.NewImage(n.Width, n.Height)
			rgba := n.RGBA(p)
			for i := 0; i < n.Width*n.Height; i++ {
				rgba[i*4+3] = 255
			}
			img.WritePixels(rgba)
			g.optSprites[name] = img
		}
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
	mx, my := ebiten.CursorPosition()
	click := clickedThisTick()
	switch g.mode {
	case modeOptions:
		g.updateOptionsMenu(mx, my, click)
	case modeSave, modeLoad:
		g.updateSlotScreen(mx, my, click)
	}
	return true
}

// updateOptionsMenu handles the five menu rows and the three sliders.
func (g *Game) updateOptionsMenu(mx, my int, click bool) {
	g.optHover = -1
	for i, r := range menuRows {
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
		g.mode, g.slotHover = modeLoad, -1
	case 3: // сохранить игру
		g.mode, g.slotHover = modeSave, -1
	case 4: // выход
		g.quit = true
	}
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

// updateSlotScreen handles the twelve save/load thumbnails and the two buttons.
func (g *Game) updateSlotScreen(mx, my int, click bool) {
	g.slotHover = -1
	for i := 0; i < slotCount; i++ {
		if pointIn(slotRect(i), mx, my) {
			g.slotHover = i
		}
	}
	if !click {
		return
	}
	if pointIn(cancelButton, mx, my) {
		g.mode = modeOptions
		return
	}
	if g.slotHover >= 0 {
		g.slotSel = g.slotHover
	}
	if pointIn(saveButton, mx, my) && g.slotSel >= 0 {
		if g.mode == modeSave {
			g.saveSlot(g.slotSel)
			g.mode = modePlay
		} else if g.loadSlot(g.slotSel) {
			g.mode = modePlay
		}
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
		// Both buttons are painted into each backdrop already (the left one
		// reads "Сохранить" on the save screen and "Загрузить" on the load
		// screen). BUT2x are their pressed states — usable only where the
		// label matches, so the load screen gets a highlight frame instead.
		cx, cy := ebiten.CursorPosition()
		for _, b := range []struct {
			r      image.Rectangle
			sprite string
		}{{saveButton, "BUT20"}, {cancelButton, "BUT21"}} {
			if !pointIn(b.r, cx, cy) {
				continue
			}
			if g.mode == modeSave || b.sprite == "BUT21" {
				g.blitOpt(screen, b.sprite, b.r.Min.X, b.r.Min.Y)
			} else {
				strokeRect(screen, b.r, 2, rgba(255, 230, 80, 255))
			}
		}
	}
	g.drawCursor(screen)
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
		switch i {
		case g.slotSel:
			strokeRect(screen, r, 3, rgba(200, 40, 30, 255))
		case g.slotHover:
			strokeRect(screen, r, 2, rgba(255, 230, 80, 255))
		}
	}
}

// strokeRect outlines a rectangle in the given colour.
func strokeRect(
	dst *ebiten.Image, r image.Rectangle, w float32, c color.Color,
) {
	vector.StrokeRect(dst, float32(r.Min.X), float32(r.Min.Y),
		float32(r.Dx()), float32(r.Dy()), w, c, false)
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

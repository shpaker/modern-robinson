package app

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
)

// loadBar loads the panel layout (BAR.BAR), its bitmaps (BAR*.NGB), and the
// global text table. Called once at start-up; the bar is scene-independent.
func (g *Game) loadBar() {
	g.itemIcons = map[string]*ebiten.Image{}
	g.barSprites = map[string]*ebiten.Image{}
	if c := g.res.SceneContainer("BAR"); c != nil {
		if d, err := c.ExtractName("BAR.BAR"); err == nil {
			g.bar = g.parser.ParseBar(string(d))
		}
	}
	sprites, pal := g.res.BarSprites()
	for name, n := range sprites {
		img := ebiten.NewImage(n.Width, n.Height)
		img.WritePixels(n.RGBA(pal))
		g.barSprites[name] = img
	}
	if bg := g.barSprites["BAR0"]; bg != nil {
		g.barBG = bg
	}
	g.texts = g.res.Texts()
}

// textLine returns the global text-table line for a 0-based id, or "".
func (g *Game) textLine(id int) string {
	if id < 0 || id >= len(g.texts) {
		return ""
	}
	return strings.TrimSpace(g.texts[id])
}

// itemCell returns the bar's item cell size, defaulting if BAR.BAR is absent.
func (g *Game) itemCell() (int, int) {
	if g.bar != nil && g.bar.ItemW > 0 {
		return g.bar.ItemW, g.bar.ItemH
	}
	return 48, 60
}

// itemIndex returns the item's position in BAR.BAR's declared order, or -1.
func (g *Game) itemIndex(name string) int {
	if g.bar == nil {
		return -1
	}
	for i, it := range g.bar.Items {
		if strings.EqualFold(it, name) {
			return i
		}
	}
	return -1
}

// itemIcon returns the icon for an item and whether it is the authored one:
// BAR6 onwards holds a normal/selected pair per declared item. Items the panel
// ships no pair for fall back to their world sprite fitted into the cell, and
// drawBar marks those as selected itself.
func (g *Game) itemIcon(name string, selected bool) (*ebiten.Image, bool) {
	if idx := g.itemIndex(name); idx >= 0 {
		n := 6 + 2*idx
		if selected {
			n++
		}
		if ic := g.barSprites["BAR"+strconv.Itoa(n)]; ic != nil {
			return ic, true
		}
	}
	key := strings.ToLower(name)
	if ic, ok := g.itemIcons[key]; ok {
		return ic, false
	}
	w, h := g.itemCell()
	ic := adapters.LoadIcon(g.res, name+".mv", w, h)
	g.itemIcons[key] = ic
	return ic, false
}

// portrait picks the character button: solo Roby before Friday joins, else the
// two-headed button highlighting the active character.
func (g *Game) portrait() *ebiten.Image {
	if g.gs.Var("FridIs") != 1 {
		return g.barSprites["BAR3"]
	}
	if strings.EqualFold(g.gs.ActiveChar, "Frid") {
		return g.barSprites["BAR2"]
	}
	return g.barSprites["BAR1"]
}

// drawBar renders the inventory panel: background, portrait, the text box
// (dialogue line or hovered object name), the visible inventory icons and the
// panel buttons.
// SetBar OFF (cutscenes) hides it; dialogue then overlays the scene bottom.
func (g *Game) drawBar(screen *ebiten.Image) {
	if !g.gs.UI["bar"] {
		if g.msg != "" {
			w := adapters.TextWidth(g.msg)
			x := (float64(ViewW) - w) / 2
			vector.FillRect(
				screen,
				float32(x-8),
				float32(ViewH-30),
				float32(w+16),
				20,
				rgba(0, 0, 0, 190),
				false,
			)
			adapters.DrawText(
				screen,
				g.msg,
				x,
				float64(ViewH-26),
				rgba(255, 255, 255, 255),
			)
		}
		return
	}
	vector.FillRect(
		screen,
		0,
		float32(PlayH),
		float32(ViewW),
		float32(BarH),
		rgba(24, 18, 12, 255),
		false,
	)
	if g.barBG != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, float64(PlayH))
		screen.DrawImage(g.barBG, op)
	}
	if g.bar == nil {
		return
	}
	if p := g.portrait(); p != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(g.bar.CharBox[0]), float64(g.bar.CharBox[1]))
		screen.DrawImage(p, op)
	}
	g.drawTextBox(screen)
	g.drawItems(screen)
	g.drawButtons(screen)
}

// drawItems fills the inventory window with the scrolled-to icons.
func (g *Game) drawItems(screen *ebiten.Image) {
	ix, iy := g.bar.Inventory[0], g.bar.Inventory[1]
	iw, ih := g.itemCell()
	for i := 0; i < g.bar.ItemsShown; i++ {
		idx := g.invScroll + i
		if idx >= len(g.gs.Inventory) {
			break
		}
		item := g.gs.Inventory[idx]
		sel := strings.EqualFold(g.gs.Active, item)
		x := float64(ix + i*iw)
		icon, authored := g.itemIcon(item, sel)
		if icon == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x, float64(iy))
		screen.DrawImage(icon, op)
		if sel && !authored {
			vector.StrokeRect(
				screen,
				float32(x),
				float32(iy),
				float32(iw),
				float32(ih),
				2,
				rgba(200, 40, 30, 255),
				false,
			)
		}
	}
}

// barOverlay is a panel bitmap that goes on top of the strip background at the
// left-top its layout box gives it.
type barOverlay struct {
	name string
	x, y int
}

// barOverlays lists what the strip background leaves out, in draw order: the
// frame that trims the inventory window, a lit arrow on whichever side has
// something to scroll to (the background prints both of them dimmed), the map
// button and the save disk. The two buttons sit in cut-outs of the background,
// so skipping them leaves a hole in the strip instead of a button.
func (g *Game) barOverlays() []barOverlay {
	out := []barOverlay{{"BAR5", g.bar.InvMask[0], g.bar.InvMask[1]}}
	if g.invScroll > 0 {
		out = append(out, barOverlay{
			"BAR78", g.bar.LeftArrow[0], g.bar.LeftArrow[1],
		})
	}
	if g.invScroll+g.bar.ItemsShown < len(g.gs.Inventory) {
		// The right arrow hugs the far edge of its box, as its print does.
		out = append(out, barOverlay{
			"BAR81",
			g.bar.RightArrow[2] - g.spriteW("BAR81"),
			g.bar.RightArrow[1],
		})
	}
	mapButton := "BAR74" // blank until the island map opens (SetMap ON)
	if g.gs.UI["map"] {
		mapButton = "BAR72"
	}
	return append(out,
		barOverlay{mapButton, g.bar.ScisorsBox[0], g.bar.ScisorsBox[1]},
		barOverlay{"BAR75", g.bar.SaveBox[0], g.bar.SaveBox[1]},
	)
}

// drawButtons paints the overlays the layout asks for.
func (g *Game) drawButtons(screen *ebiten.Image) {
	for _, o := range g.barOverlays() {
		img := g.barSprites[o.name]
		if img == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(o.x), float64(o.y))
		screen.DrawImage(img, op)
	}
}

// spriteW is the width of a panel bitmap, 0 when the panel ships none.
func (g *Game) spriteW(name string) int {
	if img := g.barSprites[name]; img != nil {
		return img.Bounds().Dx()
	}
	return 0
}

// drawTextBox writes the current dialogue line (Text event) or, when idle, the
// hovered object's name into the panel text box, word-wrapped and centred.
func (g *Game) drawTextBox(screen *ebiten.Image) {
	s := g.msg
	if s == "" {
		s = g.hover
	}
	if s == "" {
		return
	}
	b := g.bar.TextBox
	w := float64(b[2] - b[0])
	lines := adapters.WrapText(s, w-8, 3)
	total := len(lines) * adapters.FontH
	y := float64(b[1]) + (float64(b[3]-b[1])-float64(total))/2
	for _, ln := range lines {
		x := float64(b[0]) + (w-adapters.TextWidth(ln))/2
		adapters.DrawText(screen, ln, x, y, rgba(20, 16, 12, 255))
		y += adapters.FontH
	}
}

// clickBar handles a click in the bar: pick an inventory item (SetActive) or
// operate the scroll arrows.
func (g *Game) clickBar(mx, my int) {
	if g.bar == nil {
		return
	}
	switch {
	case inBox(g.bar.CharBox, mx, my):
		// The portrait toggles the controlled character once Friday joined.
		if g.gs.Var("FridIs") == 1 {
			if strings.EqualFold(g.gs.ActiveChar, "Frid") {
				g.gs.ActiveChar = "Roby"
			} else {
				g.gs.ActiveChar = "Frid"
			}
		}
		return
	case inBox(g.bar.LeftArrow, mx, my):
		if g.invScroll > 0 {
			g.invScroll--
		}
		return
	case inBox(g.bar.RightArrow, mx, my):
		if g.invScroll+g.bar.ItemsShown < len(g.gs.Inventory) {
			g.invScroll++
		}
		return
	case inBox(g.bar.SaveBox, mx, my):
		// The disk button opens the authored save screen.
		g.mode, g.slotHover = modeSave, -1
		return
	case inBox(g.bar.ScisorsBox, mx, my):
		// The map button: enabled once the island map opens (SetMap ON). Its
		// command is the one ROBY.EXE keeps next to the layout keys — GoScene
		// MAPSCR, Roby, Roin0, Frid, Frin0, 0,0 — and those two entries are what
		// makes the map a map: they park both characters at (7,0), off the far
		// side of the stage, and switch the button back off.
		if g.gs.UI["map"] && !strings.EqualFold(g.sceneName, "MAPSCR") {
			g.pending = &types.Exit{
				Scene: "MAPSCR", Entry: "Roin0", EntryFrid: "Frin0", OK: true,
			}
		}
		return
	}
	ix, iy := g.bar.Inventory[0], g.bar.Inventory[1]
	iw, ih := g.itemCell()
	for i := 0; i < g.bar.ItemsShown; i++ {
		idx := g.invScroll + i
		if idx >= len(g.gs.Inventory) {
			break
		}
		if pointIn(image.Rect(ix+i*iw, iy, ix+i*iw+iw, iy+ih), mx, my) {
			g.gs.Active = g.gs.Inventory[idx]
			return
		}
	}
}

// inBox reports whether (x,y) is inside an x0,y0,x1,y1 box.
func inBox(b [4]int, x, y int) bool {
	return x >= b[0] && x < b[2] && y >= b[1] && y < b[3]
}

// updateHover refreshes the hovered-object caption (object Text id -> table).
func (g *Game) updateHover(mx, my int) {
	g.hover = ""
	if my >= PlayH {
		return
	}
	wx, wy := mx+g.camX, my
	if hs := g.hotspotAt(wx, wy); hs != nil {
		g.hover = g.textLine(hs.ob.Text)
	}
}

var _ = types.Command{} // keep types import for future bar events

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

// itemIcon returns the icon for an item: the authored BAR pair for the first
// seven declared items (normal/selected), else the item's world sprite fitted
// into the cell (selection drawn as a red frame by drawBar).
func (g *Game) itemIcon(name string, selected bool) *ebiten.Image {
	idx := g.itemIndex(name)
	if idx >= 0 && idx < 7 {
		n := 6 + 2*idx
		if selected {
			n++
		}
		if ic := g.barSprites["BAR"+strconv.Itoa(n)]; ic != nil {
			return ic
		}
	}
	key := strings.ToLower(name)
	if ic, ok := g.itemIcons[key]; ok {
		return ic
	}
	w, h := g.itemCell()
	ic := adapters.LoadIcon(g.res, name+".mv", w, h)
	g.itemIcons[key] = ic
	return ic
}

// portrait picks the character button: solo Roby before Friday joins, else the
// two-headed button highlighting the active character.
func (g *Game) portrait() *ebiten.Image {
	if g.gs.Var("FridIs") != 1 {
		return g.barSprites["BAR3"]
	}
	if strings.EqualFold(g.gs.Active, "Frid") {
		return g.barSprites["BAR2"]
	}
	return g.barSprites["BAR1"]
}

// drawBar renders the inventory panel: background, portrait, the text box
// (dialogue line or hovered object name), and the visible inventory icons.
func (g *Game) drawBar(screen *ebiten.Image) {
	vector.FillRect(screen, 0, float32(PlayH), float32(ViewW), float32(BarH), rgba(24, 18, 12, 255), false)
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
		if icon := g.itemIcon(item, sel); icon != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(x, float64(iy))
			screen.DrawImage(icon, op)
			if sel && g.itemIndex(item) >= 7 {
				vector.StrokeRect(screen, float32(x), float32(iy), float32(iw), float32(ih),
					2, rgba(200, 40, 30, 255), false)
			}
		}
	}
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
	for _, hs := range g.hotspots {
		if pointIn(hs.rect, wx, wy) {
			g.hover = g.textLine(hs.ob.Text)
			return
		}
	}
}

var _ = types.Command{} // keep types import for future bar events

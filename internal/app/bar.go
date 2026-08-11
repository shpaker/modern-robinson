package app

import (
	"image"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/shpaker/modern-robinson/internal/adapters"
)

// loadBar loads the inventory-panel layout (BAR.BAR) and its background bitmap
// (BAR0.NGB). Called once at start-up; the bar is the same across scenes.
func (g *Game) loadBar() {
	g.itemIcons = map[string]*ebiten.Image{}
	if c := g.res.SceneContainer("BAR"); c != nil {
		if d, err := c.ExtractName("BAR.BAR"); err == nil {
			g.bar = g.parser.ParseBar(string(d))
		}
	}
	if ngb, pal := g.res.BarBackground(); ngb != nil {
		img := ebiten.NewImage(ngb.Width, ngb.Height)
		img.WritePixels(ngb.RGBA(pal))
		g.barBG = img
	}
}

// itemCell returns the bar's item cell size, defaulting if BAR.BAR is absent.
func (g *Game) itemCell() (int, int) {
	if g.bar != nil && g.bar.ItemW > 0 {
		return g.bar.ItemW, g.bar.ItemH
	}
	return 48, 60
}

// itemIcon returns (caching, including misses) the inventory icon for an item:
// its world sprite <item>.mv fitted into the bar's item cell.
func (g *Game) itemIcon(name string) *ebiten.Image {
	if ic, ok := g.itemIcons[name]; ok {
		return ic
	}
	w, h := g.itemCell()
	ic := adapters.LoadIcon(g.res, name+".mv", w, h)
	g.itemIcons[name] = ic
	return ic
}

// drawBar renders the inventory panel over the bottom BarH px: background, then
// the visible inventory item icons with the active item highlighted.
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
	ix, iy := g.bar.Inventory[0], g.bar.Inventory[1]
	iw, ih := g.itemCell()
	for i := 0; i < g.bar.ItemsShown; i++ {
		idx := g.invScroll + i
		if idx >= len(g.gs.Inventory) {
			break
		}
		item := g.gs.Inventory[idx]
		x := float64(ix + i*iw)
		if icon := g.itemIcon(item); icon != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(x, float64(iy))
			screen.DrawImage(icon, op)
		}
		if strings.EqualFold(g.gs.Active, item) {
			vector.StrokeRect(screen, float32(x), float32(iy), float32(iw), float32(ih), 2, rgba(255, 230, 80, 255), false)
		}
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

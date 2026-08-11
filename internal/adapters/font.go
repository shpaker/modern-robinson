package adapters

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/bitmapfont/v4"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// face is the bitmap UI font (12px, full Cyrillic coverage).
var face = text.NewGoXFace(bitmapfont.Face)

// FontH is the line height of the UI font.
const FontH = 12

// DrawText draws s at (x, y) in the given color.
func DrawText(dst *ebiten.Image, s string, x, y float64, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	text.Draw(dst, s, face, op)
}

// TextWidth measures s in pixels.
func TextWidth(s string) float64 {
	w, _ := text.Measure(s, face, 0)
	return w
}

// WrapText greedily wraps s by words to fit width px, returning at most
// maxLines lines (the last line is ellipsised if the text overflows).
func WrapText(s string, width float64, maxLines int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		cand := w
		if cur != "" {
			cand = cur + " " + w
		}
		if TextWidth(cand) <= width || cur == "" {
			cur = cand
			continue
		}
		lines = append(lines, cur)
		cur = w
		if len(lines) == maxLines {
			break
		}
	}
	if len(lines) < maxLines && cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == maxLines && cur != "" && lines[maxLines-1] != cur {
		lines[maxLines-1] += "…"
	}
	return lines
}

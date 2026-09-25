package mcp

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// gridStep is how far apart the grid's lines run, in the picture's pixels.
const gridStep = 40

// How opaque the grid is: a line lets the fine detail under it show through,
// a label is to be read on anything.
const (
	gridLineA  = 0x90
	gridLabelA = 0xE0
)

// withGrid is the picture with the coordinate grid over it, for the client's
// eyes only: the game's window never has it. A picture that will not take
// the grid goes as it came.
func withGrid(pic []byte) []byte {
	src, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return pic
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	drawGrid(img)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return pic
	}
	return buf.Bytes()
}

// drawGrid lays a line every gridStep pixels over img, a picture from (0,0)
// as a PNG decodes, each signed with its coordinate: x along the top edge, y
// along the left one. Lines and labels are light with a dark rim, so they
// show on any ground; the pixels away from them stay as they were.
func drawGrid(img *image.RGBA) {
	b := img.Bounds()
	light := image.NewAlpha(b)
	line := image.NewUniform(color.Alpha{A: gridLineA})
	pen := &font.Drawer{
		Dst:  light,
		Src:  image.NewUniform(color.Alpha{A: gridLabelA}),
		Face: basicfont.Face7x13,
	}
	// A digit takes rows 2..10 of its 13, from Dot.Y-9 to Dot.Y-1.
	for x := gridStep; x < b.Max.X; x += gridStep {
		draw.Draw(light, image.Rect(x, 0, x+1, b.Max.Y), line,
			image.Point{}, draw.Src)
		pen.Dot = fixed.P(x+3, 11)
		pen.DrawString(strconv.Itoa(x))
	}
	for y := gridStep; y < b.Max.Y; y += gridStep {
		draw.Draw(light, image.Rect(0, y, b.Max.X, y+1), line,
			image.Point{}, draw.Src)
		pen.Dot = fixed.P(2, y+12)
		pen.DrawString(strconv.Itoa(y))
	}
	draw.DrawMask(img, b, image.Black, image.Point{}, rim(light), b.Min,
		draw.Over)
	draw.DrawMask(img, b, image.White, image.Point{}, light, b.Min, draw.Over)
}

// rim is the one-pixel ring around a mask, as dense as what it borders.
func rim(m *image.Alpha) *image.Alpha {
	b := m.Bounds()
	out := image.NewAlpha(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if m.AlphaAt(x, y).A != 0 {
				continue
			}
			var a uint8
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					// Off the picture AlphaAt reads nothing.
					a = max(a, m.AlphaAt(x+dx, y+dy).A)
				}
			}
			out.SetAlpha(x, y, color.Alpha{A: a})
		}
	}
	return out
}

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

// gridStep is how far apart the grid's lines run, in the picture's pixels,
// on a picture shown as it came — and how near to each other they may come
// on an enlarged one.
const gridStep = 40

// How opaque the grid is: a line lets the fine detail under it show through,
// a label is to be read on anything.
const (
	gridLineA  = 0x90
	gridLabelA = 0xE0
)

// The labels run along the top and the left edge, and a line nearer to that
// edge would run through them: it is left out. The x labels take the top
// rows of the 13-px face and their rim, the y labels up to three 7-px digits
// from x=2 and theirs.
const (
	labelRows = 16
	labelCols = 32
)

// view is how a picture shows the screen: from the screen's point at on,
// each of the screen's pixels is a scale×scale block of the picture — the
// screen's (x, y) is the block from ((x-at.X)·scale, (y-at.Y)·scale).
type view struct {
	at    image.Point
	scale int
}

// asIs is the view of a picture as it came: the screen pixel for pixel.
var asIs = view{scale: 1}

// gridStepAt is the grid's step, in the screen's pixels, on a picture that
// enlarges the screen scale times: the finest step that still runs the lines
// gridStep of the picture's pixels apart or more — room for a label between
// them. It divides gridStep, so every line of the plain grid stays a line.
func gridStepAt(scale int) int {
	step := gridStep
	for d := gridStep - 1; d > 0; d-- {
		if gridStep%d == 0 && d*scale >= gridStep {
			step = d
		}
	}
	return step
}

// withGrid is the picture with the coordinate grid over it, for the client's
// eyes only: the game's window never has it. A picture that will not take
// the grid goes as it came.
func withGrid(pic []byte) []byte {
	img, err := decodeRGBA(pic)
	if err != nil {
		return pic
	}
	drawGrid(img, asIs)
	out, err := encodePNG(img)
	if err != nil {
		return pic
	}
	return out
}

// drawGrid lays the coordinate grid over img, a picture from (0,0) as a PNG
// decodes, that shows the screen through v: a line every gridStepAt(v.scale)
// of the screen's pixels, each signed with its screen coordinate — x along
// the top edge, y along the left one; a label the picture would cut short is
// left out, its line stays. Lines and labels are light with a dark rim, so
// they show on any ground; the pixels away from them stay as they were.
func drawGrid(img *image.RGBA, v view) {
	b := img.Bounds()
	step := gridStepAt(v.scale)
	light := image.NewAlpha(b)
	line := image.NewUniform(color.Alpha{A: gridLineA})
	pen := &font.Drawer{
		Dst:  light,
		Src:  image.NewUniform(color.Alpha{A: gridLabelA}),
		Face: basicfont.Face7x13,
	}
	// A digit takes rows 2..10 of its 13, from Dot.Y-9 to Dot.Y-1.
	sign := func(n, x, y int) {
		s := strconv.Itoa(n)
		if x+pen.MeasureString(s).Ceil() > b.Max.X || y > b.Max.Y {
			return
		}
		pen.Dot = fixed.P(x, y)
		pen.DrawString(s)
	}
	// The first line is the first one past the corner the picture shows.
	for x := v.at.X/step*step + step; ; x += step {
		px := (x - v.at.X) * v.scale
		if px >= b.Max.X {
			break
		}
		if px < labelCols {
			continue
		}
		draw.Draw(light, image.Rect(px, 0, px+1, b.Max.Y), line,
			image.Point{}, draw.Src)
		sign(x, px+3, 11)
	}
	for y := v.at.Y/step*step + step; ; y += step {
		py := (y - v.at.Y) * v.scale
		if py >= b.Max.Y {
			break
		}
		if py < labelRows {
			continue
		}
		draw.Draw(light, image.Rect(0, py, b.Max.X, py+1), line,
			image.Point{}, draw.Src)
		sign(y, 2, py+12)
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

// decodeRGBA reads a PNG into pixels to draw on.
func decodeRGBA(pic []byte) (*image.RGBA, error) {
	src, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil, err
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	return img, nil
}

// encodePNG is the pixels as a PNG again.
func encodePNG(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

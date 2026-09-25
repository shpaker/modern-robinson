package mcp

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// flat is a PNG of one colour all over.
func flat(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// unpack reads a PNG back into pixels.
func unpack(t *testing.T, pic []byte) *image.RGBA {
	t.Helper()
	src, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		t.Fatalf("no PNG: %v", err)
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, image.Point{}, draw.Src)
	return img
}

// nearLine is a row or a column the grid's line or its rim takes.
func nearLine(v int) bool {
	return v >= gridStep-1 && (v+1)%gridStep <= 2
}

var grounds = map[string]color.RGBA{
	"black": {A: 0xFF},
	"white": {R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	"grey":  {R: 0x80, G: 0x80, B: 0x80, A: 0xFF},
	"red":   {R: 0xC0, G: 0x20, B: 0x20, A: 0xFF},
}

// The grid runs a line every 40 px across the whole picture, light with a
// dark rim, so it shows on any ground; away from the lines and their labels
// the picture keeps every pixel it had.
func TestGridLinesShowOnAnyGround(t *testing.T) {
	for name, ground := range grounds {
		got := unpack(t, withGrid(flat(t, 640, 480, ground)))
		if got.Bounds() != image.Rect(0, 0, 640, 480) {
			t.Fatalf("%s: bounds = %v", name, got.Bounds())
		}
		shows := func(pts ...image.Point) bool {
			for _, p := range pts {
				if got.RGBAAt(p.X, p.Y) != ground {
					return true
				}
			}
			return false
		}
		for x := gridStep; x < 640; x += gridStep {
			for _, y := range []int{20, 250, 475} {
				if !shows(image.Pt(x-1, y), image.Pt(x, y), image.Pt(x+1, y)) {
					t.Errorf("%s: no line at x=%d, y=%d", name, x, y)
				}
			}
		}
		for y := gridStep; y < 480; y += gridStep {
			for _, x := range []int{30, 300, 635} {
				if !shows(image.Pt(x, y-1), image.Pt(x, y), image.Pt(x, y+1)) {
					t.Errorf("%s: no line at y=%d, x=%d", name, y, x)
				}
			}
		}
		touched := 0
		for y := 16; y < 480; y++ {
			for x := 26; x < 640; x++ {
				if !nearLine(x) && !nearLine(y) && shows(image.Pt(x, y)) {
					touched++
				}
			}
		}
		if touched > 0 {
			t.Errorf("%s: %d pixels between the lines changed", name, touched)
		}
	}
	grey := grounds["grey"]
	got := unpack(t, withGrid(flat(t, 640, 480, grey)))
	if c := got.RGBAAt(40, 250); c.R <= grey.R {
		t.Errorf("the line is no lighter than the ground: %v", c)
	}
	for _, x := range []int{39, 41} {
		if c := got.RGBAAt(x, 250); c.R >= grey.R {
			t.Errorf("the rim at x=%d is no darker than the ground: %v", x, c)
		}
	}
	if c := got.RGBAAt(40, 250); c.R == 0xFF {
		t.Errorf("the line hides what is under it: %v", c)
	}
}

// signed says the label of number n is drawn on got with its pen at (x0,
// y0): the label lights just the glyphs of that number, as the same face
// draws it apart, and leaves the rest of its place as the ground was.
func signed(
	t *testing.T, got *image.RGBA, ground color.RGBA, n, x0, y0 int,
) bool {
	t.Helper()
	text := strconv.Itoa(n)
	want := image.NewAlpha(got.Bounds())
	pen := &font.Drawer{
		Dst: want, Src: image.Opaque, Face: basicfont.Face7x13,
		Dot: fixed.P(x0, y0),
	}
	pen.DrawString(text)
	ink := image.Rectangle{}
	for y := y0 - 13; y < y0+2; y++ {
		for x := x0; x < x0+7*len(text); x++ {
			if want.AlphaAt(x, y).A != 0 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if ink.Empty() {
		t.Fatalf("%q draws nothing", text)
	}
	// The label's place: its ink with the rim around, and room for one
	// digit more, so a longer number shows too — as far as the picture has
	// it: a label may end on its last row.
	place := ink.Inset(-1)
	place.Max.X += 7
	place = place.Intersect(got.Bounds())
	inked := func(x, y int) bool {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if want.AlphaAt(x+dx, y+dy).A != 0 {
					return true
				}
			}
		}
		return false
	}
	for y := place.Min.Y; y < place.Max.Y; y++ {
		for x := place.Min.X; x < place.Max.X; x++ {
			c := got.RGBAAt(x, y)
			switch {
			case want.AlphaAt(x, y).A != 0:
				if c.R <= ground.R {
					return false
				}
			case !inked(x, y):
				if c != ground {
					return false
				}
			}
		}
	}
	return true
}

// Every line is signed with its own coordinate — x along the top, y down
// the left.
func TestGridSignsItsLines(t *testing.T) {
	ground := grounds["grey"]
	got := unpack(t, withGrid(flat(t, 640, 480, ground)))
	for x := gridStep; x < 640; x += gridStep {
		if !signed(t, got, ground, x, x+3, 11) {
			t.Errorf("the line x=%d is not signed %d", x, x)
		}
	}
	for y := gridStep; y < 480; y += gridStep {
		if !signed(t, got, ground, y, 2, y+12) {
			t.Errorf("the line y=%d is not signed %d", y, y)
		}
	}
}

// The grid fits the picture it is laid on: a scene shorter than the screen
// gets its lines as far as it reaches, and what is no PNG goes as it came.
func TestGridFitsThePicture(t *testing.T) {
	got := unpack(t, withGrid(flat(t, 640, 400, grounds["grey"])))
	if got.Bounds() != image.Rect(0, 0, 640, 400) {
		t.Errorf("bounds = %v", got.Bounds())
	}
	if got.RGBAAt(300, 360) == grounds["grey"] {
		t.Error("no line at y=360")
	}
	if raw := []byte("\x89PNG"); !bytes.Equal(withGrid(raw), raw) {
		t.Error("a picture that does not decode changed")
	}
}

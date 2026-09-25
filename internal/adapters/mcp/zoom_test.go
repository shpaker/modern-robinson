package mcp

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// pattern is a PNG whose every pixel is a colour of its own.
func pattern(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x), G: uint8(y), B: uint8(x>>8 | y>>8<<4), A: 0xFF,
			})
		}
	}
	pic, err := encodePNG(img)
	if err != nil {
		t.Fatal(err)
	}
	return pic
}

// A close-up is the region enlarged a whole number of times — as many as
// still fit the screen's size, and never past it: the pixel (x, y) of the
// picture fills the scale×scale block at ((x-x0)·scale, (y-y0)·scale), and
// the close-up holds nothing else. A region past the picture's edge is cut
// down to it, and the reply says what is shown.
func TestCloseUpEnlargesEveryPixel(t *testing.T) {
	pic := pattern(t, 640, 480)
	src := unpack(t, pic)
	for _, c := range []struct {
		r    regionIn
		want shownOut
	}{
		{regionIn{100, 100, 260, 220}, shownOut{100, 100, 260, 220, 4}},
		{regionIn{333, 17, 380, 90}, shownOut{333, 17, 380, 90, 6}},
		{regionIn{10, 20, 18, 28}, shownOut{10, 20, 18, 28, 60}},
		// Past the corner, and from before the edge.
		{regionIn{600, 440, 700, 500}, shownOut{600, 440, 640, 480, 12}},
		{regionIn{-50, -50, 30, 30}, shownOut{0, 0, 30, 30, 16}},
		// Just fits twice, as it is or once cut down to the picture.
		{regionIn{0, 0, 320, 240}, shownOut{0, 0, 320, 240, 2}},
		{regionIn{0, 0, 50, 240}, shownOut{0, 0, 50, 240, 2}},
		{regionIn{400, 300, 1000, 900}, shownOut{400, 300, 640, 480, 2}},
	} {
		got, shown, err := closeUp(pic, c.r, false)
		if err != nil {
			t.Errorf("%+v: %v", c.r, err)
			continue
		}
		if shown != c.want {
			t.Errorf("%+v: shown %+v, want %+v", c.r, shown, c.want)
		}
		k, w, h := shown.Scale, shown.X1-shown.X0, shown.Y1-shown.Y0
		if w*k > zoomW || h*k > zoomH {
			t.Errorf("%+v: ×%d is past %d×%d", c.r, k, zoomW, zoomH)
		}
		if w*(k+1) <= zoomW && h*(k+1) <= zoomH {
			t.Errorf("%+v: ×%d is not the most that fits %d×%d", c.r, k,
				zoomW, zoomH)
		}
		img := unpack(t, got)
		if img.Bounds() != image.Rect(0, 0, w*k, h*k) {
			t.Errorf("%+v: bounds = %v", c.r, img.Bounds())
			continue
		}
		bad := 0
		for y := shown.Y0; y < shown.Y1; y++ {
			for x := shown.X0; x < shown.X1; x++ {
				at := image.Pt((x-shown.X0)*k, (y-shown.Y0)*k)
				for dy := range k {
					for dx := range k {
						if img.RGBAAt(at.X+dx, at.Y+dy) != src.RGBAAt(x, y) {
							bad++
						}
					}
				}
			}
		}
		if bad > 0 {
			t.Errorf("%+v: %d pixels out of their blocks", c.r, bad)
		}
	}
}

// The grid runs finer the more a close-up enlarges: its step divides the
// plain one, so the plain grid's lines stay, and its lines come 40 to 80 px
// apart on the picture — room for a label, and a line near any aim.
func TestGridStepFollowsTheScale(t *testing.T) {
	for k, want := range map[int]int{
		1: 40, 2: 20, 3: 20, 4: 10, 5: 8, 8: 5, 10: 4, 20: 2, 40: 1, 60: 1,
	} {
		if got := gridStepAt(k); got != want {
			t.Errorf("×%d: step %d, want %d", k, got, want)
		}
	}
	for k := 1; k <= zoomH/zoomMin; k++ {
		s := gridStepAt(k)
		if gridStep%s != 0 || s*k < gridStep || s*k > 2*gridStep {
			t.Errorf("×%d: step %d, lines %d px apart", k, s, s*k)
		}
	}
}

// The grid on a close-up is signed in the screen's coordinates, not the
// enlarged picture's, and at four times runs every 10 px of the screen, 40
// px apart on the picture. A line that would run through the labels along
// an edge is left out, and so is a label the picture would cut short; away
// from the lines and the labels the close-up keeps every pixel it had.
func TestCloseUpGridSignsScreenCoordinates(t *testing.T) {
	ground := grounds["grey"]
	pic, shown, err := closeUp(flat(t, 640, 480, ground),
		regionIn{105, 97, 265, 217}, true)
	if err != nil {
		t.Fatal(err)
	}
	if shown.Scale != 4 {
		t.Fatalf("shown %+v", shown)
	}
	got := unpack(t, pic)
	lit := func(x, y int) bool { return got.RGBAAt(x, y).R > ground.R }
	for x := 120; x <= 260; x += 10 {
		px := (x - 105) * 4
		if !lit(px, 30) || lit(px-1, 30) || lit(px+1, 30) {
			t.Errorf("no line x=%d at %d", x, px)
		}
		if x < 260 && !signed(t, got, ground, x, px+3, 11) {
			t.Errorf("the line x=%d is not signed %d", x, x)
		}
	}
	for y := 110; y <= 210; y += 10 {
		py := (y - 97) * 4
		if !lit(320, py) || lit(320, py-1) || lit(320, py+1) {
			t.Errorf("no line y=%d at %d", y, py)
		}
		if !signed(t, got, ground, y, 2, py+12) {
			t.Errorf("the line y=%d is not signed %d", y, y)
		}
	}
	if got.RGBAAt(20, 30) != ground {
		t.Error("the line x=110 runs through the y labels")
	}
	if got.RGBAAt(320, 12) != ground {
		t.Error("the line y=100 runs through the x labels")
	}
	for y := range 13 {
		for x := 622; x < 640; x++ {
			if got.RGBAAt(x, y) != ground {
				t.Fatalf("the label of x=260 is cut short at (%d,%d)", x, y)
			}
		}
	}
	near := func(v, first int) bool {
		return v >= first-1 && (v-first+1)%40 <= 2
	}
	touched := 0
	for y := labelRows; y < 480; y++ {
		for x := labelCols; x < 640; x++ {
			if !near(x, 60) && !near(y, 52) && got.RGBAAt(x, y) != ground {
				touched++
			}
		}
	}
	if touched > 0 {
		t.Errorf("%d pixels between the lines changed", touched)
	}
}

// A y label the picture's bottom edge would cut short is left out, its line
// stays: at four times the line y=210 runs at 452 px, and its label, down to
// 464, would lose its lower rows on a close-up 460 px high — but fits one of
// 464, its digits ending on the last row.
func TestCloseUpGridLabelsAtTheBottom(t *testing.T) {
	ground := grounds["grey"]
	for _, c := range []struct {
		y1, high int
		signed   bool
	}{
		{212, 460, false},
		{213, 464, true},
	} {
		pic, shown, err := closeUp(flat(t, 640, 480, ground),
			regionIn{105, 97, 265, c.y1}, true)
		if err != nil {
			t.Fatal(err)
		}
		got := unpack(t, pic)
		if shown.Scale != 4 || got.Bounds().Dy() != c.high {
			t.Fatalf("y1=%d: shown %+v, %v", c.y1, shown, got.Bounds())
		}
		py := (210 - 97) * 4
		if got.RGBAAt(320, py).R <= ground.R {
			t.Errorf("y1=%d: no line y=210 at %d", c.y1, py)
		}
		if c.signed {
			if !signed(t, got, ground, 210, 2, py+12) {
				t.Errorf("y1=%d: the line y=210 is not signed", c.y1)
			}
			continue
		}
		for y := py + 2; y < c.high; y++ {
			for x := range labelCols {
				if got.RGBAAt(x, y) != ground {
					t.Fatalf("y1=%d: the label of y=210 is cut short at "+
						"(%d,%d)", c.y1, x, y)
				}
			}
		}
	}
}

// A region with no inside, one past the picture, one too small to enlarge,
// one too big to enlarge twice within the screen's size, and a picture that
// is none are refused, each saying why.
func TestCloseUpRefuses(t *testing.T) {
	pic := flat(t, 640, 480, grounds["grey"])
	for _, c := range []struct {
		r    regionIn
		want string
	}{
		{regionIn{10, 10, 5, 20}, "вывернут"},
		{regionIn{10, 20, 30, 20}, "пуст"},
		{regionIn{700, 0, 800, 100}, "за краем картинки 640×480"},
		{regionIn{0, -30, 100, 0}, "за краем"},
		{regionIn{0, 0, 5, 100}, "мал"},
		{regionIn{636, 0, 700, 100}, "от него 4×100 px"},
		{regionIn{0, 0, 640, 480}, "велик"},
		{regionIn{0, 0, 50, 300}, "нужно не больше 320×240"},
		{regionIn{-100, 100, 400, 200}, "от него 400×100 px"},
	} {
		got, _, err := closeUp(pic, c.r, true)
		if err == nil || !strings.Contains(err.Error(), c.want) ||
			!strings.HasPrefix(err.Error(), "region") || got != nil {
			t.Errorf("%+v: %v, want %q", c.r, err, c.want)
		}
	}
	if _, _, err := closeUp([]byte("\x89PNG"), regionIn{0, 0, 100, 100},
		false); err == nil {
		t.Error("a close-up of no picture")
	}
}

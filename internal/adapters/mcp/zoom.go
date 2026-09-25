package mcp

import (
	"errors"
	"fmt"
	"image"
)

// zoomMin is the least side of a region, in the picture's pixels, a close-up
// is made of.
const zoomMin = 8

// A close-up is enlarged as many whole times as still fit it into the
// screen's own size, and twice at the least, so a region is at most half the
// screen each way. Within that size it weighs no more than the puzzle's
// picture that comes with every reply, and no client has to shrink it —
// which would smear the very pixels it enlarges. A bigger region would be no
// close-up at all: the whole picture shows it as well.
const (
	zoomW = 640
	zoomH = 480
)

// regionIn is a part of the picture, in its pixels: from the corner (x0, y0)
// up to (x1, y1), the far edges not included.
type regionIn struct {
	X0 int `json:"x0" jsonschema:"левый край"`
	Y0 int `json:"y0" jsonschema:"верхний край"`
	X1 int `json:"x1" jsonschema:"правый край, больше x0"`
	Y1 int `json:"y1" jsonschema:"нижний край, больше y0"`
}

// shownOut is what a close-up shows: the region as far as the picture has
// it, and how many times it is enlarged.
type shownOut struct {
	X0    int `json:"x0"`
	Y0    int `json:"y0"`
	X1    int `json:"x1"`
	Y1    int `json:"y1"`
	Scale int `json:"scale"`
}

// errInsideOut refuses a region with no inside.
var errInsideOut = errors.New(
	"region пуст или вывернут: нужно x0 < x1 и y0 < y1")

// closeUp is region r of the picture, enlarged a whole number of times, each
// pixel a block of its own (nearest neighbour), with grid the coordinate
// grid over it signed in the picture's own coordinates; and what it shows. r
// is cut down to the picture; a region with nothing left of it, too little
// to enlarge, or too much to enlarge twice within zoomW×zoomH, is refused.
func closeUp(pic []byte, r regionIn, grid bool) ([]byte, shownOut, error) {
	want := image.Rectangle{
		Min: image.Pt(r.X0, r.Y0), Max: image.Pt(r.X1, r.Y1),
	}
	if want.Empty() {
		return nil, shownOut{}, errInsideOut
	}
	src, err := decodeRGBA(pic)
	if err != nil {
		return nil, shownOut{}, errors.New("region: картинки нет")
	}
	b := src.Bounds()
	in := want.Intersect(b)
	switch {
	case in.Empty():
		return nil, shownOut{}, fmt.Errorf("region за краем картинки %d×%d",
			b.Dx(), b.Dy())
	case in.Dx() < zoomMin || in.Dy() < zoomMin:
		return nil, shownOut{}, fmt.Errorf(
			"region мал: в картинке %d×%d от него %d×%d px, "+
				"нужно хотя бы %d по каждой стороне",
			b.Dx(), b.Dy(), in.Dx(), in.Dy(), zoomMin)
	case in.Dx()*2 > zoomW || in.Dy()*2 > zoomH:
		return nil, shownOut{}, fmt.Errorf(
			"region велик: в картинке %d×%d от него %d×%d px, "+
				"нужно не больше %d×%d; вся картинка — look без region",
			b.Dx(), b.Dy(), in.Dx(), in.Dy(), zoomW/2, zoomH/2)
	}
	k := min(zoomW/in.Dx(), zoomH/in.Dy())
	img := image.NewRGBA(image.Rect(0, 0, in.Dx()*k, in.Dy()*k))
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			img.SetRGBA(x, y, src.RGBAAt(in.Min.X+x/k, in.Min.Y+y/k))
		}
	}
	if grid {
		drawGrid(img, view{at: in.Min, scale: k})
	}
	out, err := encodePNG(img)
	if err != nil {
		return nil, shownOut{}, err
	}
	return out, shownOut{
		X0: in.Min.X, Y0: in.Min.Y, X1: in.Max.X, Y1: in.Max.Y, Scale: k,
	}, nil
}

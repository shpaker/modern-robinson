package adapters

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
)

// LoadIcon renders a movie's first frame as an inventory icon: the sprite is
// cropped to its opaque pixels and fitted (never upscaled) into a w x h image,
// centred. Returns nil if the movie is missing or empty.
func LoadIcon(res interfaces.IResources, movie string, w, h int) *ebiten.Image {
	frames, pal := res.MovieFrames(movie)
	if len(frames) == 0 {
		return nil
	}
	n := frames[0]
	src, _, _, ok := cropOpaque(n.RGBA(pal), n.Width, n.Height)
	if !ok {
		return nil
	}
	b := src.Bounds()
	s := math.Min(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	if s > 1 {
		s = 1 // keep small items pixel-accurate
	}
	dw, dh := float64(b.Dx())*s, float64(b.Dy())*s
	icon := ebiten.NewImage(w, h)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate((float64(w)-dw)/2, (float64(h)-dh)/2)
	icon.DrawImage(src, op)
	return icon
}

// DecalFrame is one frame of a scene-object animation: its opaque pixels are
// pre-placed at final screen coordinates, so it draws at offset (X, Y). A nil
// Img means a fully transparent frame.
type DecalFrame struct {
	Img  *ebiten.Image
	X, Y int
}

// LoadDecal decodes a movie into decal frames (cropped to the opaque bbox with
// the offset kept, since object sprites are full-canvas decals in screen space).
func LoadDecal(res interfaces.IResources, movie string) []DecalFrame {
	frames, pal := res.MovieFrames(movie)
	out := make([]DecalFrame, len(frames))
	for i, n := range frames {
		if img, x, y, ok := cropOpaque(n.RGBA(pal), n.Width, n.Height); ok {
			out[i] = DecalFrame{Img: img, X: x, Y: y}
		}
	}
	return out
}

// cropOpaque crops an RGBA buffer (h*w*4) to its opaque bounding box, returning
// the Ebiten image and its top-left offset, or ok=false if fully transparent.
func cropOpaque(rgba []byte, w, h int) (img *ebiten.Image, ox, oy int, ok bool) {
	minx, miny, maxx, maxy := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if rgba[(y*w+x)*4+3] > 0 {
				if x < minx {
					minx = x
				}
				if x > maxx {
					maxx = x
				}
				if y < miny {
					miny = y
				}
				if y > maxy {
					maxy = y
				}
			}
		}
	}
	if maxx < 0 {
		return nil, 0, 0, false
	}
	cw, ch := maxx-minx+1, maxy-miny+1
	sub := make([]byte, cw*ch*4)
	for y := 0; y < ch; y++ {
		src := ((miny+y)*w + minx) * 4
		copy(sub[y*cw*4:(y+1)*cw*4], rgba[src:src+cw*4])
	}
	im := ebiten.NewImage(cw, ch)
	im.WritePixels(sub)
	return im, minx, miny, true
}

// Package adapters is the Presentation layer: it turns Domain data into engine
// resources (Ebiten images, audio players). It talks to data only through
// repository interfaces. See ARCHITECTURE.md.
package adapters

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
)

// Animation is a character animation: each movie frame cropped to its own
// opaque bbox with a feet anchor, so the figure walks in place while grid
// logic moves it across the scene.
type Animation struct {
	Frames  []*ebiten.Image
	Anchors [][2]int // (ax, ay) feet anchor per frame
}

// OK reports whether the animation has any frames.
func (a *Animation) OK() bool { return a != nil && len(a.Frames) > 0 }

// LoadAnimation decodes a movie into cropped, anchored Ebiten frames.
func LoadAnimation(res interfaces.IResources, movie string) *Animation {
	frames, pal := res.MovieFrames(movie)
	a := &Animation{}
	for _, n := range frames {
		rgba := n.RGBA(pal)
		minx, miny, maxx, maxy := n.Width, n.Height, -1, -1
		for y := 0; y < n.Height; y++ {
			for x := 0; x < n.Width; x++ {
				if n.Mask[y*n.Width+x] {
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
			continue
		}
		w, h := maxx-minx+1, maxy-miny+1
		sub := make([]byte, w*h*4)
		for y := 0; y < h; y++ {
			src := ((miny+y)*n.Width + minx) * 4
			copy(sub[y*w*4:(y+1)*w*4], rgba[src:src+w*4])
		}
		img := ebiten.NewImage(w, h)
		img.WritePixels(sub)
		a.Frames = append(a.Frames, img)
		a.Anchors = append(a.Anchors, [2]int{footCenter(sub, w, h), h})
	}
	return a
}

// footCenter is the horizontal centre of the lowest 12 opaque rows.
func footCenter(sub []byte, w, h int) int {
	fy0 := h - 12
	if fy0 < 0 {
		fy0 = 0
	}
	minx, maxx := w, -1
	for y := fy0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sub[(y*w+x)*4+3] > 0 {
				if x < minx {
					minx = x
				}
				if x > maxx {
					maxx = x
				}
			}
		}
	}
	if maxx < 0 {
		return w / 2
	}
	return (minx + maxx) / 2
}

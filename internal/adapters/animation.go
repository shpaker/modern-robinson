// Package adapters is the Presentation layer: it turns Domain data into engine
// resources (Ebiten images, audio players). It talks to data only through
// repository interfaces. See ARCHITECTURE.md.
package adapters

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/interfaces"
)

// Animation is a character animation. The engine draws every frame on the
// movie's full canvas at origin = cellAnchor - Shift, so each frame is kept as
// its cropped opaque bitmap plus that crop's offset (BBox) inside the canvas.
// The figure's sub-cell motion is baked into those offsets by the artists.
type Animation struct {
	Frames []*ebiten.Image
	BBox   [][2]int // (minx, miny) of each frame's crop within the canvas
	Shift  [2]int   // the movie's canvas hotspot (.SCR origin)
}

// OK reports whether the animation has any frames.
func (a *Animation) OK() bool { return a != nil && len(a.Frames) > 0 }

// LoadAnimation decodes a movie into cropped canvas-space frames.
func LoadAnimation(res interfaces.IResources, movie string) *Animation {
	frames, pal := res.MovieFrames(movie)
	a := &Animation{Shift: res.MovieShift(movie)}
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
			// A fully transparent frame still consumes a slot so frame
			// indices stay aligned with the script.
			a.Frames = append(a.Frames, nil)
			a.BBox = append(a.BBox, [2]int{0, 0})
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
		a.BBox = append(a.BBox, [2]int{minx, miny})
	}
	return a
}

package repositories

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// feet returns the bottom-centre of a frame's opaque pixels — where the
// character stands on the canvas.
func feet(n *types.NGB) (int, int, bool) {
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
		return 0, 0, false
	}
	return (minx + maxx) / 2, maxy, true
}

// TestProbeGrid pairs each action's Aproach target cell with the place its
// movie actually draws the hero, giving ground truth for the cell->screen map.
func TestProbeGrid(t *testing.T) {
	root := testutil.GameRoot(t)
	r := NewResources(root)
	p := SceneParser{}
	for _, scene := range []string{"SCENA0", "SCENA1", "SCENA3"} {
		c := r.SceneContainer(scene)
		if c == nil {
			continue
		}
		scn, _ := c.ExtractName(scene + ".SCN")
		sc := p.ParseScene(string(scn))
		cellOf := map[string][2]int{}
		for _, o := range sc.Objects {
			cellOf[strings.ToLower(o.Name)] = [2]int{o.GX, o.GY}
		}
		t.Logf("=== %s LeftTopGrid=%v GridSize=%v GridShift=%v Len=%v",
			scene, sc.LeftTopGrid, sc.GridSize, sc.GridShift, sc.GridLength)
		for _, e := range c.Entries() {
			up := strings.ToUpper(e.Name)
			if !strings.HasPrefix(up, "ROHAN") || !strings.HasSuffix(up, ".FS") {
				continue
			}
			d, err := c.Extract(e)
			if err != nil {
				continue
			}
			fs := p.ParseFrameScript(string(d))
			// find the Aproach target
			var obj string
			var dx, dy int
			for _, fr := range fs.Frames {
				for _, ev := range fr.Events {
					if !strings.EqualFold(ev.Kw, "aproach") || len(ev.Args) != 4 {
						continue
					}
					obj, dx, dy = strings.ToLower(ev.Args[1]),
						atoiSafe(ev.Args[2]), atoiSafe(ev.Args[3])
				}
			}
			if obj == "" {
				continue
			}
			cell, ok := cellOf[obj]
			if !ok {
				continue
			}
			frames, _ := r.MovieFrames(fs.MovieName)
			if len(frames) == 0 {
				continue
			}
			fx, fy, ok := feet(frames[0])
			if !ok {
				continue
			}
			t.Logf("%-12s obj=%-9s cell=(%d,%d) feet=(%d,%d)",
				e.Name, obj, cell[0]+dx, cell[1]+dy, fx, fy)
		}
	}
}

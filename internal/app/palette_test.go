package app

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// The engine paints every sprite on stage with the scene palette and never
// reads a movie's own .COL. Many objects are cut-outs of the background, kept
// so the heroes can walk behind them: their pixels are background indices,
// while the movie still ships another palette. Painted with that one, SCENA1's
// oak comes out as a pale band across the trunk.
func TestCutoutsMatchBackgroundInScenePalette(t *testing.T) {
	res := repositories.NewResources(testutil.GameRoot(t))
	parser := repositories.SceneParser{}
	for _, c := range []struct{ scene, obj string }{
		{"SCENA1", "oak"},
		{"PALACE", "idol1"},
		{"SHIP1", "lefdoor"},
	} {
		bg, scenePal, _ := res.SceneBackground(c.scene)
		sc, fs := cutout(t, res, parser, c.scene, c.obj)
		frames, moviePal := res.MovieFrames(fs.MovieName)
		if bg == nil || len(frames) == 0 {
			t.Fatalf("%s/%s: no art", c.scene, c.obj)
		}
		grid := use_cases.NewGrid(sc, bg.Width, bg.Height)
		var ref types.ObjectRef
		for _, r := range sc.Objects {
			if strings.EqualFold(r.Name, c.obj) {
				ref = r
			}
		}
		ax, ay := grid.ToScreen(ref.GX, ref.GY)
		ox := ax - fs.Shift[0]
		oy := ay - fs.Shift[1]

		same := func(pal types.Palette) float64 {
			n, hit := 0, 0
			f := frames[0]
			for y := 0; y < f.Height; y++ {
				for x := 0; x < f.Width; x++ {
					bx, by := ox+x, oy+y
					if !f.Mask[y*f.Width+x] || bx < 0 || by < 0 ||
						bx >= bg.Width || by >= bg.Height {
						continue
					}
					n++
					if pal[f.Indices[y*f.Width+x]] ==
						scenePal[bg.Indices[by*bg.Width+bx]] {
						hit++
					}
				}
			}
			if n == 0 {
				t.Fatalf("%s/%s: sprite misses the background", c.scene, c.obj)
			}
			return float64(hit) / float64(n)
		}
		if got := same(scenePal); got < 0.95 {
			t.Errorf("%s/%s: %.2f of the sprite matches the background "+
				"in the scene palette, want >= 0.95", c.scene, c.obj, got)
		}
		if got := same(moviePal); got > 0.5 {
			t.Errorf("%s/%s: %.2f matches in the movie's own palette; "+
				"the case no longer tells the palettes apart",
				c.scene, c.obj, got)
		}
	}
}

// cutout parses a scene and the FonScript of one of its objects.
func cutout(
	t *testing.T,
	res *repositories.Resources,
	parser repositories.SceneParser,
	scene, obj string,
) (*types.Scene, *types.FrameScript) {
	t.Helper()
	c := res.SceneContainer(scene)
	if c == nil {
		t.Fatalf("%s: no container", scene)
	}
	scn, err := c.ExtractName(scene + ".SCN")
	if err != nil {
		t.Fatalf("%s: %v", scene, err)
	}
	ob := objectFor(parser, c, obj)
	if ob == nil {
		t.Fatalf("%s/%s: no .OB", scene, obj)
	}
	raw, err := c.ExtractName(strings.ToUpper(ob.FonScript) + ".FS")
	if err != nil {
		t.Fatalf("%s/%s: %v", scene, obj, err)
	}
	fs := parser.ParseFrameScript(string(raw))
	if fs.Shift == ([2]int{}) {
		fs.Shift = res.MovieShift(fs.MovieName)
	}
	return parser.ParseScene(string(scn)), fs
}

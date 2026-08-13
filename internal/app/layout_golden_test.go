package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

var updateGolden = flag.Bool(
	"update-golden",
	false,
	"rewrite testdata/layout.golden from the game files",
)

// goldenScenes is every scene of the game, in a fixed order.
var goldenScenes = []string{
	"CAB_A1", "CAB_A2", "CAB_A3", "CAB_A4", "CAB_A5",
	"CAB_B1", "CAB_B2", "CAB_B3", "CAB_B4", "CAB_B5", "CAB_B6",
	"CAB_C1", "CAB_C2", "CAB_C3", "CAB_C3A", "CAB_C4", "CAB_C5",
	"CHESS", "INT0", "INT1", "INT2", "INT3", "INT4", "MAPSCR", "PALACE",
	"SCENA0", "SCENA1", "SCENA2", "SCENA3", "SCENA4", "SCENA5", "SCENA6",
	"SCENA7", "SCENA8", "SHIP1", "SHIP2", "SHIP3",
}

// TestSceneLayoutGolden pins where every scene puts things.
//
// It records the numbers a frame is composed from rather than the pixels: the
// grid, each object's cell, the sprite origin the engine would blit its canvas
// at, its hit rectangle and its draw order. Those are exactly what the
// placement rules produce, so a regression in any of them shows up here — while
// a golden of the composited frames would cost tens of megabytes of PNGs, need
// a graphics context, and still not say which number moved.
//
// Run with -update-golden after an intentional change, and read the diff.
func TestSceneLayoutGolden(t *testing.T) {
	root := testutil.GameRoot(t)
	res := repositories.NewResources(root)
	parser := repositories.SceneParser{}

	var b strings.Builder
	for _, name := range goldenScenes {
		c := res.SceneContainer(name)
		if c == nil {
			t.Fatalf("%s: no container", name)
		}
		scn, err := c.ExtractName(name + ".SCN")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sc := parser.ParseScene(string(scn))
		if sc.Size == [2]int{0, 0} {
			sc.Size = [2]int{ViewW, PlayH}
		}
		writeSceneLayout(&b, res, parser, name, sc, c)
	}

	path := filepath.Join("testdata", "layout.golden")
	got := b.String()
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("golden rewritten:", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(
			"%v (run: go test ./internal/app -run Golden -update-golden)",
			err,
		)
	}
	if got != string(want) {
		t.Errorf("scene layout changed:\n%s", firstDiff(string(want), got))
	}
}

// writeSceneLayout appends one scene's placement to b.
func writeSceneLayout(
	b *strings.Builder,
	res *repositories.Resources,
	parser repositories.SceneParser,
	name string,
	sc *types.Scene,
	c interface {
		ExtractName(string) ([]byte, error)
	},
) {
	zper := sc.ZPerGrid
	if zper == 0 {
		zper = 8
	}
	grid := use_cases.NewGrid(sc, sc.Size[0], sc.Size[1])
	fmt.Fprintf(b, "scene %s size=%v ltg=%v gs=%v gsh=%v gl=%v zper=%d\n",
		name, sc.Size, sc.LeftTopGrid, sc.GridSize, sc.GridShift,
		sc.GridLength, zper)

	// The walkable lattice: which declared cells a character may stand on.
	var walk []string
	nx, ny := grid.Dims()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			if grid.Valid(gx, gy) {
				walk = append(walk, fmt.Sprintf("%d,%d", gx, gy))
			}
		}
	}
	fmt.Fprintf(b, "  walkable %d: %s\n", len(walk), strings.Join(walk, " "))

	rows := make([]string, 0, len(sc.Objects))
	for _, ref := range sc.Objects {
		ob := objectFor(parser, c, ref.Name)
		if ob == nil {
			rows = append(rows, fmt.Sprintf("  obj %-10s cell=%d,%d (no .OB)",
				strings.ToLower(ref.Name), ref.GX, ref.GY))
			continue
		}
		cx, cy := grid.Corner(ref.GX, ref.GY)
		ax, ay := grid.ToScreen(ref.GX, ref.GY)
		shift := objectShift(res, parser, c, ob)
		zones := make([]string, 0, len(ob.ActiveZones))
		for _, az := range ob.ActiveZones {
			zones = append(zones, fmt.Sprintf("%d,%d,%d,%d",
				cx+az[0], cy+az[1], az[2], az[3]))
		}
		rows = append(rows, fmt.Sprintf(
			"  obj %-10s cell=%d,%d star=%v z=%d cur=%d text=%d "+
				"shift=%d,%d origin=%d,%d zones=[%s] block=%v",
			strings.ToLower(ref.Name), ref.GX, ref.GY, ref.Flag,
			ref.GY*zper+ob.Z, ob.Cursor, ob.Text,
			shift[0], shift[1], ax-shift[0], ay-shift[1],
			strings.Join(zones, " "), ob.ClosedVert,
		))
	}
	sort.Strings(rows)
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
}

// objectFor parses an object's .OB, or nil when the scene ships none.
func objectFor(
	parser repositories.SceneParser,
	c interface {
		ExtractName(string) ([]byte, error)
	},
	name string,
) *types.SceneObject {
	d, err := c.ExtractName(strings.ToUpper(name) + ".OB")
	if err != nil {
		return nil
	}
	return parser.ParseObject(string(d))
}

// objectShift is the canvas hotspot the object's sprite is placed by: its
// FonScript's own Shift when it declares one, else the movie's .SCR origin.
func objectShift(
	res *repositories.Resources,
	parser repositories.SceneParser,
	c interface {
		ExtractName(string) ([]byte, error)
	},
	ob *types.SceneObject,
) [2]int {
	fon := strings.ToLower(ob.FonScript)
	if fon == "" || fon == "null" {
		return [2]int{}
	}
	raw, err := c.ExtractName(strings.ToUpper(fon) + ".FS")
	if err != nil {
		return [2]int{}
	}
	fs := parser.ParseFrameScript(string(raw))
	if fs.Shift != ([2]int{}) {
		return fs.Shift
	}
	return res.MovieShift(fs.MovieName)
}

// firstDiff reports the first differing line with a little context, which is
// far more useful than dumping two thousand identical ones.
func firstDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		wl, gl := "<missing>", "<missing>"
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			var b strings.Builder
			for j := max(0, i-2); j < i; j++ {
				fmt.Fprintf(&b, "  %s\n", w[j])
			}
			fmt.Fprintf(&b, "- want line %d: %s\n+ got  line %d: %s\n",
				i+1, wl, i+1, gl)
			return b.String()
		}
	}
	return "(files differ only in length)"
}

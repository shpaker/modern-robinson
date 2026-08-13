package app

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// sceneFor builds a game sitting on a shipped scene with its own grid, objects
// and container, but no graphics — everything the placement rules run on. The
// objects it wires up are the scene's own, so their zones and z are the
// authored ones.
func sceneFor(t *testing.T, name string) *Game {
	t.Helper()
	res := repositories.NewResources(testutil.GameRoot(t))
	c := res.SceneContainer(name)
	if c == nil {
		t.Fatalf("no %s container", name)
	}
	parser := repositories.SceneParser{}
	scn, err := c.ExtractName(name + ".SCN")
	if err != nil {
		t.Fatalf("%s.SCN: %v", name, err)
	}
	sc := parser.ParseScene(string(scn))
	g := &Game{
		res:       res,
		parser:    parser,
		gs:        types.NewGameState(),
		sceneName: name,
		sceneC:    c,
		sc:        sc,
		grid:      use_cases.NewGrid(sc, sc.Size[0], sc.Size[1]),
		w:         sc.Size[0],
		h:         sc.Size[1],
		zper:      sc.ZPerGrid,
	}
	for _, ref := range sc.Objects {
		d, err := c.ExtractName(strings.ToUpper(ref.Name) + ".OB")
		if err != nil {
			continue // a few objects ship no .OB of their own
		}
		ob := parser.ParseObject(string(d))
		g.sceneObjs = append(g.sceneObjs, &sceneObj{
			ref: ref, ob: ob, z: ref.GY*g.zper + ob.Z,
		})
	}
	g.buildHotspots()
	return g
}

// The map is a scene like any other, and what makes it read as a map is its two
// entry scripts: they park both characters on a cell off the far side of the
// 640px stage and switch the map button off. Skip them and the hero stands in
// the middle of the island, the button stays lit, and the map is walkable.
func TestMapEntriesParkBothCharactersOffStage(t *testing.T) {
	g := sceneFor(t, "MAPSCR")
	g.gs.UI["map"] = true
	g.gs.SetVar("FridIs", 1)

	g.runEntryEvents(t, "Roin0")
	g.runFridEntry("Frin0")

	if g.cell != [2]int{7, 0} {
		t.Errorf("Roby at %v, want the parking cell 7,0", g.cell)
	}
	if g.robyZ != 4 {
		t.Errorf("Roby z = %d, want 4", g.robyZ)
	}
	if g.fridCell != [2]int{7, 0} {
		t.Errorf("Friday at %v, want the parking cell 7,0", g.fridCell)
	}
	if g.fridZ != 0 {
		t.Errorf("Friday z = %d, want 0", g.fridZ)
	}
	if g.gs.UI["map"] {
		t.Error("the map button must go dim while the map is up")
	}
	if x, _ := g.grid.ToScreen(g.cell[0], g.cell[1]); x < ViewW {
		t.Errorf("parking anchor x = %d, want it past the %dpx stage", x, ViewW)
	}
}

// The parking cell is walled off (ClosedVert 6,0; 6,1; 7,1), so a click on the
// island cannot walk the hero back into view — the map takes destinations, not
// footsteps.
func TestMapParkingIsWalledOff(t *testing.T) {
	g := sceneFor(t, "MAPSCR")
	parked := [2]int{7, 0}
	nx, ny := g.grid.Dims()
	for gy := 0; gy < ny; gy++ {
		for gx := 0; gx < nx; gx++ {
			if !g.grid.Valid(gx, gy) {
				continue
			}
			if p := g.grid.Path(parked, [2]int{gx, gy}); p != nil {
				t.Fatalf("walked off the parking cell to %d,%d: %v", gx, gy, p)
			}
		}
	}
}

// Overlap is how the authors layer zones: the hut the quest builds on the map
// sits wholly inside the parrot clearing and carries the higher ZCoord, so it
// draws over it — and the click has to follow the drawing, or the hut is
// unreachable and every click on it leads to the clearing instead.
func TestTopmostZoneAnswersTheClick(t *testing.T) {
	g := sceneFor(t, "MAPSCR")
	cases := []struct {
		name string
		x, y int
		want string
	}{
		{"the hut, drawn over the clearing", 424, 185, "home"},
		{"the clearing, clear of the hut", 340, 150, "house"},
		{"the shipwreck", 430, 100, "ship"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hs := g.hotspotAt(c.x, c.y)
			if hs == nil {
				t.Fatalf("no zone at %d,%d, want %q", c.x, c.y, c.want)
			}
			if !strings.EqualFold(hs.key, c.want) {
				t.Errorf("zone at %d,%d = %q, want %q",
					c.x, c.y, hs.key, c.want)
			}
		})
	}
}

// "Empty place" is a zone over the whole scene with the lowest ZCoord there is,
// the authored floor for clicks that hit nothing. Every real object stands above
// it — including the ones listed after it, which no ObjectList-order tie-break
// could reach.
func TestEmptyPlaceIsTheFloorOfTheScene(t *testing.T) {
	for _, scene := range []string{"SCENA3", "SCENA8"} {
		t.Run(scene, func(t *testing.T) {
			g := sceneFor(t, scene)
			last := -1
			for i, hs := range g.hotspots {
				if hs.rect.Dx() > 1 && hs.rect.Dy() > 1 {
					last = i // the real zones, not the one-pixel scenery
				}
			}
			if last < 0 || !strings.EqualFold(g.hotspots[last].key, "empty") {
				t.Errorf("the last real zone searched is %q, want empty place",
					g.hotspots[last].key)
			}
		})
	}
}

// Within one z the engine walks ObjectList from the start and stops at the
// first zone that contains the point — so the earlier object answers the click
// even though the later one is drawn over it. The click and the drawing
// disagree here, and the click is the engine's.
func TestEqualZGoesToTheFirstDeclaredObject(t *testing.T) {
	g := testScene(t, obj("first", 0, [4]int{0, 0, 100, 100}),
		obj("second", 0, [4]int{0, 0, 100, 100}))
	if hs := g.hotspotAt(1, 1); hs == nil || hs.key != "first" {
		t.Fatalf("hit %v, want the object declared first", hs)
	}
}

// Zone bounds are inclusive on all four sides: a w x h zone covers one more
// pixel each way, and ActiveZone 0,0,0,0 — what 94 scenery objects declare —
// is a single pixel rather than nothing at all.
func TestZoneBoundsAreInclusive(t *testing.T) {
	g := testScene(t, obj("box", 0, [4]int{0, 0, 10, 10}),
		obj("dot", 5, [4]int{20, 20, 0, 0}))
	cases := []struct {
		name string
		x, y int
		want string
	}{
		{"far corner of the box", 10, 10, "box"},
		{"one past it", 11, 11, ""},
		{"the single pixel", 20, 20, "dot"},
		{"next to the pixel", 21, 20, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hs := g.hotspotAt(c.x, c.y)
			got := ""
			if hs != nil {
				got = hs.key
			}
			if got != c.want {
				t.Errorf("zone at %d,%d = %q, want %q", c.x, c.y, got, c.want)
			}
		})
	}
}

// obj builds a bare scene object at the grid origin for the hotspot tests.
func obj(name string, z int, zones ...[4]int) *sceneObj {
	return &sceneObj{
		ref: types.ObjectRef{Name: name},
		ob:  &types.SceneObject{Name: name, ActiveZones: zones},
		z:   z,
	}
}

// testScene wires objects into a one-cell scene whose corner is the origin, so
// a zone's coordinates are its screen coordinates.
func testScene(t *testing.T, objs ...*sceneObj) *Game {
	t.Helper()
	g := &Game{grid: use_cases.NewGrid(&types.Scene{
		GridSize: [2]int{1, 1}, GridLength: [2]int{1, 1},
	}, ViewW, PlayH)}
	g.sceneObjs = objs
	g.buildHotspots()
	return g
}

// runEntryEvents plays an entry script's events the way loadScene does, without
// a movie: the parking is all in the events of frame 0.
func (g *Game) runEntryEvents(t *testing.T, name string) {
	t.Helper()
	raw, err := g.sceneC.ExtractName(strings.ToUpper(name) + ".FS")
	if err != nil {
		t.Fatalf("%s.FS: %v", name, err)
	}
	fs := g.parser.ParseFrameScript(string(raw))
	g.applyEvents(use_cases.NewPlayer(fs, false).Update(0))
}

// The search order is the engine's: z from the top down, ObjectList order
// within a z. Anything else and a scene's zones answer the wrong clicks.
func TestHotspotsAreInTheEngineSearchOrder(t *testing.T) {
	g := sceneFor(t, "SCENA0")
	if len(g.hotspots) == 0 {
		t.Fatal("no hotspots at all")
	}
	order := map[string]int{}
	for i, ref := range g.sc.Objects {
		order[strings.ToLower(ref.Name)] = i
	}
	for i := 1; i < len(g.hotspots); i++ {
		prev, cur := g.hotspots[i-1], g.hotspots[i]
		if prev.z < cur.z {
			t.Fatalf("%s (z=%d) searched before %s (z=%d)",
				prev.key, prev.z, cur.key, cur.z)
		}
		if prev.z == cur.z &&
			order[strings.ToLower(prev.key)] > order[strings.ToLower(cur.key)] {
			t.Fatalf("at z=%d, %s is searched before %s but declared after it",
				cur.z, prev.key, cur.key)
		}
	}
}

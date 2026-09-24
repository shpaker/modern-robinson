package app

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// CreateObject on an object the scene already holds moves it: the engine
// writes the new cell into the object's own slot (0x41e9a0), so it keeps its
// place in the list, its walls follow it, and nothing is duplicated — whether
// it was on stage or had been taken.
func TestCreateObjectMovesTheObjectInItsSlot(t *testing.T) {
	sc := &types.Scene{GridSize: [2]int{10, 10}, GridLength: [2]int{6, 4}}
	stone := &types.SceneObject{Name: "bgstone", ClosedVert: [][2]int{{0, 0}}}
	pool := &types.SceneObject{Name: "pool"}
	g := &Game{
		res:  actRes{},
		gs:   types.NewGameState(),
		grid: use_cases.NewGrid(sc, 0, 0),
		zper: 8,
		objects: map[string]*types.SceneObject{
			"bgstone": stone, "pool": pool,
		},
	}
	g.sceneObjs = []*sceneObj{
		{
			ref: types.ObjectRef{Name: "bgstone", GX: 5, GY: 1, Flag: true},
			ob:  stone,
		},
		{ref: types.ObjectRef{Name: "pool", GX: 1, GY: 1}, ob: pool},
	}
	g.applyObjectBlocking()

	g.spawnObject("bgstone", 2, 2)
	if len(g.sceneObjs) != 2 {
		t.Fatalf("%d objects, want the stone moved, not duplicated",
			len(g.sceneObjs))
	}
	s := g.sceneObjs[0]
	if s.ref.GX != 2 || s.ref.GY != 2 || !s.ref.Flag || s.z != 2*8 ||
		s.removed {
		t.Fatalf("slot 0 = %+v z=%d removed=%v, want the stone at (2,2)",
			s.ref, s.z, s.removed)
	}
	if !g.grid.Valid(5, 1) || g.grid.Valid(2, 2) {
		t.Error("the stone's wall must leave (5,1) for (2,2)")
	}

	g.hideObject("bgstone")
	g.spawnObject("bgstone", 3, 1)
	if len(g.sceneObjs) != 2 || g.sceneObjs[0].removed ||
		g.sceneObjs[0].ref.GX != 3 {
		t.Fatalf("a taken object must come back in its slot: %+v",
			g.sceneObjs[0].ref)
	}
	if !g.grid.Valid(2, 2) || g.grid.Valid(3, 1) {
		t.Error("the stone's wall must follow it to (3,1)")
	}
}

// loadObjects builds a real scene's objects the way loadScene does, over the
// given removals and CreateObject records.
func loadObjects(
	t *testing.T, name string, gone map[string]bool,
	spawns map[string][2]int,
) []*sceneObj {
	t.Helper()
	res := repositories.NewResources(testutil.GameRoot(t))
	c := res.SceneContainer(name)
	parser := repositories.SceneParser{}
	scn, _ := c.ExtractName(name + ".SCN")
	sc := parser.ParseScene(string(scn))
	objects := map[string]*types.SceneObject{}
	for _, e := range c.Entries() {
		if strings.HasSuffix(strings.ToUpper(e.Name), ".OB") {
			if d, err := c.Extract(e); err == nil {
				objects[strings.ToLower(e.Name[:len(e.Name)-3])] = parser.ParseObject(
					string(d),
				)
			}
		}
	}
	g := &Game{res: res, parser: parser}
	return loadSceneObjects(res, g.pal, g.parseFS, sc, objects, fonScripts(c),
		func(obj string) bool { return gone[strings.ToLower(obj)] },
		func(obj string) (types.Spawn, bool) {
			cell, ok := spawns[strings.ToLower(obj)]
			return types.Spawn{Obj: obj, GX: cell[0], GY: cell[1]}, ok
		})
}

func findObj(objs []*sceneObj, name string) (int, *sceneObj) {
	for i, s := range objs {
		if strings.EqualFold(s.ref.Name, name) {
			return i, s
		}
	}
	return -1, nil
}

// ROHANWO1 deletes SCENA1's sm1 at (4,1) and creates it at (2,2). The next
// visit builds it from the ObjectList, where it sits at (4,1): the recorded
// cell has to win, in the object's own slot.
func TestSceneLoadBuildsMovedObjectAtItsNewCell(t *testing.T) {
	before := loadObjects(t, "SCENA1", nil, nil)
	slot, s := findObj(before, "sm1")
	if s == nil || s.ref.GX != 4 || s.ref.GY != 1 {
		t.Fatalf("sm1 on a fresh visit = %v, want it at (4,1)", s)
	}
	after := loadObjects(t, "SCENA1", nil,
		map[string][2]int{"sm1": {2, 2}})
	i, s := findObj(after, "sm1")
	if s == nil || s.ref.GX != 2 || s.ref.GY != 2 || i != slot {
		t.Fatalf("moved sm1 = slot %d %v, want slot %d at (2,2)", i, s, slot)
	}
}

// Friday's double fraskcon is hidden at the start and only a CreateObject
// brings it on; a removal keeps the crab off the beach even though it starts
// there.
func TestSceneLoadHonoursSpawnsAndRemovals(t *testing.T) {
	if _, s := findObj(loadObjects(t, "SCENA0", nil, nil), "fraskcon"); s != nil {
		t.Fatal("fraskcon must wait for a CreateObject")
	}
	objs := loadObjects(t, "SCENA0", map[string]bool{"crb": true},
		map[string][2]int{"fraskcon": {3, 3}})
	if _, s := findObj(objs, "fraskcon"); s == nil || s.ref.GX != 3 {
		t.Fatalf("created fraskcon = %v, want it at (3,3)", s)
	}
	if _, s := findObj(objs, "crb"); s != nil {
		t.Fatal("the caught crab must stay off the beach")
	}
}

// ROCON hides Friday and creates her double relative to her (Fraskcon,
// Frid,0,0): the double stands where she stood, not on the hero.
func TestCreateObjectRelativeToFridayUsesHerCell(t *testing.T) {
	g := &Game{
		gs:        types.NewGameState(),
		sceneName: "SCENA1", // elsewhere, so only the record is written
		cell:      [2]int{1, 1},
		fridCell:  [2]int{5, 2},
	}
	g.createObject([]string{"SCENA0", "Fraskcon", "Frid", "0", "0"})
	if sp, ok := g.gs.SpawnAt("SCENA0", "fraskcon"); !ok || sp.GX != 5 ||
		sp.GY != 2 {
		t.Fatalf("double at %v %v, want Friday's cell (5,2)", sp, ok)
	}
	g.createObject([]string{"SCENA0", "banana", "Roby", "2", "0"})
	if sp, _ := g.gs.SpawnAt("SCENA0", "banana"); sp.GX != 3 || sp.GY != 1 {
		t.Fatalf("banana at %v, want the hero's cell + (2,0)", sp)
	}
}

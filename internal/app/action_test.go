package app

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// fsPack is a scene container holding only named blobs, which is all
// actionScript ever asks a container for. It records every lookup so a test can
// assert the order the candidates were tried in, and needs no game files.
type fsPack struct {
	files map[string][]byte
	asked []string
}

func newPack(names ...string) *fsPack {
	p := &fsPack{files: map[string][]byte{}}
	for _, n := range names {
		p.files[strings.ToUpper(n)] = []byte("MovieName \"" + n + "\";")
	}
	return p
}

func (p *fsPack) Entries() []types.Entry {
	out := make([]types.Entry, 0, len(p.files))
	for n := range p.files {
		out = append(out, types.Entry{Name: n})
	}
	return out
}

func (p *fsPack) Find(name string) (types.Entry, bool) {
	if _, ok := p.files[strings.ToUpper(name)]; ok {
		return types.Entry{Name: strings.ToUpper(name)}, true
	}
	return types.Entry{}, false
}

func (p *fsPack) FindExt(string) (types.Entry, bool) {
	return types.Entry{}, false
}

func (p *fsPack) Extract(e types.Entry) ([]byte, error) {
	return p.ExtractName(e.Name)
}

func (p *fsPack) ExtractName(name string) ([]byte, error) {
	p.asked = append(p.asked, strings.ToUpper(name))
	if d, ok := p.files[strings.ToUpper(name)]; ok {
		return d, nil
	}
	return nil, errors.New("no entry " + name)
}

// gameWith builds the slice of Game that actionScript actually touches: the
// quest state and the scene container. No Ebiten, no resources.
func gameWith(pack *fsPack, char, active string) *Game {
	gs := types.NewGameState()
	gs.ActiveChar, gs.Active = char, active
	return &Game{gs: gs, sceneC: pack}
}

// A script name spends exactly three letters per name, upper-cased. The
// truncation is lossy on purpose: it is what makes "condom" and "confr" (and
// "hand"/"handfr") reach the same CON/HAN scripts.
func TestScriptTokTruncates(t *testing.T) {
	cases := map[string]string{
		"hand":   "HAN",
		"handfr": "HAN",
		"condom": "CON",
		"confr":  "CON",
		"axe":    "AXE",
		"goleft": "GOL",
		"go":     "GO", // shorter than three letters is kept whole
		"":       "",
	}
	for in, want := range cases {
		if got := scriptTok(in); got != want {
			t.Errorf("scriptTok(%q) = %q, want %q", in, got, want)
		}
	}
}

// The candidate list is the item's own script first, then the bare-handed
// default, so a tool the object does not answer to still gets the plain
// reaction. An empty hand yields one candidate only — HAN *is* the default, and
// asking for it twice would just waste a container lookup.
func TestActionNames(t *testing.T) {
	cases := []struct {
		char, active, obj string
		want              []string
	}{
		{"Roby", "hand", "goleft", []string{"ROHANGOL"}},
		{"Roby", "axe", "wood", []string{"ROAXEWOO", "ROHANWOO"}},
		{"Frid", "confr", "goleft", []string{"FRCONGOL", "FRHANGOL"}},
		{"Frid", "handfr", "goleft", []string{"FRHANGOL"}},
		// Only Frid switches the prefix; anything else is the hero.
		{"", "hand", "goleft", []string{"ROHANGOL"}},
		{"frid", "hand", "goleft", []string{"FRHANGOL"}},
	}
	for _, c := range cases {
		got := actionNames(c.char, c.active, c.obj)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf(
				"actionNames(%q,%q,%q) = %v, want %v",
				c.char,
				c.active,
				c.obj,
				got,
				c.want,
			)
		}
	}
}

func TestActionScriptPicksTheItemScript(t *testing.T) {
	pack := newPack("ROAXEWOO.FS", "ROHANWOO.FS")
	name, raw, ok := gameWith(pack, "Roby", "axe").actionScript("wood")
	if !ok || name != "ROAXEWOO" {
		t.Fatalf("name = %q ok = %v, want ROAXEWOO", name, ok)
	}
	if !strings.Contains(string(raw), "ROAXEWOO") {
		t.Fatalf("raw came from the wrong entry: %q", raw)
	}
}

// The bare-handed script is the fallback, not a second choice tried in
// parallel: Friday holding his rope on an exit that only has FRHANGOL must
// still leave the scene rather than do nothing.
func TestActionScriptFallsBackToBareHand(t *testing.T) {
	pack := newPack("FRHANGOL.FS")
	name, _, ok := gameWith(pack, "Frid", "confr").actionScript("goleft")
	if !ok || name != "FRHANGOL" {
		t.Fatalf("name = %q ok = %v, want FRHANGOL", name, ok)
	}
	want := []string{"FRCONGOL.FS", "FRHANGOL.FS"}
	if strings.Join(pack.asked, ",") != strings.Join(want, ",") {
		t.Fatalf("lookups = %v, want %v", pack.asked, want)
	}
}

// A script redirects its own successor through a character variable named after
// itself (ROHANGOL ends with SetCharVar rohangol,"r1hangol"), which is how the
// hero's parting remark changes each time he leaves. The redirection must beat
// the original name, or he repeats the first line forever.
func TestActionScriptCharVarRedirectWins(t *testing.T) {
	pack := newPack("ROHANGOL.FS", "R1HANGOL.FS")
	g := gameWith(pack, "Roby", "hand")
	g.gs.SetCharVar("rohangol", "r1hangol")
	name, raw, ok := g.actionScript("goleft")
	if !ok || !strings.EqualFold(name, "r1hangol") {
		t.Fatalf("name = %q ok = %v, want r1hangol", name, ok)
	}
	if !strings.Contains(strings.ToUpper(string(raw)), "R1HANGOL") {
		t.Fatalf("raw came from the wrong entry: %q", raw)
	}
}

// A redirection that names a script this scene does not ship must not kill the
// action: the variable is global but the scripts are per-scene, so the base name
// stays the fallback.
func TestActionScriptCharVarMissingFallsBackToBase(t *testing.T) {
	pack := newPack("ROHANGOL.FS")
	g := gameWith(pack, "Roby", "hand")
	g.gs.SetCharVar("rohangol", "r9hangol")
	name, _, ok := g.actionScript("goleft")
	if !ok || name != "ROHANGOL" {
		t.Fatalf("name = %q ok = %v, want ROHANGOL", name, ok)
	}
}

// No script at all is a normal outcome, not an error: the caller falls back to
// examining the object.
func TestActionScriptMissingReportsNotOK(t *testing.T) {
	pack := newPack("ROHANAXE.FS")
	name, raw, ok := gameWith(pack, "Roby", "hand").actionScript("goleft")
	if ok || name != "" || raw != nil {
		t.Fatalf("got (%q, %v, %v), want no script", name, raw, ok)
	}
}

// cellsOf resolves object names to grid cells, standing in for Game.objCell.
func cellsOf(m map[string][2]int) func(string) (int, int, bool) {
	return func(name string) (int, int, bool) {
		if c, ok := m[name]; ok {
			return c[0], c[1], true
		}
		return 0, 0, false
	}
}

func TestAproachGoalForms(t *testing.T) {
	cells := cellsOf(map[string][2]int{"gorght": {7, 2}})
	clicked := &[2]int{3, 1}
	cases := []struct {
		name    string
		args    []string
		clicked *[2]int
		wx, wy  int
		wok     bool
	}{
		// Four args name the object to approach, and it is not always the one
		// clicked: SCENA4's ROHATGOL sends the hero to gorght because the hat
		// glide starts at the far edge. The named cell must win.
		{
			"named object plus offset",
			[]string{"Roby", "gorght", "1", "-1"},
			clicked, 8, 1, true,
		},
		// A name this scene does not place falls back to the clicked object, so
		// the action still happens next to the thing that was clicked.
		{
			"unknown name falls back",
			[]string{"Roby", "ghost", "1", "0"},
			clicked, 4, 1, true,
		},
		// An entry script has no clicked object to fall back on.
		{
			"unknown name without a click",
			[]string{"Roby", "ghost", "1", "0"},
			nil, 0, 0, false,
		},
		// Three args are an absolute cell; the clicked object is irrelevant.
		{"absolute cell", []string{"Roby", "4", "2"}, clicked, 4, 2, true},
		{"too few args", []string{"Roby"}, clicked, 0, 0, false},
	}
	for _, c := range cases {
		x, y, ok := aproachGoal(c.args, c.clicked, cells)
		if x != c.wx || y != c.wy || ok != c.wok {
			t.Errorf(
				"%s: aproachGoal = (%d,%d,%v), want (%d,%d,%v)",
				c.name,
				x,
				y,
				ok,
				c.wx,
				c.wy,
				c.wok,
			)
		}
	}
}

// recGrid is a real grid that counts the routing requests made of it, so a test
// can tell "walked nowhere" from "was never asked to walk".
type recGrid struct {
	interfaces.IGrid
	paths int
}

func (r *recGrid) Path(start, goal [2]int) [][2]int {
	r.paths++
	return r.IGrid.Path(start, goal)
}

// scena0 mirrors SCENA0's grid parameters, closed cells included: the pool sits
// on (5,2), which the scene itself declares impassable.
func scena0() *types.Scene {
	return &types.Scene{
		Size:        [2]int{1024, 400},
		LeftTopGrid: [2]int{15, 150},
		GridSize:    [2]int{144, 36},
		GridShift:   [2]int{46, -99},
		GridLength:  [2]int{8, 5},
		ZPerGrid:    8,
		ClosedVert: [][2]int{
			{4, 2},
			{4, 3},
			{5, 2},
			{5, 3},
			{6, 3},
			{5, 4},
			{6, 4},
			{7, 0},
			{7, 1},
			{7, 2},
			{7, 3},
			{7, 4},
		},
	}
}

// actRes answers every asset lookup with nothing: no movie frames, no shift and
// no container to load walk cycles from, so the walkers fail to start and the
// tests see the placement fallbacks rather than an animation.
type actRes struct{ noScenes }

func (actRes) MovieFrames(string) ([]*types.NGB, types.Palette) {
	return nil, types.Palette{}
}

func (actRes) MovieShift(string) [2]int { return [2]int{} }

// actGame builds the slice of Game that resolveAction and startObjectAction
// touch: quest state, scene container, parser, grid and the placed objects. No
// Ebiten and no art — LoadDecal over actRes yields zero frames and never asks
// for an image.
func actGame(
	sc *types.Scene, script string, objs map[string][2]int, char string,
) (*Game, *recGrid) {
	pack := &fsPack{files: map[string][]byte{}}
	for name, body := range map[string]string{
		"ROHANPOO.FS": script, "FRHANPOO.FS": script, "ROHANCNT.FS": script,
		"ROHANSTB.FS": script, "FRHANGOL.FS": script,
	} {
		pack.files[name] = []byte(body)
	}
	gs := types.NewGameState()
	gs.ActiveChar, gs.Active = char, "hand"
	if strings.EqualFold(char, "Frid") {
		gs.Active = "handfr"
	}
	grid := &recGrid{IGrid: use_cases.NewGrid(sc, sc.Size[0], sc.Size[1])}
	g := &Game{
		res:        actRes{},
		parser:     repositories.SceneParser{},
		gs:         gs,
		sceneC:     pack,
		grid:       grid,
		zper:       sc.ZPerGrid,
		cycleCache: map[string]*walkCycle{},
		roby:       walker{prefix: "RG", chr: "ROBY"},
		fridWalk:   walker{prefix: "FG", chr: "FRID"},
	}
	for name, cell := range objs {
		g.sceneObjs = append(g.sceneObjs, &sceneObj{
			ref: types.ObjectRef{Name: name, GX: cell[0], GY: cell[1]},
		})
	}
	return g, grid
}

// The pool's own cell is closed at scene load and ROHANPOO opens it in the very
// frame that routes the walk. The engine runs a frame's commands in order, so
// the SetVert has already reshaped the grid when the Aproach routes — route
// before that edit lands and NearestFree hands back (5,1), one row north: the
// hero then drinks 36px above the puddle. With no cycle art the walk resolves
// by placement, so the cell itself is the observable.
func TestActionOpensVertBeforeRouting(t *testing.T) {
	const src = "MovieName Rohanpoo.mv;\nShift 139,29;\nTotalFrames 30;\n" +
		"Frame 0,2;\nDelay 142;\nSetVert 5,2,open;\nAproach Roby, 5,2;\nEnd;"
	g, rec := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{1, 0}

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0) // frame 0: SetVert, then the Aproach routes
	if !g.grid.Valid(5, 2) {
		t.Error("frame 0's SetVert must have opened (5,2) before routing")
	}
	if g.cell != [2]int{5, 2} {
		t.Errorf("cell = %v, want the pool's own cell (5,2)", g.cell)
	}
	if rec.paths != 1 {
		t.Errorf("Path called %d times, want 1", rec.paths)
	}
}

// CAB_A1's RO*CNT refusals approach 2,0 — a wall cell no script ever opens — so
// the nearest-walkable fallback has to stay, and nothing may be opened for them.
func TestActionKeepsNearestFreeWhenNothingOpens(t *testing.T) {
	const src = "MovieName Cannotdo.mv;\nTotalFrames 14;\n" +
		"Frame 0,1;\nDelay 142;\nAproach Roby,4,2;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{1, 0}

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0)
	if g.cell == [2]int{4, 2} {
		t.Error("cell = (4,2), want the nearest walkable cell instead")
	}
	if g.grid.Blocked(g.cell[0], g.cell[1]) {
		t.Errorf("cell = %v is closed, want a walkable cell", g.cell)
	}
	if !g.grid.Blocked(4, 2) {
		t.Error("(4,2) must stay closed: the script opens nothing")
	}
}

// An Idiot/Whynot refusal carries no Aproach, so it plays where the hero
// stands. Counting the routing requests is what separates the fix from the
// bug: startWalk fails either way without the cycle art, so a nil path proves
// nothing on its own.
func TestActionNoAproachPlaysInPlace(t *testing.T) {
	const src = "MovieName Idiot.mv;\nTotalFrames 34;\n" +
		"Frame 0,2;\nDelay 142;\nText 486,1;\nEnd;"
	g, rec := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{1, 0}

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the refusal armed")
	}
	g.updateAction(0)
	if g.act == nil {
		t.Error("the refusal must still be playing")
	}
	if rec.paths != 0 {
		t.Errorf("Path called %d times, want 0", rec.paths)
	}
	if g.path != nil || g.cell != [2]int{1, 0} || g.roby.walking() {
		t.Errorf("the hero moved: path %v cell %v", g.path, g.cell)
	}
	if !g.grid.Blocked(5, 2) {
		t.Error("a script with no Aproach must not reshape the grid")
	}
}

// Friday's own action routes Friday, not the hero: 82 FR scripts approach only
// her, and the hero walking in her place is what used to put her movie roughly
// on target by accident.
func TestActionFridWalksNotRoby(t *testing.T) {
	const src = "MovieName Frhanpoo.mv;\nTotalFrames 12;\n" +
		"Frame 0,1;\nDelay 142;\nSetVert 5,2,open;\nAproach Frid, pool,0,0;\nEnd;"
	g, rec := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Frid")
	g.cell = [2]int{1, 0}

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want Friday's action armed")
	}
	if g.act == nil || !g.act.frid {
		t.Fatalf("act = %v, want one owned by Friday", g.act)
	}
	g.updateAction(0)
	if g.cell != [2]int{1, 0} || g.path != nil || g.roby.walking() {
		t.Errorf("the hero moved: path %v cell %v", g.path, g.cell)
	}
	// Friday has no walk cycles here, so fridWalkTo places her instead.
	if g.fridCell != [2]int{5, 2} {
		t.Errorf("fridCell = %v, want the pool's cell (5,2)", g.fridCell)
	}
	if rec.paths != 0 {
		t.Errorf(
			"Path called %d times, want 0: Friday uses PathStraight",
			rec.paths,
		)
	}
}

// A target the hero cannot reach must not turn the click into a no-op: the
// action still plays, just from where he is.
func TestActionUnreachableTargetStillPlays(t *testing.T) {
	sc := scena0()
	// Wall off column 3 entirely, leaving the pool's side unreachable from (0,0).
	for y := 0; y < sc.GridLength[1]; y++ {
		sc.ClosedVert = append(sc.ClosedVert, [2]int{3, y})
	}
	const src = "MovieName Rohanpoo.mv;\nTotalFrames 30;\n" +
		"Frame 0,2;\nDelay 142;\nSetVert 5,2,open;\nAproach Roby, 5,2;\nEnd;"
	g, rec := actGame(sc, src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{0, 0}

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed anyway")
	}
	g.updateAction(0)
	if g.act == nil {
		t.Error("an unreachable target must still leave the action playing")
	}
	if rec.paths != 1 {
		t.Errorf("Path called %d times, want 1", rec.paths)
	}
	if g.path != nil || g.cell != [2]int{0, 0} {
		t.Errorf("the hero moved: path %v cell %v", g.path, g.cell)
	}
}

// fakeCycle builds a playable two-frame walk cycle with no art on frame 0 and
// the given frame-0 events — enough for the walker to actually walk in a test.
func fakeCycle(events ...types.Command) *walkCycle {
	return &walkCycle{
		anim: &adapters.Animation{
			Frames: make([]*ebiten.Image, 2),
			BBox:   make([][2]int, 2),
		},
		frames: []*types.Frame{
			{Delay: 90, Events: events},
			{Delay: 90},
		},
	}
}

// stepX is the authored cell step a walk cycle fires on its frame 0.
func stepX(char string, d int) types.Command {
	return types.Command{
		Kw:   "shift",
		Args: []string{char, "X", strconv.Itoa(d)},
	}
}

// walkableCycles seeds the cycle cache with fake eastbound cycles for both
// characters, so startWalk succeeds without any game art: accelerate (no cell
// change), step, brake-with-step — the chain WalkCycles builds for a straight
// two-cell route.
func walkableCycles(g *Game) {
	for _, c := range []struct{ prefix, char string }{
		{"RG", "Roby"}, {"FG", "Frid"},
	} {
		g.cycleCache[c.prefix+"_56"] = fakeCycle()
		g.cycleCache[c.prefix+"_66"] = fakeCycle(stepX(c.char, 1))
		g.cycleCache[c.prefix+"_65"] = fakeCycle(stepX(c.char, 1))
	}
}

// An Aproach is a playback event: the movie pauses on it until the walk it
// started has finished, so the events of later frames must not fire while the
// character is still on his way. This is what used to be lost — the route was
// computed once, on the click, and an Aproach on frame 1+ never walked anyone.
func TestActionAproachPausesPlayback(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 90;\nAproach Roby, 3,0;\n" +
		"Frame 1,1;\nDelay 90;\nSetVar probe,1;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{1, 0}
	walkableCycles(g)

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0) // frame 0 fires; its Aproach starts the walk
	if g.act == nil || g.act.wait != waitRoby {
		t.Fatalf("act.wait = %v, want waitRoby", g.act)
	}
	if !g.roby.walking() {
		t.Fatal("the hero must be walking his cycles")
	}
	for i := 0; i < 200 && g.act != nil; i++ {
		if g.act.wait == waitRoby && g.gs.Var("probe") != 0 {
			t.Fatal("frame 1 fired while the walk was still underway")
		}
		g.updateWalk(0.09)
		g.updateAction(0.09)
	}
	if g.act != nil {
		t.Fatal("the action never finished")
	}
	if g.cell != [2]int{3, 0} {
		t.Errorf("cell = %v, want the walked-to (3,0)", g.cell)
	}
	if g.gs.Var("probe") != 1 {
		t.Error("frame 1 must have fired after the walk")
	}
}

// One frame routinely routes both characters — SCENA6's ROBT3STB sends Friday
// to the stumbling block and the hero one cell to its right — and the commands
// run in order: her walk holds the movie first, his follows from the paused
// frame's queue.
func TestActionQueueRoutesBothCharacters(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 2;\n" +
		"Frame 0,1;\nDelay 90;\nAproach Frid, stb, 0,0;\nAproach Roby, stb, 1,0;\n" +
		"Frame 1,1;\nDelay 90;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"stb": {2, 0}}, "Roby")
	g.cell = [2]int{0, 0}
	g.fridCell = [2]int{0, 0}
	walkableCycles(g)

	if !g.startObjectAction("stb") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0)
	if g.act == nil || g.act.wait != waitFrid {
		t.Fatalf("act.wait = %v, want waitFrid first", g.act)
	}
	if g.cell != [2]int{0, 0} {
		t.Fatal("the hero must not move before Friday has arrived")
	}
	for i := 0; i < 100 && g.aproachWalking(waitFrid); i++ {
		g.updateFridWalk(0.09)
	}
	if g.fridCell != [2]int{2, 0} {
		t.Fatalf("fridCell = %v, want the block's cell (2,0)", g.fridCell)
	}
	g.updateAction(0) // her walk is over: the paused frame routes the hero next
	if g.act == nil || g.act.wait != waitRoby {
		t.Fatalf("act.wait = %v, want waitRoby second", g.act)
	}
	for i := 0; i < 100 && g.roby.walking(); i++ {
		g.updateWalk(0.09)
	}
	if g.cell != [2]int{3, 0} {
		t.Errorf("cell = %v, want one right of the block (3,0)", g.cell)
	}
}

// An Aproach on a late frame is the fix this model exists for: SHIP2's
// ROHANOUT walks the hero on frame 2, and the FR*GOL exit pairs walk him on
// frame 1, after Friday. Without cycle art the walk resolves by placement, so
// the cell records that the event fired at all.
func TestActionLateAproachRoutesRoby(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 3;\n" +
		"Frame 0,1;\nDelay 142;\nAproach Frid, goleft, 0,0;\n" +
		"Frame 1,1;\nDelay 142;\nAproach Roby, goleft, 0,1;\n" +
		"Frame 2,1;\nDelay 142;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"goleft": {0, 1}}, "Frid")
	g.cell = [2]int{3, 0}
	g.fridCell = [2]int{2, 0}

	if !g.startObjectAction("goleft") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0) // frame 0: Friday lands on the exit (no art: placed)
	if g.fridCell != [2]int{0, 1} {
		t.Fatalf("fridCell = %v, want the exit cell (0,1)", g.fridCell)
	}
	if g.cell != [2]int{3, 0} {
		t.Fatal("the hero must not move on frame 0")
	}
	g.updateAction(0.15) // into frame 1: now the hero is routed too
	if g.cell != [2]int{0, 2} {
		t.Errorf("cell = %v, want the exit cell plus offset (0,2)", g.cell)
	}
}

// Skipping a cutscene that is paused on an Aproach must not deadlock the
// fast-forward loop: the walker lands on his goal at once, the rest of the
// frames rush by with their sounds and subtitles muted, and the final state
// matches what playing it out would have left.
func TestSkipCutsceneCutsAproachShort(t *testing.T) {
	const src = "MovieName Test.mv;\nTotalFrames 3;\n" +
		"Frame 0,1;\nDelay 90;\nAproach Roby, 3,0;\n" +
		"Frame 1,1;\nDelay 90;\nSetVar probe,1;\nSound \"rr001.wav\",1;\n" +
		"Frame 2,1;\nDelay 90;\nAproach Roby, 5,0;\nEnd;"
	g, _ := actGame(scena0(), src, map[string][2]int{"pool": {5, 2}}, "Roby")
	g.cell = [2]int{1, 0}
	fa := &fakeAudio{}
	g.audio = fa
	g.gs.UI["interrupt"] = true
	walkableCycles(g)

	if !g.startObjectAction("pool") {
		t.Fatal("startObjectAction = false, want the action armed")
	}
	g.updateAction(0)
	if g.act == nil || g.act.wait != waitRoby {
		t.Fatalf("act.wait = %v, want waitRoby", g.act)
	}
	g.updateWalk(0.09) // he is mid-route when the player skips
	if !g.skipCutscene() {
		t.Fatal("skipCutscene = false, want the skip taken")
	}
	if g.act != nil {
		t.Fatal("the action must be gone after the skip")
	}
	if g.gs.Var("probe") != 1 {
		t.Error("the skipped frames' state must still land")
	}
	// Frame 2's Aproach re-aims him past the frame-0 goal; the skip resolves
	// both walks instantly, so he ends where playing it out would leave him.
	if g.cell != [2]int{5, 0} {
		t.Errorf("cell = %v, want the final Aproach goal (5,0)", g.cell)
	}
	if g.roby.walking() {
		t.Error("no walk may survive the skip")
	}
	if len(fa.played) != 0 {
		t.Errorf("skipped sounds must stay muted, got %v", fa.played)
	}
	if fa.stops == 0 {
		t.Error("the skip must cut the effects that had already started")
	}
}

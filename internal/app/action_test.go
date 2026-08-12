package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/types"
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

// aproach fs with a single event on frame 0.
func aproachFS(args ...string) *types.FrameScript {
	return &types.FrameScript{Frames: []*types.Frame{{
		Index:  0,
		Events: []types.Command{{Kw: "aproach", Args: args}},
	}}}
}

func TestAproachTargetForms(t *testing.T) {
	cells := cellsOf(map[string][2]int{"gorght": {7, 2}})
	const clickedX, clickedY = 3, 1
	cases := []struct {
		name   string
		fs     *types.FrameScript
		wx, wy int
	}{
		// Four args name the object to approach, and it is not always the one
		// clicked: SCENA4's ROHATGOL sends the hero to gorght because the hat
		// glide starts at the far edge. The named cell must win.
		{
			"named object plus offset", aproachFS("Roby", "gorght", "1", "-1"),
			8, 1,
		},
		// A name this scene does not place falls back to the clicked object, so
		// the action still happens next to the thing that was clicked.
		{
			"unknown name falls back", aproachFS("Roby", "ghost", "1", "0"),
			clickedX + 1, clickedY,
		},
		// Three args are an absolute cell; the clicked object is irrelevant.
		{"absolute cell", aproachFS("Roby", "4", "2"), 4, 2},
		// No Aproach means "act where the object is".
		{"no aproach", &types.FrameScript{Frames: []*types.Frame{{
			Events: []types.Command{{Kw: "sound", Args: []string{"a.wav"}}},
		}}}, clickedX, clickedY},
		{"empty script", &types.FrameScript{}, clickedX, clickedY},
	}
	for _, c := range cases {
		x, y := aproachTarget(c.fs, clickedX, clickedY, cells)
		if x != c.wx || y != c.wy {
			t.Errorf(
				"%s: aproachTarget = (%d,%d), want (%d,%d)",
				c.name,
				x,
				y,
				c.wx,
				c.wy,
			)
		}
	}
}

// The first Aproach wins: a script may carry more than one across its frames
// (later ones re-aim a cutscene), but only the opening walk is a destination.
func TestAproachTargetTakesTheFirstEvent(t *testing.T) {
	fs := &types.FrameScript{Frames: []*types.Frame{
		{Index: 0},
		{Index: 6, Events: []types.Command{
			{Kw: "aproach", Args: []string{"Roby", "1", "1"}},
		}},
		{Index: 9, Events: []types.Command{
			{Kw: "aproach", Args: []string{"Roby", "5", "5"}},
		}},
	}}
	if x, y := aproachTarget(fs, 0, 0, cellsOf(nil)); x != 1 || y != 1 {
		t.Fatalf("aproachTarget = (%d,%d), want (1,1)", x, y)
	}
}

// aproachTarget matches the keyword exactly, so it depends on the parser
// lower-casing every event keyword. Run a real .FS through the real parser to
// keep the two ends of that contract together.
func TestAproachTargetOnParsedScript(t *testing.T) {
	const src = `Frame 0,1;  Delay 200; Aproach Roby,axe,0,0;
Frame 6,1;  Delay 400; DelObject CAB_B1, axe,2,2;
End;`
	var p repositories.SceneParser
	fs := p.ParseFrameScript(src)
	cells := cellsOf(map[string][2]int{"axe": {5, 3}})
	if x, y := aproachTarget(fs, 0, 0, cells); x != 5 || y != 3 {
		t.Fatalf("aproachTarget = (%d,%d), want the axe's cell (5,3)", x, y)
	}
}

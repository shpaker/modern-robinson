package repositories

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

func TestParseFrameScript(t *testing.T) {
	p := SceneParser{}
	fire := "ScriptName\tDO NOTE;\nMovieName\tFire.mv;\nShift\t90,227;\n" +
		"TotalFrames\t3;\nFrame 0,1;\nDelay 142;\nSound fire,4;\n" +
		"Frame 1,1;\nDelay 142;\nSound fire,4;\n" +
		"Frame 2,1;\nDelay 142;\nSound fire,4;\nEnd;"
	fs := p.ParseFrameScript(fire)
	if fs.MovieName != "Fire.mv" || fs.Shift != [2]int{90, 227} ||
		fs.Total != 3 {
		t.Fatalf("header = %q %v %d", fs.MovieName, fs.Shift, fs.Total)
	}
	if len(fs.Frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(fs.Frames))
	}
	f0 := fs.Frames[0]
	if f0.Delay != 142 || len(f0.Events) != 1 || f0.Events[0].Kw != "sound" ||
		len(f0.Events[0].Args) != 2 || f0.Events[0].Args[0] != "fire" {
		t.Errorf("frame0 = %+v", f0)
	}

	action := "ScriptName x;\nMovieName a.mv;\nShift 0,0;\nTotalFrames 1;\n" +
		"Frame 0,1;\nDelay 200;\nDelObject SCENA0, smoke, Roby,0,0;\nEnd;"
	fs2 := p.ParseFrameScript(action)
	if fs2.Frames[0].Events[0].Kw != "delobject" {
		t.Errorf("event = %q", fs2.Frames[0].Events[0].Kw)
	}
}

func TestParseSceneAndExits(t *testing.T) {
	root := testutil.GameRoot(t)
	res := NewResources(root)
	c := res.SceneContainer("SCENA0")
	if c == nil {
		t.Fatal("SCENA0 container not found")
	}
	scnData, err := c.ExtractName("SCENA0.SCN")
	if err != nil {
		t.Fatal(err)
	}
	p := SceneParser{}
	sc := p.ParseScene(string(scnData))

	if sc.LeftTopGrid != [2]int{15, 150} {
		t.Errorf("LeftTopGrid = %v, want [15 150]", sc.LeftTopGrid)
	}
	if sc.GridSize != [2]int{144, 36} {
		t.Errorf("GridSize = %v, want [144 36]", sc.GridSize)
	}
	if sc.GridShift != [2]int{46, -99} {
		t.Errorf("GridShift = %v, want [46 -99]", sc.GridShift)
	}
	// All seventeen rows of SCENA0's ObjectList, including the ambient driver
	// named "sound" — a name that collides with a script keyword.
	if len(sc.Objects) != 17 {
		t.Errorf("objects = %d, want 17", len(sc.Objects))
	}
	var hasSound bool
	for _, o := range sc.Objects {
		if strings.EqualFold(o.Name, "sound") {
			hasSound = true
		}
	}
	if !hasSound {
		t.Error("the object named \"sound\" was dropped as a keyword")
	}
	if sv, ok := sc.SoundVars["step"]; !ok || sv[0] != "step.wav" {
		t.Errorf("step sound = %v, want step.wav", sv)
	}
	// SCENA0's ambience is eight starred rows that all answer to "fon1", so the
	// pool only survives as a list — the name map keeps just the last of them.
	amb := sc.AmbientSounds()
	if len(amb) != 8 {
		t.Errorf("ambient pool = %d entries, want 8", len(amb))
	}
	for _, a := range amb {
		if !strings.EqualFold(a.Name, "fon1") || a.Voices != "5" {
			t.Errorf("ambient entry = %+v, want fon1 with 5 voices", a)
		}
		// The pool is only audible if the names it carries reach real audio.
		if res.Sound(a.Wav) == nil {
			t.Errorf("ambient wav %q resolves to nothing", a.Wav)
		}
	}

	left, right := p.SceneExits(c)
	if !left.OK || left.Scene != "SCENA1" || left.GX != 5 {
		t.Errorf("exitL = %+v, want SCENA1 (5,0)", left)
	}
	if !right.OK || right.Scene != "SCENA3" {
		t.Errorf("exitR = %+v, want SCENA3", right)
	}
}

// A SoundVariables block mixes plain entries, addressed by name, with the
// starred rows of the ambient pool, which share one name and are addressed by
// position — so the list has to keep the duplicates the name map folds away.
func TestParseSceneSoundVariables(t *testing.T) {
	scn := "SceneName\tTEST;\nSoundVariables\tstep,\"step.wav\",1;\n" +
		"\t\t\tfon1,\"s11.wav\",5,*;\n\t\t\tfon1,\"s12.wav\",5,*;\nEnd;"
	sc := SceneParser{}.ParseScene(scn)

	if len(sc.Sounds) != 3 {
		t.Fatalf("sounds = %d, want 3", len(sc.Sounds))
	}
	if sc.Sounds[0] != (types.SoundVar{Name: "step", Wav: "step.wav", Voices: "1"}) {
		t.Errorf("first entry = %+v, want plain step.wav", sc.Sounds[0])
	}
	amb := sc.AmbientSounds()
	if len(amb) != 2 || amb[0].Wav != "s11.wav" || amb[1].Wav != "s12.wav" {
		t.Errorf("ambient pool = %+v, want s11 then s12 in file order", amb)
	}
	// The plain lookup path still works, and still collapses the pool.
	if sv := sc.SoundVars["step"]; sv[0] != "step.wav" {
		t.Errorf("SoundVars[step] = %v, want step.wav", sv)
	}
	if sv := sc.SoundVars["fon1"]; sv[0] != "s12.wav" {
		t.Errorf("SoundVars[fon1] = %v, want the last row s12.wav", sv)
	}
}

// fakeContainer serves in-memory entries, enough for SceneExits.
type fakeContainer struct{ files map[string]string }

func (f fakeContainer) Entries() []types.Entry {
	var out []types.Entry
	for name := range f.files {
		out = append(out, types.Entry{Name: name})
	}
	return out
}

func (f fakeContainer) Find(name string) (types.Entry, bool) {
	if _, ok := f.files[name]; ok {
		return types.Entry{Name: name}, true
	}
	return types.Entry{}, false
}

func (fakeContainer) FindExt(string) (types.Entry, bool) {
	return types.Entry{}, false
}

func (f fakeContainer) Extract(e types.Entry) ([]byte, error) {
	return []byte(f.files[e.Name]), nil
}

func (f fakeContainer) ExtractName(name string) ([]byte, error) {
	return []byte(f.files[name]), nil
}

// A departure script's first GoScene may be the guarded island-discovery
// branch (Disc6); the bare-jump exit must carry the script's final,
// unconditional GoScene instead.
func TestSceneExitsSkipTheGuardedGoScene(t *testing.T) {
	gol := "ScriptName x;\nMovieName Ro_left.mv;\nTotalFrames 1;\n" +
		"Frame 0,1;\nDelay 90;\n" +
		"If Island,0;\nIf Ban2Hou,1;\n" +
		"SetVar Island,1;\nGoScene SCENA7, Roby, Disc6, 5, 0;\n" +
		"EndIf;\nEndIf;\n" +
		"GoScene SCENA7, Roby, Roin6, Frid, Frin6, 6, 0;\nEnd;\n"
	c := fakeContainer{files: map[string]string{"ROHANGOL.FS": gol}}
	left, _ := SceneParser{}.SceneExits(c)
	if !left.OK || left.Entry != "Roin6" || left.GX != 6 {
		t.Errorf("exitL = %+v, want the unconditional Roin6 (6,0)", left)
	}
}

// The same on the shipped data: SCENA2's goleft scripts carry the island
// check, and the exit must still name the plain arrival.
func TestSceneExitsOnGuardedSceneData(t *testing.T) {
	root := testutil.GameRoot(t)
	c := NewResources(root).SceneContainer("SCENA2")
	if c == nil {
		t.Fatal("SCENA2 container not found")
	}
	left, _ := SceneParser{}.SceneExits(c)
	if !left.OK || left.Scene != "SCENA7" {
		t.Fatalf("exitL = %+v, want SCENA7", left)
	}
	if strings.EqualFold(left.Entry, "Disc6") {
		t.Errorf("exitL.Entry = %q: the guarded discovery branch leaked "+
			"into the bare-jump exit", left.Entry)
	}
}

func TestParseStartup(t *testing.T) {
	inf := "SceneDirectory\t\\SCEN;\n" +
		"Scenes\t\tINT0,*;\n\t\tSCENA0;\n" +
		"IntVariables\tCrabNeed,0;\n\t\tTreeIs,1;\n\t\tMapParts,4;\n\t\tFind6,30;\n" +
		"CharVariables\trohanbgs,\"rohanbgs\";\n\t\trohanpop,\"hirobin\";\n" +
		"GridDebug 0;\nEnd;\n"
	vars, chars := SceneParser{}.ParseStartup(inf)
	if vars["treeis"] != 1 || vars["mapparts"] != 4 || vars["find6"] != 30 {
		t.Fatalf("int vars = %v", vars)
	}
	if vars["crabneed"] != 0 {
		t.Fatalf("crabneed = %d, want 0", vars["crabneed"])
	}
	if chars["rohanbgs"] != "rohanbgs" || chars["rohanpop"] != "hirobin" {
		t.Fatalf("char vars = %v", chars)
	}
}

func TestParseBar(t *testing.T) {
	txt := "DrawBar 1;\nBarLTWH 0,400,640,80;\nInventoryLTWH 303,410,146,60;\n" +
		"InvMaskLT 297,400;\nItemWH 48,60;\nItemsDisplayed 3;\n" +
		"LeftArrowBox 267,412,297,469;\nRightArrowBox 457,412,484,467;\n" +
		"Items hand,\"x\";\n\taxe,\"y\";\nEnd;\n"
	b := SceneParser{}.ParseBar(txt)
	if b.Rect != [4]int{0, 400, 640, 80} {
		t.Fatalf("Rect=%v", b.Rect)
	}
	if b.Inventory != [4]int{303, 410, 146, 60} {
		t.Fatalf("Inventory=%v", b.Inventory)
	}
	if b.InvMask != [2]int{297, 400} {
		t.Fatalf("InvMask=%v", b.InvMask)
	}
	if b.ItemW != 48 || b.ItemH != 60 || b.ItemsShown != 3 {
		t.Fatalf("item cell=%dx%d shown=%d", b.ItemW, b.ItemH, b.ItemsShown)
	}
	if b.LeftArrow != [4]int{267, 412, 297, 469} || b.RightArrow[0] != 457 {
		t.Fatalf("arrows L=%v R=%v", b.LeftArrow, b.RightArrow)
	}
	if len(b.Items) != 2 || b.Items[0] != "hand" || b.Items[1] != "axe" {
		t.Fatalf("items=%v", b.Items)
	}
}

package repositories

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
)

func TestParseFrameScript(t *testing.T) {
	p := SceneParser{}
	fire := "ScriptName\tDO NOTE;\nMovieName\tFire.mv;\nShift\t90,227;\n" +
		"TotalFrames\t3;\nFrame 0,1;\nDelay 142;\nSound fire,4;\n" +
		"Frame 1,1;\nDelay 142;\nSound fire,4;\n" +
		"Frame 2,1;\nDelay 142;\nSound fire,4;\nEnd;"
	fs := p.ParseFrameScript(fire)
	if fs.MovieName != "Fire.mv" || fs.Shift != [2]int{90, 227} || fs.Total != 3 {
		t.Fatalf("header = %q %v %d", fs.MovieName, fs.Shift, fs.Total)
	}
	if len(fs.Frames) != 3 {
		t.Fatalf("frames = %d, want 3", len(fs.Frames))
	}
	if !fs.Looping {
		t.Error("fire (no terminal command) must loop")
	}
	f0 := fs.Frames[0]
	if f0.Delay != 142 || len(f0.Events) != 1 || f0.Events[0].Kw != "sound" ||
		len(f0.Events[0].Args) != 2 || f0.Events[0].Args[0] != "fire" {
		t.Errorf("frame0 = %+v", f0)
	}

	action := "ScriptName x;\nMovieName a.mv;\nShift 0,0;\nTotalFrames 1;\n" +
		"Frame 0,1;\nDelay 200;\nDelObject SCENA0, smoke, Roby,0,0;\nEnd;"
	fs2 := p.ParseFrameScript(action)
	if fs2.Looping {
		t.Error("action with DelObject must be one-shot")
	}
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
	if len(sc.Objects) != 16 {
		t.Errorf("objects = %d, want 16", len(sc.Objects))
	}
	if sv, ok := sc.SoundVars["step"]; !ok || sv[0] != "step.wav" {
		t.Errorf("step sound = %v, want step.wav", sv)
	}

	left, right := p.SceneExits(c)
	if !left.OK || left.Scene != "SCENA1" || left.GX != 5 {
		t.Errorf("exitL = %+v, want SCENA1 (5,0)", left)
	}
	if !right.OK || right.Scene != "SCENA3" {
		t.Errorf("exitR = %+v, want SCENA3", right)
	}
}

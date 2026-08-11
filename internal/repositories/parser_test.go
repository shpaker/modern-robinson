package repositories

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
)

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

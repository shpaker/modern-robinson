package repositories

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/testutil"
)

func TestInitialVisibility(t *testing.T) {
	root := testutil.GameRoot(t)
	r := NewResources(root)
	p := SceneParser{}

	sceneObjects := func(scene string) []string {
		c := r.SceneContainer(scene)
		raw, err := c.ExtractName(scene + ".SCN")
		if err != nil {
			t.Fatalf("%s.SCN: %v", scene, err)
		}
		sc := p.ParseScene(string(raw))
		names := make([]string, len(sc.Objects))
		for i, o := range sc.Objects {
			names[i] = o.Name
		}
		return names
	}

	// SCENA0: pickups and the fire start hidden; landmarks start visible.
	v0 := r.InitialVisibility(sceneObjects("SCENA0"))
	if len(v0) == 0 {
		t.Fatal("no BGI cluster matched SCENA0")
	}
	for _, hidden := range []string{"fire", "smoke", "bgstone", "woods", "fraskcon"} {
		if v0[hidden] {
			t.Errorf("SCENA0 %s should start hidden", hidden)
		}
	}
	for _, vis := range []string{"cc1", "crb", "pool", "goleft"} {
		if !v0[vis] {
			t.Errorf("SCENA0 %s should start visible", vis)
		}
	}

	// PALACE: the chief and his idle native are visible; quest variants hidden.
	vp := r.InitialVisibility(sceneObjects("PALACE"))
	if len(vp) == 0 {
		t.Fatal("no BGI cluster matched PALACE")
	}
	for _, vis := range []string{"king", "man", "tr2", "organ", "tubs"} {
		if !vp[vis] {
			t.Errorf("PALACE %s should start visible", vis)
		}
	}
	for _, hidden := range []string{"kng", "kingtalk", "endtalk", "mn1", "tr1", "mus1", "mus3", "pipe"} {
		if vp[hidden] {
			t.Errorf("PALACE %s should start hidden", hidden)
		}
	}
}

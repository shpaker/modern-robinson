package app

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// Leaving SCENA0 by ROHANGOL with the full scene environment running (object
// FonScripts, idle chatter, Friday) must reach its GoScene: the exit once hung
// waiting on a walk that was not the movie's owner's.
func TestLeavingScena0ReachesGoScene(t *testing.T) {
	res := repositories.NewResources(testutil.GameRoot(t))
	c := res.SceneContainer("SCENA0")
	parser := repositories.SceneParser{}
	scn, _ := c.ExtractName("SCENA0.SCN")
	sc := parser.ParseScene(string(scn))
	g := &Game{
		res:        res,
		parser:     parser,
		gs:         types.NewGameState(),
		audio:      &fakeAudio{},
		sceneName:  "SCENA0",
		sceneC:     c,
		sc:         sc,
		grid:       use_cases.NewGrid(sc, sc.Size[0], sc.Size[1]),
		w:          sc.Size[0],
		h:          sc.Size[1],
		zper:       sc.ZPerGrid,
		cycleCache: map[string]*walkCycle{},
		roby:       walker{prefix: "RG", chr: "ROBY"},
		fridWalk:   walker{prefix: "FG", chr: "FRID"},
	}
	g.objects = map[string]*types.SceneObject{}
	for _, e := range c.Entries() {
		up := strings.ToUpper(e.Name)
		if strings.HasSuffix(up, ".OB") {
			if d, err := c.Extract(e); err == nil {
				ob := parser.ParseObject(string(d))
				g.objects[strings.ToLower(e.Name[:len(e.Name)-3])] = ob
			}
		}
	}
	g.fsByName = fonScripts(c)
	g.sceneObjs = loadSceneObjects(res, g.pal, g.parseFS, sc, g.objects,
		g.fsByName,
		func(string) bool { return false })
	g.applyObjectBlocking()
	g.buildHotspots()
	g.loadCharacter()
	g.cell = [2]int{1, 2}
	g.gs.Active, g.gs.ActiveChar = "hand", "Roby"

	if !g.startObjectAction("goleft") {
		t.Fatal("no ROHANGOL")
	}
	for i := 0; i < 6000; i++ {
		dt := 1.0 / 60
		for _, s := range g.sceneObjs {
			g.applyEvents(s.update(dt))
		}
		g.updateWalk(dt)
		g.moving = g.roby.walking()
		g.updateFridWalk(dt)
		g.updateAction(dt)
		g.updateIdle(dt)
		if g.pending != nil {
			return
		}
	}
	t.Fatalf("stuck after 6000 ticks: act=%v fridPath=%v cell=%v",
		g.act != nil, g.fridPath, g.cell)
}

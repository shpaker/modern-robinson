package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// sceneObj is a live animated object on the scene: its FonScript drives frame
// timing (looping ambient or one-shot), rendered as a screen-space decal.
type sceneObj struct {
	ref     types.ObjectRef
	ob      *types.SceneObject
	shift   [2]int // FonScript origin (used for hotspot placement)
	z       int    // draw order = gy*ZPerGrid + ZCoord
	frames  []adapters.DecalFrame
	player  *use_cases.Player
	visible bool
}

// loadSceneObjects builds the live objects for a scene's ObjectList, wiring each
// object's FonScript to its movie frames and a player.
func loadSceneObjects(res interfaces.IResources, parser interfaces.ISceneParser,
	c interfaces.IContainer, sc *types.Scene, objects map[string]*types.SceneObject,
) []*sceneObj {
	fsByName := map[string][]byte{}
	for _, e := range c.Entries() {
		if strings.HasSuffix(strings.ToUpper(e.Name), ".FS") {
			if d, err := c.Extract(e); err == nil {
				fsByName[strings.ToLower(e.Name[:len(e.Name)-3])] = d
			}
		}
	}
	zper := sc.ZPerGrid
	if zper == 0 {
		zper = 8
	}
	var out []*sceneObj
	for _, ref := range sc.Objects {
		ob := objects[strings.ToLower(ref.Name)]
		if ob == nil {
			continue
		}
		inst := &sceneObj{ref: ref, ob: ob, z: ref.GY*zper + ob.Z}
		fon := strings.ToLower(ob.FonScript)
		if raw, ok := fsByName[fon]; fon != "" && fon != "null" && ok {
			fs := parser.ParseFrameScript(string(raw))
			inst.shift = fs.Shift
			// Auto-play only looping FonScripts (ambient loops + 1-frame statics);
			// one-shot scripts (smoke/cutscenes) stay hidden until triggered.
			if fs.MovieName != "" && fs.Looping {
				inst.frames = adapters.LoadDecal(res, fs.MovieName)
				inst.player = use_cases.NewPlayer(fs)
				inst.visible = len(inst.frames) > 0
			}
		}
		out = append(out, inst)
	}
	return out
}

// update advances the object's animation and returns the events it fired.
func (s *sceneObj) update(dt float64) []types.Command {
	if s.player == nil {
		return nil
	}
	ev := s.player.Update(dt)
	if s.player.Done() {
		s.visible = false // one-shot finished (e.g. smoke self-deletes)
	}
	return ev
}

// draw blits the current animation frame at its screen offset.
func (s *sceneObj) draw(screen *ebiten.Image) {
	if !s.visible || s.player == nil || len(s.frames) == 0 {
		return
	}
	i := s.player.FrameIndex()
	if i < 0 || i >= len(s.frames) || s.frames[i].Img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(s.frames[i].X), float64(s.frames[i].Y))
	screen.DrawImage(s.frames[i].Img, op)
}

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
	visible bool // has a frame to draw this tick
	removed bool // taken/consumed (DelObject) — no sprite, no hotspot
}

// fonScripts reads every .FS entry of a scene container, keyed by lower-case
// base name, so objects can wire up their FonScript animations.
func fonScripts(c interfaces.IContainer) map[string][]byte {
	out := map[string][]byte{}
	for _, e := range c.Entries() {
		if strings.HasSuffix(strings.ToUpper(e.Name), ".FS") {
			if d, err := c.Extract(e); err == nil {
				out[strings.ToLower(e.Name[:len(e.Name)-3])] = d
			}
		}
	}
	return out
}

// buildSceneObj builds one live object, wiring its FonScript to movie frames and
// a player when the script is a looping ambient animation. Returns nil if the
// object has no .OB definition.
func buildSceneObj(res interfaces.IResources, parser interfaces.ISceneParser,
	fsByName map[string][]byte, zper int, ref types.ObjectRef,
	ob *types.SceneObject,
) *sceneObj {
	if ob == nil {
		return nil
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
	return inst
}

// loadSceneObjects builds the live objects for a scene's ObjectList, skipping
// any the quest state records as already taken.
func loadSceneObjects(res interfaces.IResources, parser interfaces.ISceneParser,
	sc *types.Scene, objects map[string]*types.SceneObject,
	fsByName map[string][]byte, gone func(obj string) bool,
) []*sceneObj {
	zper := sc.ZPerGrid
	if zper == 0 {
		zper = 8
	}
	var out []*sceneObj
	for _, ref := range sc.Objects {
		if gone(ref.Name) {
			continue
		}
		if inst := buildSceneObj(res, parser, fsByName, zper,
			ref, objects[strings.ToLower(ref.Name)]); inst != nil {
			out = append(out, inst)
		}
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

// draw blits the current animation frame at its screen offset, shifted by the
// camera offset xoff (negative camX).
func (s *sceneObj) draw(screen *ebiten.Image, xoff int) {
	if !s.visible || s.player == nil || len(s.frames) == 0 {
		return
	}
	i := s.player.FrameIndex()
	if i < 0 || i >= len(s.frames) || s.frames[i].Img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(s.frames[i].X+xoff), float64(s.frames[i].Y))
	screen.DrawImage(s.frames[i].Img, op)
}

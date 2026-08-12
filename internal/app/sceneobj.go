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
	shift   [2]int // FonScript Shift: the sprite hotspot in canvas space
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
		// The FonScript's own Shift wins; the movie's .SCR origin is the
		// fallback for the scripts that omit it (see Game.movieShift).
		inst.shift = res.MovieShift(fs.MovieName)
		if fs.Shift != ([2]int{}) {
			inst.shift = fs.Shift
		}
		if fs.MovieName != "" {
			inst.frames = adapters.LoadDecal(res, fs.MovieName)
			inst.visible = len(inst.frames) > 0
			// An object's FonScript is ambient scenery, so it loops, unless its
			// last frame closes the script out (a driver like START.FS, which
			// holds its final frame after firing GoScene).
			inst.player = use_cases.NewPlayer(fs, !endsScript(fs))
		}
	}
	return inst
}

// endsScript reports whether a script's last frame closes it out -- the ambient
// loops just run out of frames, while a driver finishes with a world change.
func endsScript(fs *types.FrameScript) bool {
	if len(fs.Frames) == 0 {
		return false
	}
	for _, ev := range fs.Frames[len(fs.Frames)-1].Events {
		switch strings.ToLower(ev.Kw) {
		case "goscene", "delobject", "deleteobject", "showchar", "startgame":
			return true
		}
	}
	return false
}

// loadSceneObjects builds the live objects for a scene's ObjectList, skipping
// any the quest state records as taken plus any BEGIN.BGI marks as initially
// hidden (they wait for a CreateObject).
func loadSceneObjects(res interfaces.IResources, parser interfaces.ISceneParser,
	sc *types.Scene, objects map[string]*types.SceneObject,
	fsByName map[string][]byte, gone func(obj string) bool,
) []*sceneObj {
	zper := sc.ZPerGrid
	if zper == 0 {
		zper = 8
	}
	names := make([]string, len(sc.Objects))
	for i, ref := range sc.Objects {
		names[i] = ref.Name
	}
	initial := res.InitialVisibility(names)
	var out []*sceneObj
	for _, ref := range sc.Objects {
		if gone(ref.Name) {
			continue
		}
		if v, ok := initial[strings.ToLower(ref.Name)]; ok && !v {
			continue // hidden at game start until a CreateObject
		}
		if inst := buildSceneObj(res, parser, fsByName, zper,
			ref, objects[strings.ToLower(ref.Name)]); inst != nil {
			out = append(out, inst)
		}
	}
	return out
}

// update advances the object's animation and returns the events it fired.
// A finished one-shot holds its last frame (self-removing scripts end with a
// DelObject which hides the object through the interpreter).
func (s *sceneObj) update(dt float64) []types.Command {
	if s.player == nil {
		return nil
	}
	return s.player.Update(dt)
}

// draw blits the current animation frame at the engine's placement: the sprite
// canvas origin is anchor(cell) - FonScript.Shift, and each frame's cropped
// bitmap sits at its own (X,Y) within that canvas. xoff is the camera offset.
func (s *sceneObj) draw(screen *ebiten.Image, grid interfaces.IGrid, xoff int) {
	if !s.visible || len(s.frames) == 0 {
		return
	}
	i := 0
	if s.player != nil {
		i = s.player.FrameIndex()
	}
	if i < 0 || i >= len(s.frames) || s.frames[i].Img == nil {
		return
	}
	ax, ay := grid.ToScreen(s.ref.GX, s.ref.GY)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(
		float64(ax-s.shift[0]+s.frames[i].X+xoff),
		float64(ay-s.shift[1]+s.frames[i].Y),
	)
	screen.DrawImage(s.frames[i].Img, op)
}

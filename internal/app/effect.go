package app

import (
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// applyEvents runs a batch of frame events through the quest interpreter
// (evaluating If/EndIf and applying state) and enacts the world/presentation
// commands that survive.
func (g *Game) applyEvents(cmds []types.Command) {
	if len(cmds) == 0 {
		return
	}
	for _, c := range g.interp.Exec(cmds, g.gs) {
		g.applyEffect(c)
	}
}

// applyEffect enacts one surviving command against the live scene and engine.
// Pure state commands (Set/If/AddItem/…) never reach here — the interpreter
// consumes them.
func (g *Game) applyEffect(c types.Command) {
	kw := strings.ToLower(c.Kw)
	if g.fridEffect(kw, c.Args) {
		return // a Frid-targeted Set/Aproach/Show/Hide
	}
	switch kw {
	case "sound":
		g.playSound(c.Args)
	case "text":
		if len(c.Args) > 0 {
			id, _ := strconv.Atoi(c.Args[0])
			if s := g.textLine(id); s != "" {
				// Rough read-time heuristic: base + per-character.
				g.msg, g.msgT = s, 1.2+0.05*float64(len(s))
			}
		}
	case "createobject":
		g.createObject(c.Args)
	case "delobject", "deleteobject":
		g.delObject(c.Args)
	case "goscene":
		g.gosceneFromEvent(c.Args)
	case "hidechar":
		if robyTarget(c.Args) {
			g.charHidden = true
		}
	case "showchar":
		if robyTarget(c.Args) {
			g.charHidden = false
		}
	case "setvert":
		g.setVert(c.Args)
	case "setrest":
		g.setRest(c.Args)
	case "setmouse", "setmap", "lockbar", "setbar", "showcursor", "interrupt":
		g.setToggle(strings.ToLower(c.Kw), c.Args)
	case "startgame":
		g.startMinigame(c.Args)
	case "set":
		g.setCharCoord(c.Args)
	case "setmusic":
		g.setMusic(c.Args)
	}
	// aproach/shift still drive the walk+action phase.
}

// setCharCoord teleports a character's grid coordinate: Set char,X|Y|Z,n.
// Entry scripts use it to place the hero on arrival. Friday is stage 3e.
func (g *Game) setCharCoord(args []string) {
	if len(args) < 3 || !strings.EqualFold(args[0], "Roby") {
		return
	}
	n := atoiArg(args[2])
	switch strings.ToUpper(args[1]) {
	case "X":
		g.cell[0] = n
	case "Y":
		g.cell[1] = n
	default:
		return // Z: draw-order tweak, charZCoord covers the common case
	}
	px, py := g.grid.ToScreen(g.cell[0], g.cell[1])
	g.pos = [2]float64{float64(px), float64(py)}
	g.clampCamera()
}

// setToggle records an ON/OFF UI switch (map access, mouse lock, bar lock...).
func (g *Game) setToggle(kw string, args []string) {
	if len(args) < 1 {
		return
	}
	on := strings.EqualFold(args[0], "ON")
	switch kw {
	case "setmouse":
		g.gs.UI["mouse"] = on
	case "setmap":
		g.gs.UI["map"] = on
	case "lockbar":
		g.gs.UI["barlock"] = on
	case "setbar":
		g.gs.UI["bar"] = on
	case "showcursor":
		g.gs.UI["cursor"] = on
	case "interrupt":
		g.gs.UI["interrupt"] = on
	}
}

// createObject spawns an object into a scene (persisting it) and, when that is
// the current scene, into the live world. Forms:
//
//	CreateObject scene,obj,gx,gy          absolute cell
//	CreateObject scene,obj,char,dx,dy     cell relative to the character
func (g *Game) createObject(args []string) {
	if len(args) < 4 {
		return
	}
	scene, obj := args[0], args[1]
	var gx, gy int
	if len(args) >= 5 { // char-relative
		gx, gy = g.cell[0]+atoiArg(args[3]), g.cell[1]+atoiArg(args[4])
	} else {
		gx, gy = atoiArg(args[2]), atoiArg(args[3])
	}
	g.gs.MarkSpawn(scene, obj, gx, gy)
	if strings.EqualFold(scene, g.sceneName) {
		g.spawnObject(obj, gx, gy)
		g.buildHotspots()
	}
}

// spawnObject builds one live object at a cell and appends it to the scene,
// loading its .OB from the container if not already parsed. It is idempotent:
// an object already live is not duplicated.
func (g *Game) spawnObject(obj string, gx, gy int) {
	for _, s := range g.sceneObjs {
		if strings.EqualFold(s.ref.Name, obj) && !s.removed {
			return
		}
	}
	ob := g.objects[strings.ToLower(obj)]
	if ob == nil {
		if d, err := g.sceneC.ExtractName(strings.ToUpper(obj) + ".OB"); err == nil {
			ob = g.parser.ParseObject(string(d))
			g.objects[strings.ToLower(ob.Name)] = ob
		}
	}
	ref := types.ObjectRef{Name: obj, GX: gx, GY: gy}
	if inst := buildSceneObj(g.res, g.parser, g.fsByName, g.zper, ref, ob); inst != nil {
		g.sceneObjs = append(g.sceneObjs, inst)
	}
}

// delObject removes an object from a scene, persisting the removal. Form:
// DelObject scene,obj,... — args[1] is the object name.
func (g *Game) delObject(args []string) {
	if len(args) < 2 {
		if len(args) == 1 { // bare DelObject obj
			g.gs.MarkGone(g.sceneName, args[0])
			g.hideObject(args[0])
		}
		return
	}
	scene, obj := args[0], args[1]
	g.gs.MarkGone(scene, obj)
	if strings.EqualFold(scene, g.sceneName) {
		g.hideObject(obj)
	}
}

// setVert toggles a walk cell's passability. Form: SetVert gx,gy,open|close|closed.
func (g *Game) setVert(args []string) {
	if len(args) < 3 {
		return
	}
	g.grid.SetVert(
		atoiArg(args[0]),
		atoiArg(args[1]),
		strings.EqualFold(args[2], "open"),
	)
}

// setMusic switches the looping background track: SetMusic id|none. Track ids
// are WAV names in the wave bank (music1 -> MUSIC1.WAV, mpalace, mfrid, ...).
func (g *Game) setMusic(args []string) {
	if len(args) < 1 {
		return
	}
	g.startMusic(args[0])
}

// startMusic resolves and loops a music track by id; "none" stops music.
func (g *Game) startMusic(id string) {
	if strings.EqualFold(id, "none") || id == "" {
		g.audio.StopMusic()
		return
	}
	file := id
	if !strings.Contains(file, ".") {
		file += ".wav"
	}
	g.audio.PlayMusic(strings.ToLower(file), g.res.Sound(file))
}

// robyTarget reports whether a HideChar/ShowChar command targets the main
// character (Roby); Friday's variants are handled by fridEffect.
func robyTarget(args []string) bool {
	return len(args) == 0 || strings.EqualFold(args[0], "Roby")
}

func atoiArg(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

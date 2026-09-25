package app

import (
	"strconv"
	"strings"

	"github.com/shpaker/modern-robinson/internal/types"
)

// applyEvents runs a batch of frame events through the quest interpreter
// (evaluating If/EndIf and applying state) and enacts the world/presentation
// commands that survive. Action-movie frames go through enqueueAction instead,
// which honours Aproach pauses.
func (g *Game) applyEvents(cmds []types.Command) {
	if len(cmds) == 0 {
		return
	}
	out, rest := g.exec(cmds)
	// A StartGame in the batch suspends the script: the tail is re-judged once
	// the minigame has written its result, so the success branch can actually be
	// taken. Hand it over before enacting anything, because StartGame runs
	// inside the loop below and its own "no assets" fallback finishes the tail
	// itself — it has to be able to see it.
	g.mgResume = rest
	for _, c := range out {
		g.applyEffect(c)
	}
}

// exec runs commands through the quest interpreter. When they rebuilt the bar
// (an item came or went, control changed hands) the inventory window goes back
// to its first slot, as the engine's rebuild does (0x4044e0 zeroes the
// scroll): a delete made while scrolled no longer leaves a gap.
func (g *Game) exec(
	cmds []types.Command,
) ([]types.Command, []types.Command) {
	rev := g.gs.InvRev
	out, rest := g.interp.Exec(cmds, g.gs)
	if g.gs.InvRev != rev {
		g.invScroll = 0
	}
	return out, rest
}

// presentational reports whether a command only speaks to the player, carrying
// no state or world change a later frame could depend on. A skipped cutscene
// mutes these: the whole remainder fires within one tick, so its sounds and
// subtitles would all land at once (see skipCutscene).
func presentational(kw string) bool {
	switch kw {
	case "sound", "text":
		return true
	}
	return false
}

// applyEffect enacts one surviving command against the live scene and engine.
// Pure state commands (Set/If/AddItem/…) never reach here — the interpreter
// consumes them.
func (g *Game) applyEffect(c types.Command) {
	kw := strings.ToLower(c.Kw)
	g.trace(c)
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
				g.ctl.heard(s)
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
	case "shiftscreen":
		g.shiftScreen(c.Args)
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
	case "clearscreen":
		// The original wipes the movie frame before leaving a cutscene
		// (INT1.FS frame 313). Here the scene swap repaints anyway and
		// loadScene cuts the sounds, so there is nothing left to do.
	}
	// An action movie's aproach never reaches here (playActionQueue pauses on
	// it); shift is the walk cycles' own step (see applyWalkEvents).
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
	case "Z":
		// The walk cycles choreograph this per step so the hero can pass
		// behind same-row props; it is his sub-slot, not his row.
		g.robyZ = n
		return
	default:
		return
	}
	// The view stays where it is: Set moves no scroll (0x41a4f6), GoScene
	// has already put the screen where the arrival wants it.
	px, py := g.grid.ToScreen(g.cell[0], g.cell[1])
	g.pos = [2]float64{float64(px), float64(py)}
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
//
// The character is the one named, not the hero: ROCON puts Friday's double
// (Fraskcon, Frid,0,0) on her own cell (0x41e9a0 adds char+0x254/+0x258).
func (g *Game) createObject(args []string) {
	if len(args) < 4 {
		return
	}
	scene, obj := args[0], args[1]
	var gx, gy int
	if len(args) >= 5 { // char-relative
		base := g.cell
		if strings.EqualFold(args[2], "Frid") {
			base = g.fridCell
		}
		gx, gy = base[0]+atoiArg(args[3]), base[1]+atoiArg(args[4])
	} else {
		gx, gy = atoiArg(args[2]), atoiArg(args[3])
	}
	g.gs.MarkSpawn(scene, obj, gx, gy)
	if strings.EqualFold(scene, g.sceneName) {
		g.spawnObject(obj, gx, gy)
		g.buildHotspots()
	}
}

// spawnObject places an object at a cell the way the engine's CreateObject
// does (0x41e9a0): the object keeps its slot in the scene, takes the new cell
// and starts its FonScript over, so one already on stage moves instead of
// being duplicated. An object the scene holds no slot for is appended, its .OB
// loaded from the container if not already parsed.
func (g *Game) spawnObject(obj string, gx, gy int) {
	slot := -1
	for i, s := range g.sceneObjs {
		if strings.EqualFold(s.ref.Name, obj) {
			slot = i
			break
		}
	}
	ob := g.objects[strings.ToLower(obj)]
	if ob == nil {
		if d, err := g.sceneC.ExtractName(strings.ToUpper(obj) + ".OB"); err == nil {
			ob = g.parser.ParseObject(string(d))
			g.objects[strings.ToLower(ob.Name)] = ob
		}
	}
	ref := types.ObjectRef{Name: obj}
	if slot >= 0 {
		ref = g.sceneObjs[slot].ref
	}
	ref.GX, ref.GY = gx, gy
	inst := buildSceneObj(g.res, g.pal, g.parseFS, g.fsByName, g.zper, ref, ob)
	if inst == nil {
		return
	}
	if slot < 0 {
		g.sceneObjs = append(g.sceneObjs, inst)
	} else {
		if old := g.sceneObjs[slot]; !old.removed {
			g.setObjectBlocking(old, true) // its walls leave the old cell
		}
		g.sceneObjs[slot] = inst
	}
	g.setObjectBlocking(inst, false) // it brings its walls along
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

// vertArgs decodes a SetVert command's arguments: gx,gy,open|close|closed. It is
// shared with preAproachVerts, which has to read the same edits a frame ahead of
// the engine to route an Aproach onto a cell the frame opens (see action.go).
func vertArgs(args []string) (types.Vert, bool) {
	if len(args) < 3 {
		return types.Vert{}, false
	}
	return types.Vert{
		GX:   atoiArg(args[0]),
		GY:   atoiArg(args[1]),
		Open: strings.EqualFold(args[2], "open"),
	}, true
}

// setVert toggles a walk cell's passability. Form: SetVert gx,gy,open|close|closed.
func (g *Game) setVert(args []string) {
	v, ok := vertArgs(args)
	if !ok {
		return
	}
	g.grid.SetVert(v.GX, v.GY, v.Open)
	// survives leaving the scene
	g.gs.MarkVert(g.sceneName, v.GX, v.GY, v.Open)
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

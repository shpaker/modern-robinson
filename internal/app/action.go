package app

import (
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// actionPlay is a character action triggered by clicking an object: the hero
// walks to the target cell, then the action movie plays (frames + events).
type actionPlay struct {
	fs      *types.FrameScript
	frames  []adapters.DecalFrame
	shift   [2]int // the movie's canvas hotspot (.SCR origin)
	player  *use_cases.Player
	target  [2]int
	started bool // true once the walk finished and the movie is playing
}

// actionScript names the script a click runs: <RO|FR> + the held item's first
// three letters + the object's first three (ROHANGOL is Roby, bare hand, the
// left exit; ROAXEWOO is Roby chopping wood with the axe). A script may then
// redirect its own successor through a character variable named after itself —
// ROHANGOL ends with SetCharVar rohangol,"r1hangol", which is how the hero's
// remark changes each time he leaves — so that redirection wins when set.
// Falls back to the bare-handed script when the held item has none.
func (g *Game) actionScript(objName string) (string, []byte, bool) {
	char := "RO"
	if strings.EqualFold(g.gs.ActiveChar, "Frid") {
		char = "FR"
	}
	tok := func(s string) string {
		s = strings.ToUpper(s)
		if len(s) > 3 {
			s = s[:3]
		}
		return s
	}
	obj := tok(objName)
	names := []string{char + tok(g.gs.Active) + obj}
	if tok(g.gs.Active) != "HAN" {
		names = append(names, char+"HAN"+obj) // the empty-handed default
	}
	for _, base := range names {
		name := base
		if v := g.gs.CharVar(strings.ToLower(base)); v != "" {
			name = v // the script handed off to a variant of itself
		}
		if raw, err := g.sceneC.ExtractName(strings.ToUpper(name) + ".FS"); err == nil {
			return name, raw, true
		}
		if name != base {
			if raw, err := g.sceneC.ExtractName(base + ".FS"); err == nil {
				return base, raw, true
			}
		}
	}
	return "", nil, false
}

// resolveAction builds the action a click on an object runs: walk to its Aproach
// cell, then play the script's movie and events.
func (g *Game) resolveAction(objName string) *actionPlay {
	if g.sceneC == nil {
		return nil
	}
	_, raw, ok := g.actionScript(objName)
	if !ok {
		return nil
	}
	fs := g.parser.ParseFrameScript(string(raw))
	if fs.MovieName == "" {
		return nil
	}
	// Click actions are always one-shot; looping is a FonScript (ambient)
	// property, not a script one (the terminal-command heuristic can mislabel
	// an action whose world-change fires on an early frame, e.g. ROHANBGS).
	fs.Looping = false
	ocx, ocy, ok := g.objCell(objName)
	if !ok {
		return nil
	}
	tx, ty := aproachTarget(fs, ocx, ocy, g.objCell)
	fx, fy, _ := g.grid.NearestFree(tx, ty)
	return &actionPlay{
		fs:     fs,
		frames: adapters.LoadDecal(g.res, fs.MovieName),
		shift:  g.res.MovieShift(fs.MovieName),
		player: use_cases.NewPlayer(fs),
		target: [2]int{fx, fy},
	}
}

// aproachTarget reads the first Aproach event to find the action's target cell.
// Forms: (Roby, obj, dx, dy) -> that object's cell + offset; (Roby, gx, gy) ->
// absolute. The object named in the event is not always the one clicked --
// SCENA4's ROHATGOL sends the hero to gorght because the hat glide starts at
// the far edge -- so cellOf resolves the name and the clicked object's own cell
// is only the fallback.
func aproachTarget(
	fs *types.FrameScript, ocx, ocy int, cellOf func(string) (int, int, bool),
) (int, int) {
	for _, fr := range fs.Frames {
		for _, ev := range fr.Events {
			if ev.Kw != "aproach" {
				continue
			}
			a := ev.Args
			switch len(a) {
			case 4:
				dx, _ := strconv.Atoi(a[2])
				dy, _ := strconv.Atoi(a[3])
				tx, ty := ocx, ocy
				if cx, cy, ok := cellOf(a[1]); ok {
					tx, ty = cx, cy
				}
				return tx + dx, ty + dy
			case 3:
				gx, _ := strconv.Atoi(a[1])
				gy, _ := strconv.Atoi(a[2])
				return gx, gy
			}
		}
	}
	return ocx, ocy
}

// objCell returns the scene-grid cell of an object by name.
func (g *Game) objCell(name string) (int, int, bool) {
	name = strings.ToLower(name)
	for _, s := range g.sceneObjs {
		if strings.ToLower(s.ref.Name) == name {
			return s.ref.GX, s.ref.GY, true
		}
	}
	return 0, 0, false
}

// startObjectAction resolves and begins an object's action; returns false if
// there is no action script (caller falls back to examine).
func (g *Game) startObjectAction(objName string) bool {
	ap := g.resolveAction(objName)
	if ap == nil {
		return false
	}
	g.pendingAct = ap
	// Walk to the action's Aproach cell first, playing the authored cycle chain
	// — the hero walks there, he does not slide.
	g.path = nil
	if p := g.grid.Path(g.cell, ap.target); len(p) > 1 {
		if evs, ok := g.startWalk(&g.roby, g.cell, p[1:]); ok {
			g.path = p[1:]
			g.applyWalkEvents(evs)
		}
	}
	return true
}

// updateAction advances an in-progress action: start it once the walk ends,
// then play frames, apply their events, and clear it when finished.
func (g *Game) updateAction(dt float64) {
	// The action itself starts once the walk to its Aproach cell has finished.
	if g.pendingAct != nil && !g.roby.walking() {
		g.act = g.pendingAct
		g.pendingAct = nil
	}
	if g.act == nil {
		return
	}
	g.act.started = true
	g.applyEvents(g.act.player.Update(dt))
	if g.act.player.Done() {
		g.act = nil
	}
}

// hideObject removes an object's sprite and hotspot (picked up / consumed).
func (g *Game) hideObject(name string) {
	name = strings.ToLower(name)
	for _, s := range g.sceneObjs {
		if strings.ToLower(s.ref.Name) == name {
			s.visible = false
			s.removed = true
			s.player = nil
		}
	}
	g.buildHotspots()
}

// gosceneFromEvent queues a scene transition from a GoScene event's args:
// scene,char,entry,gx,gy (5-arg) or scene,c1,e1,c2,e2,gx,gy (7-arg; both
// characters travel). The last two ints are the spawn cell; args[2] is the
// hero's arrival script.
func (g *Game) gosceneFromEvent(args []string) {
	if len(args) < 3 {
		return
	}
	gx, _ := strconv.Atoi(args[len(args)-2])
	gy, _ := strconv.Atoi(args[len(args)-1])
	ex := &types.Exit{Scene: strings.ToUpper(args[0]), GX: gx, GY: gy, OK: true}
	// scene,char,entry,gx,gy or scene,c1,e1,c2,e2,gx,gy; the pair whose char
	// is Frid becomes his silent arrival, the other one is the hero's.
	assign := func(char, entry string) {
		if strings.EqualFold(char, "Frid") {
			ex.EntryFrid = entry
		} else {
			ex.Entry = entry
		}
	}
	if len(args) >= 5 {
		assign(args[1], args[2])
	}
	if len(args) >= 7 {
		assign(args[3], args[4])
	}
	g.pending = ex
}

// startEntry plays a scene's arrival script (Roin2, robawake, Int1...) as the
// current character action, without a walk phase.
func (g *Game) startEntry(name string) {
	raw, err := g.sceneC.ExtractName(strings.ToUpper(name) + ".FS")
	if err != nil {
		return
	}
	fs := g.parser.ParseFrameScript(string(raw))
	if fs.MovieName == "" {
		return
	}
	fs.Looping = false
	g.act = &actionPlay{
		fs:      fs,
		frames:  adapters.LoadDecal(g.res, fs.MovieName),
		shift:   g.res.MovieShift(fs.MovieName),
		player:  use_cases.NewPlayer(fs),
		started: true,
	}
}

// drawAction draws the current action-movie frame (decal) if one is playing.
func (g *Game) drawAction(screen *ebiten.Image) bool {
	if g.act == nil || !g.act.started || len(g.act.frames) == 0 {
		return false
	}
	i := g.act.player.FrameIndex()
	if i < 0 || i >= len(g.act.frames) || g.act.frames[i].Img == nil {
		return true // playing but this frame is empty
	}
	// An action movie is the hero's own animation: canvas origin at the hero's
	// cell anchor minus the movie Shift, with each frame's crop offset added.
	f := g.act.frames[i]
	ox := g.pos[0] - float64(g.act.shift[0]) - float64(g.camX)
	oy := g.pos[1] - float64(g.act.shift[1])
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(ox+float64(f.X), oy+float64(f.Y))
	screen.DrawImage(f.Img, op)
	return true
}

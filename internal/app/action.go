package app

import (
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// aproachWait names the walker a paused action movie is waiting for.
type aproachWait int

const (
	waitNone aproachWait = iota
	waitRoby
	waitFrid
)

// actionPlay is a character action triggered by clicking an object. The movie
// starts at once; an Aproach among a frame's events pauses playback until the
// addressed character has walked over, so a script routes its characters
// mid-film as often as it likes (SHIP2's ROHANOUT walks Friday on frame 0 and
// the hero on frame 2). A script with no Aproach plays where the character
// stands: the engine has no implicit approach, and 1716 action scripts rely on
// that — every Cannotdo/Fool/Idiot/Whynot refusal is authored without one.
type actionPlay struct {
	fs      *types.FrameScript
	frames  []adapters.DecalFrame
	shift   [2]int // the movie's canvas hotspot (.SCR origin)
	player  *use_cases.Player
	frid    bool // the action belongs to Friday, so she anchors the movie
	started bool // the movie has begun playing (it may be paused by wait)
	// clicked is the clicked object's cell — the fallback for an Aproach that
	// names an object the scene does not place. nil for entry scripts.
	clicked *[2]int
	wait    aproachWait     // the walker playback is paused for, if any
	queue   []types.Command // the paused frame's commands still to enact
}

// scriptTok is the token a script name spends on one item or object: the first
// three letters, upper-cased. The truncation is the engine's, and it is lossy on
// purpose — "condom" and "confr" both become CON, so Friday's own items reach
// the same scripts.
func scriptTok(s string) string {
	s = strings.ToUpper(s)
	if len(s) > 3 {
		s = s[:3]
	}
	return s
}

// actionNames lists the script names a click may run, best first: <RO|FR> + the
// held item's token + the object's (ROHANGOL is Roby, bare hand, the left exit;
// ROAXEWOO is Roby chopping wood with the axe), then the bare-handed default so
// a tool the object does not answer to still gets the plain reaction.
func actionNames(activeChar, active, objName string) []string {
	char := "RO"
	if strings.EqualFold(activeChar, "Frid") {
		char = "FR"
	}
	obj := scriptTok(objName)
	item := scriptTok(active)
	names := []string{char + item + obj}
	if item != "HAN" {
		names = append(names, char+"HAN"+obj) // the empty-handed default
	}
	return names
}

// actionScript picks the script a click runs from actionNames' candidates. A
// script may redirect its own successor through a character variable named after
// itself — ROHANGOL ends with SetCharVar rohangol,"r1hangol", which is how the
// hero's remark changes each time he leaves — so that redirection wins when set.
func (g *Game) actionScript(objName string) (string, []byte, bool) {
	for _, base := range actionNames(g.gs.ActiveChar, g.gs.Active, objName) {
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

// movieShift is the point of the canvas the engine lands on the cell anchor. It
// comes from the movie's .SCR origin, but a FonScript may override it and the
// intro bridges rely on that: INT1.FS authors a Shift equal to its scene's
// anchor(0,0) so the full-window cutscene lands at the origin, while Int1.mv's
// own origin would push it 260px off.
func (g *Game) movieShift(fs *types.FrameScript) [2]int {
	if fs.Shift != ([2]int{}) {
		return fs.Shift
	}
	return g.res.MovieShift(fs.MovieName)
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
	// Click actions always play once.
	ocx, ocy, ok := g.objCell(objName)
	if !ok {
		return nil
	}
	return &actionPlay{
		fs:      fs,
		frames:  adapters.LoadDecal(g.res, fs.MovieName),
		shift:   g.movieShift(fs),
		player:  use_cases.NewPlayer(fs, false),
		frid:    strings.EqualFold(g.gs.ActiveChar, "Frid"),
		clicked: &[2]int{ocx, ocy},
	}
}

// aproachGoal resolves an Aproach event's target cell. Forms: (char, obj, dx,
// dy) -> that object's cell + offset; (char, gx, gy) -> absolute. The object
// named in the event is not always the one clicked -- SCENA4's ROHATGOL sends
// the hero to gorght because the hat glide starts at the far edge -- so cellOf
// resolves the name and the clicked object's own cell is only the fallback.
func aproachGoal(
	args []string, clicked *[2]int, cellOf func(string) (int, int, bool),
) (int, int, bool) {
	switch len(args) {
	case 4:
		dx, _ := strconv.Atoi(args[2])
		dy, _ := strconv.Atoi(args[3])
		if cx, cy, ok := cellOf(args[1]); ok {
			return cx + dx, cy + dy, true
		}
		if clicked != nil {
			return clicked[0] + dx, clicked[1] + dy, true
		}
		return 0, 0, false
	case 3:
		gx, _ := strconv.Atoi(args[1])
		gy, _ := strconv.Atoi(args[2])
		return gx, gy, true
	}
	return 0, 0, false
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
// there is no action script (caller falls back to examine). The script starts
// this very tick — updateAction fires frame 0, and its Aproach (if any) walks
// the character over before the movie advances.
func (g *Game) startObjectAction(objName string) bool {
	ap := g.resolveAction(objName)
	if ap == nil {
		return false
	}
	// A new action is a new intent: a click-walk still in progress stops on
	// the cell it reached, and the script routes the hero from there.
	g.roby.cur = nil
	g.path = nil
	g.act = ap
	return true
}

// aproachWalking reports whether the walker the movie waits for is still going.
func (g *Game) aproachWalking(w aproachWait) bool {
	switch w {
	case waitRoby:
		return g.roby.walking()
	case waitFrid:
		return len(g.fridPath) > 0
	}
	return false
}

// updateAction advances an in-progress action: play frames, apply their
// events — pausing at every Aproach until the walk it started has finished —
// and clear the action when the script is done.
func (g *Game) updateAction(dt float64) {
	ap := g.act
	if ap == nil {
		return
	}
	ap.started = true
	if ap.wait != waitNone {
		if g.aproachWalking(ap.wait) {
			return // the movie holds while the character walks
		}
		ap.wait = waitNone
		g.playActionQueue(ap, false) // the rest of the paused frame
		if ap.wait != waitNone {
			return // the same frame routed another character
		}
	}
	g.enqueueAction(ap, ap.player.Update(dt), false)
	if g.act == ap && ap.wait == waitNone && ap.player.Done() {
		g.act = nil
	}
}

// enqueueAction runs a batch of movie-frame commands through the quest
// interpreter and enacts the survivors in order, honouring Aproach pauses.
// A StartGame in the batch suspends the script the usual way (see mgResume).
func (g *Game) enqueueAction(ap *actionPlay, cmds []types.Command, skip bool) {
	if len(cmds) == 0 {
		return
	}
	out, rest := g.interp.Exec(cmds, g.gs)
	g.mgResume = rest
	ap.queue = append(ap.queue, out...)
	g.playActionQueue(ap, skip)
}

// playActionQueue enacts the interpreted commands the movie still owes. An
// Aproach starts the addressed character's walk and pauses the queue (and the
// player) until he arrives; the engine runs a frame's commands in order, so a
// SetVert ahead of the Aproach has already reshaped the walk grid by the time
// the route is computed — SCENA0's pool sits on a cell its own script opens.
// With skip the walk resolves instantly and presentation commands are muted
// (see skipCutscene).
func (g *Game) playActionQueue(ap *actionPlay, skip bool) {
	for len(ap.queue) > 0 {
		c := ap.queue[0]
		ap.queue = ap.queue[1:]
		kw := strings.ToLower(c.Kw)
		if kw == "aproach" || kw == "approach" {
			g.trace(c)
			if w := g.startAproach(c.Args, ap.clicked, skip); w != waitNone {
				ap.wait = w
				return
			}
			continue
		}
		if skip && presentational(kw) {
			continue
		}
		g.applyEffect(c)
	}
}

// startAproach begins the walk an Aproach playback event asks for and reports
// the walker the movie has to wait for (waitNone when there is nothing to wait
// on). With skip the walk resolves instantly: the character lands where the
// finished walk would have left him.
func (g *Game) startAproach(
	args []string,
	clicked *[2]int,
	skip bool,
) aproachWait {
	gx, gy, ok := aproachGoal(args, clicked, g.objCell)
	if !ok {
		return waitNone
	}
	if strings.EqualFold(args[0], "Frid") {
		if skip {
			// Mirror fridWalkTo's fallbacks, minus the walk: she always lands.
			if tx, ty, free := g.grid.NearestFree(gx, gy); free {
				gx, gy = tx, ty
			}
			g.placeFrid([2]int{gx, gy})
			return waitNone
		}
		g.fridWalkTo(gx, gy)
		if len(g.fridPath) > 0 {
			return waitFrid
		}
		return waitNone
	}
	if !strings.EqualFold(args[0], "Roby") {
		return waitNone
	}
	tx, ty, free := g.grid.NearestFree(gx, gy)
	if !free {
		return waitNone
	}
	p := g.grid.Path(g.cell, [2]int{tx, ty})
	if len(p) < 2 {
		return waitNone // already there, or unreachable: play in place
	}
	if skip {
		g.placeRoby(p[len(p)-1])
		return waitNone
	}
	if evs, walked := g.startWalk(&g.roby, g.cell, p[1:]); walked {
		g.path = p[1:]
		g.applyWalkEvents(evs)
		return waitRoby
	}
	// No cycle art (headless runs): land him on the goal rather than play the
	// movie astray.
	g.placeRoby([2]int{tx, ty})
	return waitNone
}

// placeRoby lands the hero on a cell at once, cutting any walk short.
func (g *Game) placeRoby(cell [2]int) {
	g.cell = cell
	px, py := g.grid.ToScreen(cell[0], cell[1])
	g.pos = [2]float64{float64(px), float64(py)}
	g.path = nil
	g.roby.cur = nil
}

// placeFrid lands Friday on a cell at once, cutting any walk short.
func (g *Game) placeFrid(cell [2]int) {
	g.fridCell = cell
	g.fridWalk.cur = nil
	g.fridSync()
}

// cutAproachShort finishes the walk a paused movie is waiting for: the
// character lands on the cell the walk was heading for (skipping a cutscene
// must leave the same state playing it out would have).
func (g *Game) cutAproachShort(w aproachWait) {
	switch w {
	case waitRoby:
		if n := len(g.path); n > 0 {
			g.placeRoby(g.path[n-1])
		} else {
			g.roby.cur = nil
		}
	case waitFrid:
		if n := len(g.fridPath); n > 0 {
			g.placeFrid(g.fridPath[n-1])
		} else {
			g.fridWalk.cur = nil
			g.fridPath = nil
		}
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
	g.act = &actionPlay{
		fs:      fs,
		frames:  adapters.LoadDecal(g.res, fs.MovieName),
		shift:   g.movieShift(fs),
		player:  use_cases.NewPlayer(fs, false),
		started: true,
	}
	// Fire frame 0 now rather than on the next updateAction: input runs earlier
	// in the tick, so a cutscene's opening "Interrupt ON" would otherwise land
	// one tick late and swallow the player's first click.
	g.enqueueAction(g.act, g.act.player.Update(0), false)
}

// drawAction draws the current action-movie frame (decal) if one is playing.
// A movie paused for an Aproach steps aside: the characters draw themselves
// (walk cycles, idle) until the walk is over.
func (g *Game) drawAction(screen *ebiten.Image) bool {
	if g.act == nil || !g.act.started || g.act.wait != waitNone ||
		len(g.act.frames) == 0 {
		return false
	}
	i := g.act.player.FrameIndex()
	if i < 0 || i >= len(g.act.frames) || g.act.frames[i].Img == nil {
		return true // playing but this frame is empty
	}
	// An action movie is the acting character's own animation: canvas origin at
	// his cell anchor minus the movie Shift, with each frame's crop offset added.
	f := g.act.frames[i]
	px, py := g.pos[0], g.pos[1]
	if g.act.frid {
		px, py = g.fridPos[0], g.fridPos[1]
	}
	ox := px - float64(g.act.shift[0]) - float64(g.camX)
	oy := py - float64(g.act.shift[1])
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(ox+float64(f.X), oy+float64(f.Y))
	screen.DrawImage(f.Img, op)
	return true
}

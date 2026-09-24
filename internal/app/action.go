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
// starts at once; an Aproach among a frame's events sends the addressed
// character walking, and when that is the movie's owner playback pauses until
// he has walked over, so a script routes its characters mid-film as often as it
// likes (SHIP2's ROHANOUT walks Friday on frame 0 and the hero on frame 2). The
// other character walks alongside: ROBY.EXE ticks a movie only through its
// owner, so it never waits for anyone else. A script with no Aproach plays where the character
// stands: the engine has no implicit approach, and 1716 action scripts rely on
// that — every Cannotdo/Fool/Idiot/Whynot refusal is authored without one.
type actionPlay struct {
	name    string // the script it plays, as resolved (for the debug HUD)
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

// charTok is the two-letter prefix an action script spends on the acting
// character: RO for the hero, FR for Friday.
func charTok(activeChar string) string {
	if strings.EqualFold(activeChar, "Frid") {
		return "FR"
	}
	return "RO"
}

// actionName is the one script name a click on an object runs: <RO|FR> + the
// held item's token + the object's (ROHANGOL is Roby, bare hand, the left
// exit; ROAXEWOO is Roby chopping wood with the axe). The engine composes
// exactly this name (0x41e320) and gives up silently when the scene ships no
// such script — there is no bare-handed fallback, and the manual's promise of
// a reaction to "any" use of an item is kept by data: the scenes carry a
// script per item even for the plain refusals (641 of them share Cannotdo.mv,
// the arms-spread "can't do that" shrug).
func actionName(activeChar, active, objName string) string {
	return charTok(activeChar) + scriptTok(active) + scriptTok(objName)
}

// selfActionName is the five-letter script of using the held item on the
// acting character himself — ROHAT puts the hat on (engine 0x40f414).
func selfActionName(activeChar, active string) string {
	return charTok(activeChar) + scriptTok(active)
}

// actionScript resolves the script a click on an object runs, following the
// CharVar redirection (see lookupScript).
func (g *Game) actionScript(objName string) (string, []byte, bool) {
	return g.lookupScript(
		actionName(g.gs.ActiveChar, g.gs.Active, objName),
	)
}

// lookupScript fetches an action script by its composed name. A script may
// redirect its own successor through a character variable named after itself —
// ROHANGOL ends with SetCharVar rohangol,"r1hangol", which is how the hero's
// remark changes each time he leaves — so that redirection wins when set.
func (g *Game) lookupScript(base string) (string, []byte, bool) {
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
	ocx, ocy, ok := g.objCell(objName)
	if !ok {
		return nil
	}
	return g.resolveNamed(
		actionName(g.gs.ActiveChar, g.gs.Active, objName),
		[2]int{ocx, ocy},
	)
}

// resolveNamed builds the one-shot action of a composed script name, anchored
// to the given cell (the clicked object's, or the acting character's own for a
// self action).
func (g *Game) resolveNamed(name string, cell [2]int) *actionPlay {
	if g.sceneC == nil {
		return nil
	}
	script, raw, ok := g.lookupScript(name)
	if !ok {
		return nil
	}
	fs := g.parseFS(raw)
	if fs.MovieName == "" {
		return nil
	}
	// Click actions always play once.
	return &actionPlay{
		name:    script,
		fs:      fs,
		frames:  adapters.LoadDecal(g.res, fs.MovieName, g.pal),
		shift:   g.movieShift(fs),
		player:  use_cases.NewPlayer(fs, false),
		frid:    strings.EqualFold(g.gs.ActiveChar, "Frid"),
		clicked: &cell,
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
// there is no action script (the engine then eats the click). The script starts
// this very tick — updateAction fires frame 0, and its Aproach (if any) walks
// the character over before the movie advances.
func (g *Game) startObjectAction(objName string) bool {
	return g.beginAction(g.resolveAction(objName))
}

// startSelfAction handles a click on the acting character's own cell: the held
// item is aimed at himself (ROHAT is how the hat goes on — engine 0x40f110,
// which compares grid cells, not sprite pixels). It reports whether the click
// is spoken for: with the bare hand, or with an item the scene ships no script
// for, the click is eaten without a walk — only a click elsewhere walks.
func (g *Game) startSelfAction(cx, cy int) bool {
	cell, standing := g.actingCell()
	if [2]int{cx, cy} != cell || !standing {
		return false
	}
	if scriptTok(g.gs.Active) == "HAN" || g.gs.Active == "" {
		return true // the bare hand on himself: eaten, nothing to do
	}
	g.beginAction(g.resolveNamed(
		selfActionName(g.gs.ActiveChar, g.gs.Active), cell,
	))
	return true
}

// actingCell is the cell of the character the player controls, and whether he
// is standing on it rather than walking through.
func (g *Game) actingCell() ([2]int, bool) {
	if strings.EqualFold(g.gs.ActiveChar, "Frid") {
		return g.fridCell, len(g.fridPath) == 0
	}
	return g.cell, !g.roby.walking()
}

// beginAction installs a resolved action as the playing one and, like the
// engine at every movie start (0x40e936), turns the mouse off for its run —
// updateAction turns it back on when the movie is over, which half the scripts
// setting SetMouse OFF silently rely on.
func (g *Game) beginAction(ap *actionPlay) bool {
	if ap == nil {
		return false
	}
	// A click-walk still in progress goes on: ROBY.EXE loads the script and
	// runs its frame 0 at once (0x4101d0), so its Aproach reroutes the walk
	// without cutting the step, and the movie holds until the hero stands.
	g.act = ap
	g.gs.UI["mouse"] = false
	return true
}

// owner is the walker whose walk holds the movie: Friday for her own actions.
func (ap *actionPlay) owner() aproachWait {
	if ap.frid {
		return waitFrid
	}
	return waitRoby
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
	// The engine ticks a script through its owner only while he stands
	// (+0x590), so a movie whose owner is still walking — a click-walk its
	// frame 0 did not reroute — waits for him like after its own Aproach.
	if g.act == ap && ap.wait == waitNone && g.aproachWalking(ap.owner()) {
		ap.wait = ap.owner()
	}
	if g.act == ap && ap.wait == waitNone && ap.player.Done() {
		g.act = nil
		// The movie is over: the engine hands the mouse back by itself
		// (0x40ebea) — 51 scripts end on SetMouse OFF and lean on this.
		g.gs.UI["mouse"] = true
	}
}

// enqueueAction runs a batch of movie-frame commands through the quest
// interpreter and enacts the survivors in order, honouring Aproach pauses.
// A StartGame in the batch suspends the script the usual way (see mgResume).
func (g *Game) enqueueAction(ap *actionPlay, cmds []types.Command, skip bool) {
	if len(cmds) == 0 {
		return
	}
	out, rest := g.exec(cmds)
	g.mgResume = rest
	ap.queue = append(ap.queue, out...)
	g.playActionQueue(ap, skip)
}

// playActionQueue enacts the interpreted commands the movie still owes. An
// Aproach starts the addressed character's walk; the owner's own walk pauses
// the queue (and the player) until he arrives; the engine runs a frame's commands in order, so a
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
			w := g.startAproach(c.Args, ap.clicked, ap.frid, skip)
			if w != waitNone {
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
// the walker the movie has to wait for: only its owner (Friday when frid),
// waitNone for the other character or when there is nothing to wait on. With
// skip the walk resolves instantly: the character lands where the finished
// walk would have left him.
func (g *Game) startAproach(
	args []string,
	clicked *[2]int,
	frid, skip bool,
) aproachWait {
	gx, gy, ok := aproachGoal(args, clicked, g.objCell)
	if !ok {
		return waitNone
	}
	if strings.EqualFold(args[0], "Frid") {
		if skip || g.fridHidden {
			g.landFrid(gx, gy)
			return waitNone
		}
		g.fridWalkTo(gx, gy)
		if frid && len(g.fridPath) > 0 {
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
	if g.roby.walking() {
		g.rerouteRoby([2]int{tx, ty}) // under way: never cut the step
		switch {
		case skip:
			g.cutAproachShort(waitRoby) // land where that walk ends
			return waitNone
		case frid:
			return waitNone
		}
		return waitRoby
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
		if frid {
			return waitNone // Friday's movie goes on while he walks
		}
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

// landFrid puts Friday where a walk to a cell would end, without the walk:
// fridWalkTo's fallbacks minus the route. A hidden Friday is always landed so.
// ROBY.EXE walks her anyway, but unseen, unheard and waited for by no one, so
// the cell she ends on is all of that walk anybody can tell — and before she
// joins the party every island exit sends her across the whole scene.
func (g *Game) landFrid(gx, gy int) {
	if tx, ty, free := g.grid.NearestFree(gx, gy); free {
		gx, gy = tx, ty
	}
	g.placeFrid([2]int{gx, gy})
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

// hideObject removes an object's sprite, hotspot and walk blocking (picked up /
// consumed / chased away): the caught crab and the fed crocodile have to open
// the cells they were lying on, or the hero stays walled off the ford.
func (g *Game) hideObject(name string) {
	name = strings.ToLower(name)
	for _, s := range g.sceneObjs {
		if strings.ToLower(s.ref.Name) == name {
			if !s.removed {
				g.setObjectBlocking(s, true)
			}
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
	fs := g.parseFS(raw)
	if fs.MovieName == "" {
		return
	}
	g.act = &actionPlay{
		name:    name,
		fs:      fs,
		frames:  adapters.LoadDecal(g.res, fs.MovieName, g.pal),
		shift:   g.movieShift(fs),
		player:  use_cases.NewPlayer(fs, false),
		started: true,
	}
	g.gs.UI["mouse"] = false // as at every movie start; see beginAction
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

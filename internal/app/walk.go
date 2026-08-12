package app

import (
	"strings"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/types"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// A walk cycle is a movie plus the frame script that times it: RG_XY.FS in
// ROBY.DAN (FG_XY.FS in FRID.DAN) carries each frame's Delay and its events —
// the step sounds and, on frame 0, the grid shift the engine applies.
type walkCycle struct {
	anim   *adapters.Animation
	frames []*types.Frame
}

// walker plays a route as the character's authored cycle chain: accelerate,
// one cycle per step (carrying the turn to the next direction), brake.
//
// The engine bakes the sub-cell motion of a step into the movie canvases: a
// cycle's frames march the figure across the cell, and frame 0 fires
// "Shift char,X|Y,±1" to move the cell itself. So a walking character is drawn
// at anchor(cell) - Shift + frameBBox and nothing interpolates its position —
// the animation *is* the movement. Each cycle plays exactly once, then the
// chain advances.
type walker struct {
	prefix string // "RG" (Roby) or "FG" (Friday)
	chr    string // character container: "ROBY" / "FRID"
	cycles []string
	idx    int
	cur    *walkCycle
	frame  int
	acc    float64
	done   bool // the current cycle has played out
}

// cycleKey is the resource base name of a cycle, e.g. "RG_56".
func (w *walker) cycleKey(i int) string {
	if i < 0 || i >= len(w.cycles) {
		return ""
	}
	return w.prefix + "_" + w.cycles[i]
}

// loadCycle fetches (and caches) a cycle's movie frames and its frame script.
func (g *Game) loadCycle(key, chr string) *walkCycle {
	if c, ok := g.cycleCache[key]; ok {
		return c
	}
	c := &walkCycle{anim: adapters.LoadAnimation(g.res, key+".mv")}
	if cc := g.res.SceneContainer(chr); cc != nil {
		if raw, err := cc.ExtractName(key + ".FS"); err == nil {
			c.frames = g.parser.ParseFrameScript(string(raw)).Frames
		}
	}
	if !c.anim.OK() {
		c = nil
	}
	g.cycleCache[key] = c
	return c
}

// frameEvents returns the events of the walker's current frame.
func (w *walker) frameEvents() []types.Command {
	if w.cur == nil || w.frame >= len(w.cur.frames) {
		return nil
	}
	return w.cur.frames[w.frame].Events
}

// enter loads cycle idx and returns its frame-0 events — that is where the
// authored cell shift, the step sound and the Z choreography live.
func (g *Game) enter(w *walker) []types.Command {
	w.frame, w.acc, w.done = 0, 0, false
	w.cur = g.loadCycle(w.cycleKey(w.idx), w.chr)
	if w.cur == nil {
		w.done = true
		return nil
	}
	return w.frameEvents()
}

// startWalk begins a route; returns the frame-0 events of the first cycle and
// whether there is anything to walk.
func (g *Game) startWalk(
	w *walker, from [2]int, route [][2]int,
) ([]types.Command, bool) {
	arrows := w.prefix == "FG"
	w.cycles = use_cases.WalkCycles(from, route, arrows)
	w.idx = 0
	if len(w.cycles) == 0 {
		w.cur, w.done = nil, true
		return nil, false
	}
	evs := g.enter(w)
	return evs, w.cur != nil
}

// nextCycle advances the chain; returns the new cycle's frame-0 events. When
// the chain is spent the walker reports walking() == false.
func (g *Game) nextCycle(w *walker) []types.Command {
	w.idx++
	if w.idx >= len(w.cycles) {
		w.cur, w.done = nil, true
		return nil
	}
	return g.enter(w)
}

// walking reports whether a cycle is still playing.
func (w *walker) walking() bool { return w.cur != nil && !w.done }

// frameDelay is the current frame's duration in seconds (negative delays mark
// ambient frames in the scripts; their magnitude is the duration).
func (w *walker) frameDelay() float64 {
	const fallback = 0.09
	if w.cur == nil || len(w.cur.frames) == 0 {
		return fallback
	}
	f := w.cur.frames[minInt(w.frame, len(w.cur.frames)-1)]
	d := float64(f.Delay) / 1000
	if d < 0 {
		d = -d
	}
	if d <= 0 {
		return fallback
	}
	return d
}

// advance ticks the cycle once through (a cycle never loops: the chain moves on)
// and returns the events of every frame it entered, so the caller can apply the
// authored cell shifts and step sounds.
func (w *walker) advance(dt float64) []types.Command {
	if !w.walking() || !w.cur.anim.OK() {
		return nil
	}
	var fired []types.Command
	w.acc += dt
	for w.acc >= w.frameDelay() {
		w.acc -= w.frameDelay()
		if w.frame+1 >= len(w.cur.anim.Frames) {
			w.done = true // played out; the caller picks the next cycle
			break
		}
		w.frame++
		fired = append(fired, w.frameEvents()...)
	}
	return fired
}

// anim is the animation to draw, or nil when the walker is idle.
func (w *walker) anim() *adapters.Animation {
	if w.cur == nil {
		return nil
	}
	return w.cur.anim
}

// updateWalk drives the hero's cycle chain: advance the animation, apply the
// events it fires (cell shifts, step sounds, Z), and move on to the next cycle
// when one plays out.
func (g *Game) updateWalk(dt float64) {
	if !g.roby.walking() {
		return
	}
	g.applyWalkEvents(g.roby.advance(dt))
	for g.roby.done {
		evs := g.nextCycle(&g.roby)
		if !g.roby.walking() {
			g.path = nil // the route is finished
			break
		}
		g.applyWalkEvents(evs)
	}
}

// updateFridWalk walks Friday along his own chain, four directions only.
func (g *Game) updateFridWalk(dt float64) {
	if !g.fridWalk.walking() {
		return
	}
	g.applyWalkEvents(g.fridWalk.advance(dt))
	for g.fridWalk.done {
		evs := g.nextCycle(&g.fridWalk)
		if !g.fridWalk.walking() {
			g.fridPath = nil
			break
		}
		g.applyWalkEvents(evs)
	}
}

// fridWalkTo sends Friday walking to a cell (scripts move him, he has no
// free will of his own); it falls back to placing him when there is no path.
func (g *Game) fridWalkTo(gx, gy int) {
	tx, ty, ok := g.grid.NearestFree(gx, gy)
	if !ok {
		g.fridCell = [2]int{gx, gy}
		g.fridSync()
		return
	}
	p := g.grid.Path(g.fridCell, [2]int{tx, ty})
	if len(p) < 2 {
		g.fridCell = [2]int{tx, ty}
		g.fridSync()
		return
	}
	g.fridPath = p[1:]
	evs, ok := g.startWalk(&g.fridWalk, g.fridCell, g.fridPath)
	if !ok {
		g.fridPath = nil
		g.fridCell = [2]int{tx, ty}
		g.fridSync()
		return
	}
	g.applyWalkEvents(evs)
}

// fridSync snaps Friday's screen position to his cell.
func (g *Game) fridSync() {
	x, y := g.grid.ToScreen(g.fridCell[0], g.fridCell[1])
	g.fridPos = [2]float64{float64(x), float64(y)}
	g.fridPath = nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// applyWalkEvents applies the events a walk cycle fired. Only the relative cell
// step is special: "Shift char,X|Y,±n" is what actually moves a walking
// character, and the interpreter has no case for it. Everything else -- the
// footstep sound and the "Set char,Z,n" that chooses the draw-order slot -- goes
// through the ordinary dispatch, which is what gives Friday's events their turn
// through fridEffect.
func (g *Game) applyWalkEvents(evs []types.Command) {
	for _, ev := range evs {
		if strings.EqualFold(ev.Kw, "shift") && !g.fridEffect("shift", ev.Args) {
			g.shiftCharCell(ev.Args)
			continue
		}
		if strings.EqualFold(ev.Kw, "shift") {
			continue // fridEffect took it
		}
		g.applyEffect(ev)
	}
}

// shiftCharCell applies "Shift char,X|Y,±n": it moves the character's cell by a
// relative step and re-anchors its screen position, without touching Z (the
// cycles carry an explicit Set char,Z for that).
func (g *Game) shiftCharCell(args []string) {
	if len(args) < 3 {
		return
	}
	d := atoiArg(args[2])
	if d == 0 {
		return
	}
	frid := strings.EqualFold(args[0], "Frid")
	cell := &g.cell
	if frid {
		cell = &g.fridCell
	}
	switch strings.ToUpper(args[1]) {
	case "X":
		cell[0] += d
	case "Y":
		cell[1] += d
	default:
		return
	}
	x, y := g.grid.ToScreen(cell[0], cell[1])
	if frid {
		g.fridPos = [2]float64{float64(x), float64(y)}
		return
	}
	g.pos = [2]float64{float64(x), float64(y)}
}

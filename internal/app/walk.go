package app

import (
	"math"
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
type walker struct {
	prefix string // "RG" (Roby) or "FG" (Friday)
	chr    string // character container: "ROBY" / "FRID"
	cycles []string
	idx    int
	cur    *walkCycle
	frame  int
	acc    float64
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

// start begins a route; returns false when there is nothing to walk.
func (g *Game) startWalk(w *walker, from [2]int, route [][2]int) bool {
	arrows := w.prefix == "FG"
	w.cycles = use_cases.WalkCycles(from, route, arrows)
	w.idx, w.frame, w.acc = 0, 0, 0
	if len(w.cycles) == 0 {
		w.cur = nil
		return false
	}
	w.cur = g.loadCycle(w.cycleKey(0), w.chr)
	return true
}

// nextCycle moves to the next cycle of the chain (called on each cell arrival).
func (g *Game) nextCycle(w *walker) {
	w.idx++
	w.frame, w.acc = 0, 0
	w.cur = g.loadCycle(w.cycleKey(w.idx), w.chr)
}

// frameDelay is the current frame's duration in seconds (negative delays mark
// ambient frames in the scripts; their magnitude is the duration).
func (w *walker) frameDelay() float64 {
	const fallback = 0.09
	if w.cur == nil || len(w.cur.frames) == 0 {
		return fallback
	}
	f := w.cur.frames[w.frame%len(w.cur.frames)]
	d := float64(f.Delay) / 1000
	if d < 0 {
		d = -d
	}
	if d <= 0 {
		return fallback
	}
	return d
}

// advance ticks the cycle's animation and returns the events of every frame it
// entered, so the caller can play the authored step sounds.
func (w *walker) advance(dt float64) []types.Command {
	if w.cur == nil || !w.cur.anim.OK() {
		return nil
	}
	var fired []types.Command
	w.acc += dt
	for w.acc >= w.frameDelay() {
		w.acc -= w.frameDelay()
		w.frame = (w.frame + 1) % len(w.cur.anim.Frames)
		if w.cur.frames != nil && w.frame < len(w.cur.frames) {
			fired = append(fired, w.cur.frames[w.frame].Events...)
		}
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

// walkSounds plays the Sound events a cycle fired (step noises).
func (g *Game) walkSounds(evs []types.Command) {
	for _, ev := range evs {
		if strings.EqualFold(ev.Kw, "sound") {
			g.playSound(ev.Args)
		}
	}
}

// walkSpeed is how fast a character crosses the grid, in pixels per second.
const walkSpeed = 220

// stepToward slides pos towards the next cell of path; on arrival it commits the
// cell, drops it from the route, and reports true so the caller can advance the
// walk cycle.
func (g *Game) stepToward(
	pos *[2]float64, cell *[2]int, path *[][2]int, dt float64,
) bool {
	if len(*path) == 0 {
		return false
	}
	next := (*path)[0]
	tx, ty := g.grid.ToScreen(next[0], next[1])
	dx, dy := float64(tx)-pos[0], float64(ty)-pos[1]
	dist := math.Hypot(dx, dy)
	step := walkSpeed * dt
	if dist > step && dist > 0 {
		pos[0] += dx / dist * step
		pos[1] += dy / dist * step
		return false
	}
	*pos = [2]float64{float64(tx), float64(ty)}
	*cell = next
	*path = (*path)[1:]
	return true
}

// updateFridWalk walks Friday along his route, four directions only.
func (g *Game) updateFridWalk(dt float64) {
	if len(g.fridPath) == 0 {
		return
	}
	if g.stepToward(&g.fridPos, &g.fridCell, &g.fridPath, dt) {
		g.nextCycle(&g.fridWalk)
	}
	g.walkSounds(g.fridWalk.advance(dt))
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
	if !g.startWalk(&g.fridWalk, g.fridCell, g.fridPath) {
		g.fridPath = nil
		g.fridCell = [2]int{tx, ty}
		g.fridSync()
	}
}

// fridSync snaps Friday's screen position to his cell.
func (g *Game) fridSync() {
	x, y := g.grid.ToScreen(g.fridCell[0], g.fridCell[1])
	g.fridPos = [2]float64{float64(x), float64(y)}
	g.fridPath = nil
}

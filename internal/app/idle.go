package app

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/use_cases"
)

// ROBY.CHR lists three idle slots: [0] standing (head), [1] the acknowledge
// animation (the ok0… chain), [2] the long "bored" idle (the roby1… chain).
// SetRest char,state,anim swaps slot 1 or 2 for another script, which is how the
// scenes change what Robinson mutters while he waits.
const (
	restStand = 0
	restOK    = 1
	restBored = 2
)

// idleAfter is how long the hero stands still before playing his long idle.
const idleAfter = 9.0

// HEAD.MV is not an animation but a 3x3 table of standing poses: its nine
// frames all carry the same Delay and no events, and every frame draws the same
// body with only the head turned. Measured by the eyes in each frame, the
// columns look left / straight / right and the rows look down / straight / up,
// so the frame is chosen by where the cursor is relative to the hero's head —
// he follows the mouse instead of rolling his head on a timer.
const (
	headCols    = 3
	headDeadX   = 44 // cursor within this many px stays "straight ahead"
	headDeadY   = 40
	headEyeDX   = 30 // eyes relative to the cell anchor (Shift 116,85)
	headEyeDY   = 15
	headUpRow   = 2
	headMidRow  = 1
	headDownRow = 0
)

// headFrame picks the standing pose for a cursor offset from the hero's eyes.
func headFrame(dx, dy int) int {
	col := 1
	switch {
	case dx < -headDeadX:
		col = 0
	case dx > headDeadX:
		col = 2
	}
	row := headMidRow
	switch {
	case dy < -headDeadY:
		row = headUpRow
	case dy > headDeadY:
		row = headDownRow
	}
	return row*headCols + col
}

// lookAtCursor aims the hero's standing pose at the cursor. It only applies to
// the standing loop (slot 0); the ok/bored chains are real animations.
func (g *Game) lookAtCursor() {
	if !g.idle.OK() || len(g.idle.Frames) < headCols*headCols {
		return // not the nine-pose table: leave the frame alone
	}
	mx, my := ebiten.CursorPosition()
	ex := int(g.pos[0]) - g.camX + headEyeDX
	ey := int(g.pos[1]) + headEyeDY
	g.frameI = headFrame(mx-ex, my-ey)
}

// loadCharacter reads ROBY.CHR and takes its standing animation and idle slots.
func (g *Game) loadCharacter() {
	c := g.res.SceneContainer("ROBY")
	if c == nil {
		return
	}
	d, err := c.ExtractName("ROBY.CHR")
	if err != nil {
		return
	}
	ch := g.parser.ParseChar(string(d))
	g.restSlots = ch.Idle
	if s := ch.Idle[restStand]; s != "" {
		if a := adapters.LoadAnimation(g.res, s+".mv"); a.OK() {
			g.idle = a // HEAD.mv: the real standing loop
		}
	}
}

// setRest applies SetRest char,state,anim: it swaps one of the hero's idle
// slots. Friday has no chains of his own in the scripts.
func (g *Game) setRest(args []string) {
	if len(args) < 3 || !strings.EqualFold(args[0], "Roby") {
		return
	}
	state := atoiArg(args[1])
	if state < 0 || state >= len(g.restSlots) {
		return
	}
	g.restSlots[state] = args[2]
	g.idleT = 0
}

// updateIdle counts standing time and plays the long idle once it is due.
func (g *Game) updateIdle(dt float64) {
	g.updateIdlePlay(dt)
	if g.moving || g.act != nil || g.idleAct != nil {
		g.idleT = 0
		return
	}
	g.idleT += dt
	if g.idleT < idleAfter {
		return
	}
	g.idleT = 0
	g.playIdleSlot(restBored)
}

// idlePlay is a one-shot idle animation playing where the hero stands. It is
// drawn on the hero's cell anchor minus its own movie Shift, the same placement
// as the standing and walking loops, so it plays wherever he is.
type idlePlay struct {
	anim   *adapters.Animation
	player *use_cases.Player
}

// playIdleSlot starts one of the hero's idle-slot scripts; its frame events
// (Text, Sound, SetRest) run through the interpreter as usual.
func (g *Game) playIdleSlot(slot int) {
	name := g.restSlots[slot]
	if name == "" {
		return
	}
	c := g.res.SceneContainer("ROBY")
	if c == nil {
		return
	}
	raw, err := c.ExtractName(strings.ToUpper(name) + ".FS")
	if err != nil {
		return
	}
	fs := g.parser.ParseFrameScript(string(raw))
	if fs.MovieName == "" {
		return
	}
	a := adapters.LoadAnimation(g.res, fs.MovieName)
	if !a.OK() {
		return
	}
	g.idleAct = &idlePlay{anim: a, player: use_cases.NewPlayer(fs, false)}
}

// updateIdlePlay advances a running idle animation and applies its events.
func (g *Game) updateIdlePlay(dt float64) {
	if g.idleAct == nil {
		return
	}
	g.applyEvents(g.idleAct.player.Update(dt))
	if g.idleAct.player.Done() {
		g.idleAct = nil
	}
}

// skipCutscene fast-forwards the playing action to its end, applying every
// event it still owes so the quest state stays correct. The engine only allows
// this while Interrupt is ON.
func (g *Game) skipCutscene() bool {
	if g.act == nil || !g.gs.UI["interrupt"] {
		return false
	}
	// A movie paused for an Aproach walk first lands the walker on his goal,
	// then plays out the rest of the paused frame. In skip mode every further
	// Aproach resolves instantly too, so the player below never blocks.
	if g.act.wait != waitNone {
		g.cutAproachShort(g.act.wait)
		g.act.wait = waitNone
	}
	g.playActionQueue(g.act, true)
	// Step the player with generous slices until it reports done, applying the
	// state and world events it still owes but muting its sounds and subtitles:
	// the whole remainder fires within one tick, so playing them would stack
	// every line the cutscene had left (INT1 alone still owes rain on seven
	// channels, thunder, and Robinson's scream). Stop as soon as an event ends
	// the script, so skipping cannot run past a scene change or launch a
	// minigame twice.
	for i := 0; i < 10000 && !g.act.player.Done() &&
		g.pending == nil && g.mg == nil; i++ {
		g.enqueueAction(g.act, g.act.player.Update(1), true)
	}
	g.act = nil
	g.msg, g.msgT = "", 0 // the line on screen belonged to the skipped scene
	g.audio.StopEffects() // and so does whatever it had already started
	return true
}

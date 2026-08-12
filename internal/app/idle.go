package app

import (
	"strings"

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
	if g.moving || g.act != nil || g.pendingAct != nil || g.idleAct != nil {
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

// idlePlay is a one-shot idle animation playing where the hero stands. Unlike
// an object action (a decal authored for a fixed spot) it is drawn with the same
// feet anchor as the standing and walking loops, so it plays wherever he is.
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
	fs.Looping = false
	a := adapters.LoadAnimation(g.res, fs.MovieName)
	if !a.OK() {
		return
	}
	g.idleAct = &idlePlay{anim: a, player: use_cases.NewPlayer(fs)}
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
	// Step the player with generous slices until it reports done; the events
	// it fires are applied as usual.
	for i := 0; i < 10000 && !g.act.player.Done(); i++ {
		g.applyEvents(g.act.player.Update(1))
	}
	g.act = nil
	return true
}

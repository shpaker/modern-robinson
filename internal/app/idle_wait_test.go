package app

import (
	"strings"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
)

// idleTick is one engine tick at 60 TPS.
const idleTick = 1.0 / 60

// idleDueTick stands the hero still for up to limit ticks, game time running
// at the given scale, and returns the tick the long idle fired on (0: never).
// With no slot 2 loaded the chain itself cannot start, so the firing shows as
// the wait dropping back to zero.
func idleDueTick(g *Game, scale float64, limit int) int {
	for i := 1; i <= limit; i++ {
		g.updateIdle(idleTick*scale, idleTick)
		if g.idleT == 0 {
			return i
		}
	}
	return 0
}

// The engine starts the long idle 27 000 ms after the last chain, walk or
// script ended, on timeGetTime; the speed slider divides frame delays, not
// this deadline. So the wait is 1620 ticks whatever the slider says.
func TestLongIdleIsDueAfter27Seconds(t *testing.T) {
	for _, scale := range []float64{0.5, 1, 1.5} {
		g := &Game{}
		if got := idleDueTick(g, scale, 3000); got < 1620 || got > 1621 {
			t.Errorf("scale %v: long idle on tick %d, want 1620..1621",
				scale, got)
		}
	}
}

// A walk or a script rearms the deadline: the wait starts over from its end,
// not from the last time the hero stood.
func TestIdleWaitRestartsAfterWalksAndActions(t *testing.T) {
	cases := []struct {
		name string
		busy func(g *Game, on bool)
	}{
		{"walk", func(g *Game, on bool) { g.moving = on }},
		{"action", func(g *Game, on bool) {
			g.act = nil
			if on {
				g.act = &actionPlay{}
			}
		}},
	}
	for _, c := range cases {
		g := &Game{}
		if got := idleDueTick(g, 1, 1200); got != 0 {
			t.Fatalf("%s: long idle on tick %d, want none yet", c.name, got)
		}
		c.busy(g, true)
		g.updateIdle(idleTick, idleTick)
		if g.idleT != 0 {
			t.Errorf("%s: wait = %v while busy, want 0", c.name, g.idleT)
		}
		c.busy(g, false)
		if got := idleDueTick(g, 1, 1200); got != 0 {
			t.Errorf("%s: long idle %d ticks after the %s, want the wait "+
				"restarted", c.name, got, c.name)
		}
		if got := idleDueTick(g, 1, 480); got == 0 {
			t.Errorf("%s: no long idle 1680 ticks after the %s",
				c.name, c.name)
		}
	}
}

// HideChar holds the chain back but not the clock (0x40e018): a hidden hero
// never starts it, and once shown an overdue one starts it on the next tick.
func TestHiddenHeroHoldsTheLongIdle(t *testing.T) {
	g := &Game{charHidden: true}
	if got := idleDueTick(g, 1, 1800); got != 0 {
		t.Fatalf("hidden hero's long idle on tick %d, want none", got)
	}
	if g.idleT < 29.9 {
		t.Fatalf("wait = %v after 30 s hidden, want it still running",
			g.idleT)
	}
	g.charHidden = false
	g.updateIdle(idleTick, idleTick)
	if g.idleT != 0 {
		t.Errorf("wait = %v once shown, want the overdue idle started",
			g.idleT)
	}
}

// newIdleGame enters SCENA0 directly: no entry script, no fade, the hero
// standing with ROBY.CHR's slots loaded.
func newIdleGame(t *testing.T) *Game {
	t.Helper()
	t.Setenv("ROBINSON_SCENE", "SCENA0")
	res := repositories.NewResources(testutil.GameRoot(t))
	g := NewGameWith(res, DefaultConfig())
	g.audio = &fakeAudio{}
	if g.act != nil || g.moving {
		t.Fatal("direct entry must leave the hero standing idle")
	}
	return g
}

// Slot 2 plays roby1, whose last frame swaps the slot to Roby1a
// (SetRest Roby,2,Roby1a); the chain's end rearms the deadline, so the next
// one follows 27 s later.
func TestLongIdlePlaysSlotTwoThenRearms(t *testing.T) {
	g := newIdleGame(t)
	if !strings.EqualFold(g.restSlots[restBored], "roby1") {
		t.Fatalf("slot 2 = %q, want roby1", g.restSlots[restBored])
	}
	stand := func(n int) {
		for i := 0; i < n; i++ {
			g.updateIdle(idleTick, idleTick)
		}
	}
	stand(1619)
	if g.idleAct != nil {
		t.Fatal("long idle started before 27 s")
	}
	stand(2)
	if g.idleAct == nil {
		t.Fatal("no long idle after 27 s")
	}
	for i := 0; i < 3600 && g.idleAct != nil; i++ {
		g.updateIdle(idleTick, idleTick)
	}
	if g.idleAct != nil {
		t.Fatal("roby1 still playing after a minute")
	}
	if !strings.EqualFold(g.restSlots[restBored], "Roby1a") {
		t.Fatalf("slot 2 = %q after roby1, want Roby1a",
			g.restSlots[restBored])
	}
	// The tick the chain ended on already counts towards the next wait.
	stand(1618)
	if g.idleAct != nil {
		t.Fatal("next long idle started before 27 s")
	}
	stand(3)
	if g.idleAct == nil {
		t.Fatal("no next long idle 27 s after roby1 ended")
	}
}

// The full Update keeps the wait on the unscaled tick at either end of the
// speed slider.
func TestLongIdleIgnoresTheSpeedSlider(t *testing.T) {
	for _, speed := range []float64{0, 1} {
		g := newIdleGame(t)
		g.speed = speed
		due := 0
		for i := 1; i <= 3000 && due == 0; i++ {
			_ = g.Update()
			if g.idleAct != nil {
				due = i
			}
		}
		if due < 1620 || due > 1622 {
			t.Errorf("speed %v: long idle on tick %d, want 1620..1622",
				speed, due)
		}
	}
}

// The slider still paces the chain itself: its frames run on Delays, which the
// engine divides by the speed setting (0x416699).
func TestSpeedSliderPacesTheIdleChain(t *testing.T) {
	ticks := map[float64]int{}
	for _, speed := range []float64{0, 1} {
		g := newIdleGame(t)
		g.speed = speed
		g.idleT = idleAfter // overdue: the chain starts on the first tick
		_ = g.Update()
		if g.idleAct == nil {
			t.Fatalf("speed %v: no long idle once overdue", speed)
		}
		n := 1
		for ; n < 10000 && g.idleAct != nil; n++ {
			_ = g.Update()
		}
		if g.idleAct != nil {
			t.Fatalf("speed %v: roby1 still playing after %d ticks", speed, n)
		}
		ticks[speed] = n
	}
	// 1.5 against 0.5 times the authored pace: a third as many ticks.
	if slow, fast := ticks[0], ticks[1]; fast*2 > slow {
		t.Errorf("roby1 took %d ticks at full speed and %d at the slowest, "+
			"want about a third", fast, slow)
	}
}

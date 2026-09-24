package app

import (
	"image"
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// cursorGame is a 640x400 scene in play with the hand selected, a left exit
// zone (Cursor 1) and an ordinary object (Cursor 0) side by side.
func cursorGame() *Game {
	g := &Game{gs: types.NewGameState(), mode: modePlay, w: 640, h: 400}
	g.gs.ActiveChar, g.gs.Active = "Roby", "hand"
	g.hotspots = []hotspot{
		{
			key:  "goleft",
			rect: image.Rect(0, 0, 50, 400),
			ob:   &types.SceneObject{Name: "gooak", Cursor: 1},
		},
		{
			key:  "palma",
			rect: image.Rect(300, 100, 340, 300),
			ob:   &types.SceneObject{Name: "palma"},
		},
	}
	return g
}

// Over the scene the engine shows an exit arrow only on a zone that asks for
// one; everywhere else, objects included, the cursor is the item in hand.
func TestPlayCursorIsTheItemInHand(t *testing.T) {
	g := cursorGame()
	cases := []struct {
		x, y int
		want string
	}{
		{10, 200, "ADV1"},     // the exit zone
		{320, 200, "HAND"},    // an ordinary object: no cursor of its own
		{500, 300, "HAND"},    // bare ground
		{320, 400, "HAND"},    // y == ScreenSize.y still belongs to the scene
		{20, 450, cursorHand}, // the bar
	}
	for _, c := range cases {
		if got := g.playCursor(c.x, c.y); got != c.want {
			t.Errorf("cursor at %d,%d = %s, want %s", c.x, c.y, got, c.want)
		}
	}
	g.gs.Active = "hat"
	if got := g.playCursor(320, 200); got != "HAT" {
		t.Errorf("with the hat in hand = %s, want HAT", got)
	}
	if got := g.playCursor(10, 200); got != "ADV1" {
		t.Errorf("an exit keeps its arrow whatever is in hand: %s", got)
	}
}

// Friday's turn shows her item over the scene, but the bar always shows the
// engine's own hand (AdvancedItems[0]).
func TestPlayCursorForFriday(t *testing.T) {
	g := cursorGame()
	g.gs.ActiveChar, g.gs.Active = "Frid", "handfr"
	if got := g.playCursor(500, 300); got != "HANDFR" {
		t.Errorf("Friday over the scene = %s, want HANDFR", got)
	}
	if got := g.playCursor(500, 450); got != cursorHand {
		t.Errorf("Friday over the bar = %s, want HAND", got)
	}
}

// SetMouse OFF, which every movie does, shows the waiting clock on the scene
// and on the bar alike; LockBar makes the bar keep whatever cursor it had.
func TestPlayCursorUnderSetMouseAndLockBar(t *testing.T) {
	g := cursorGame()
	g.gs.UI["mouse"] = false
	for _, p := range [][2]int{{10, 200}, {500, 300}, {500, 450}} {
		if got := g.playCursor(p[0], p[1]); got != cursorWait {
			t.Errorf("SetMouse OFF at %v = %s, want the clock", p, got)
		}
	}
	g.gs.UI["mouse"] = true
	g.gs.UI["barlock"] = true
	g.cursorName = "ADV1" // what the scene last showed
	if got := g.playCursor(500, 450); got != "ADV1" {
		t.Errorf("LockBar over the bar = %s, want the cursor kept", got)
	}
	g.gs.UI["mouse"] = false
	if got := g.playCursor(500, 450); got != "ADV1" {
		t.Errorf("LockBar is checked before SetMouse: got %s", got)
	}
}

// The intro bridges are 480 tall: the whole window is scene, and the bottom
// strip never turns into the bar's hand.
func TestPlayCursorOnAFullWindowScene(t *testing.T) {
	g := cursorGame()
	g.h = 480
	g.gs.Active = "hat"
	if got := g.playCursor(320, 450); got != "HAT" {
		t.Errorf("bottom of a 480-tall scene = %s, want the item", got)
	}
}

// Outside play the original leaves the choice to others: the intro hides the
// cursor, loading shows the clock, the menus and the minigames the system
// arrow, and ShowCursor off hides it in play.
func TestCursorForModes(t *testing.T) {
	g := cursorGame()
	cases := []struct {
		mode   int
		name   string
		system bool
	}{
		{modeLogo, "", false},
		{modeTitle, "", false},
		{modeLoading, cursorWait, false},
		{modeOptions, "", true},
		{modeSave, "", true},
		{modeLoad, "", true},
		{modePlay, "HAND", false},
	}
	for _, c := range cases {
		g.mode = c.mode
		name, system := g.cursorFor(500, 300)
		if name != c.name || system != c.system {
			t.Errorf("mode %d: cursor %q system %v, want %q %v",
				c.mode, name, system, c.name, c.system)
		}
	}
	g.mode = modePlay
	g.gs.UI["cursor"] = false
	if name, system := g.cursorFor(500, 300); name != "" || system {
		t.Errorf("ShowCursor off: %q %v, want nothing", name, system)
	}
}

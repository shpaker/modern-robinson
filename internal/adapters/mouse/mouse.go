// Package mouse is the pointer the minigames read: the real mouse, or one a
// driver outside the game holds for them while it plays a puzzle by picture
// (adapters/mcp). Everything runs on the game loop's goroutine.
package mouse

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// state is the pointer on one tick.
type state struct {
	x, y        int
	left, right bool
}

func (s state) down(b ebiten.MouseButton) bool {
	switch b {
	case ebiten.MouseButtonLeft:
		return s.left
	case ebiten.MouseButtonRight:
		return s.right
	}
	return false
}

// held is the driven pointer: its state this tick and the one before, which
// together make the press edges.
var (
	held      bool
	cur, prev state
)

// Hold drives the pointer for this tick: at x,y with the given buttons down.
// The driver calls it once per tick for as long as it holds the pointer; the
// real mouse is not read until Release.
func Hold(x, y int, left, right bool) {
	if held {
		prev = cur
	} else {
		prev = state{x: x, y: y} // taking over presses nothing by itself
	}
	cur, held = state{x, y, left, right}, true
}

// Release hands the pointer back to the real mouse.
func Release() { held = false }

// Held reports whether a driver holds the pointer.
func Held() bool { return held }

// Position is where the pointer is.
func Position() (int, int) {
	if held {
		return cur.x, cur.y
	}
	return ebiten.CursorPosition()
}

// JustPressed reports a button that went down this tick.
func JustPressed(b ebiten.MouseButton) bool {
	if held {
		return cur.down(b) && !prev.down(b)
	}
	return inpututil.IsMouseButtonJustPressed(b)
}

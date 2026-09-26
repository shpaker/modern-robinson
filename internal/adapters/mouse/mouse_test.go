package mouse

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// A held pointer presses on the tick its button goes down and on no other:
// the minigames act on that edge, so a button held across ticks must not
// click twice, and taking the pointer over must not click at all.
func TestHeldPointerPressesOnTheEdge(t *testing.T) {
	defer Release()
	steps := []struct {
		left, right bool
		wantL       bool
		wantR       bool
	}{
		{false, false, false, false}, // taken over: nothing pressed
		{true, false, true, false},   // down: the click
		{true, false, false, false},  // still down: no second click
		{false, false, false, false}, // up
		{false, true, false, true},   // right button
	}
	for i, s := range steps {
		Hold(100+i, 200, s.left, s.right)
		if got := JustPressed(ebiten.MouseButtonLeft); got != s.wantL {
			t.Errorf("tick %d: left pressed = %v, want %v", i, got, s.wantL)
		}
		if got := JustPressed(ebiten.MouseButtonRight); got != s.wantR {
			t.Errorf("tick %d: right pressed = %v, want %v", i, got, s.wantR)
		}
		if x, y := Position(); x != 100+i || y != 200 {
			t.Errorf("tick %d: position %d,%d", i, x, y)
		}
	}
}

// Taking the pointer over with the button already down is a press: the driver
// asked for it on that very tick.
func TestTakeOverWithButtonDownPresses(t *testing.T) {
	defer Release()
	Hold(5, 5, true, false)
	if !JustPressed(ebiten.MouseButtonLeft) {
		t.Fatal("a held pointer that starts pressed must click")
	}
	if !Held() {
		t.Fatal("the pointer is held")
	}
	Release()
	if Held() {
		t.Fatal("released pointer still held")
	}
}

// The real mouse is read where the game says — through the bend of its
// picture — and a driver's held pointer still wins over it.
func TestSetRealReadsTheGamesPointer(t *testing.T) {
	defer SetReal(ebiten.CursorPosition)
	defer Release()
	SetReal(func() (int, int) { return 7, 9 })
	if x, y := Position(); x != 7 || y != 9 {
		t.Errorf("real position %d,%d, want 7,9", x, y)
	}
	Hold(100, 200, false, false)
	if x, y := Position(); x != 100 || y != 200 {
		t.Errorf("held position %d,%d, want 100,200", x, y)
	}
}

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

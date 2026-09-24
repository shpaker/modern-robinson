package minigame

import (
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// The minigames pick pieces by their pixels, but never outside the bitmap: the
// bounds test has to come first, or a click next to a piece reads a neighbour's
// memory. Reading an actual pixel needs a running graphics context, so only the
// guards are exercised here.
func TestOpaqueGuardsBounds(t *testing.T) {
	if Opaque(nil, 0, 0) {
		t.Error("a missing sprite must never answer a click")
	}
	img := ebiten.NewImage(4, 4)
	outside := [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 4}, {4, 4}}
	for _, p := range outside {
		if Opaque(img, p[0], p[1]) {
			t.Errorf("Opaque(%d,%d) is outside a 4x4 sprite", p[0], p[1])
		}
	}
}

// Hit rectangles are half-open like image.Rectangle itself: the engine's
// buttons end one pixel short of their far corner.
func TestInIsHalfOpen(t *testing.T) {
	r := image.Rect(10, 20, 30, 40)
	for _, p := range [][2]int{{10, 20}, {29, 39}} {
		if !In(r, p[0], p[1]) {
			t.Errorf("In(%v) = false, want the corner inside", p)
		}
	}
	for _, p := range [][2]int{{30, 20}, {10, 40}, {9, 25}} {
		if In(r, p[0], p[1]) {
			t.Errorf("In(%v) = true, want the far edge outside", p)
		}
	}
}

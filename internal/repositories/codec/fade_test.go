package codec

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// grayPalette maps index i to the gray level i, so a remap table's mean
// luminance is simply the mean of its bytes.
func grayPalette() types.Palette {
	var p types.Palette
	for i := 0; i < 256; i++ {
		p[i] = [4]byte{byte(i), byte(i), byte(i), 255}
	}
	return p
}

func TestFadeCurve(t *testing.T) {
	// Two steps: identity, then everything to black.
	fad := make([]byte, 512)
	for i := 0; i < 256; i++ {
		fad[i] = byte(i)
		fad[256+i] = 0
	}
	c := FadeCurve(fad, grayPalette())
	if len(c) != 2 {
		t.Fatalf("steps = %d, want 2", len(c))
	}
	if c[0] != 1 {
		t.Errorf("step 0 = %v, want 1 (untouched)", c[0])
	}
	if c[1] != 0 {
		t.Errorf("step 1 = %v, want 0 (black)", c[1])
	}
}

func TestFadeCurveMonotonic(t *testing.T) {
	// Four steps, each halving the index (so each is darker than the last).
	fad := make([]byte, 4*256)
	for s := 0; s < 4; s++ {
		for i := 0; i < 256; i++ {
			fad[s*256+i] = byte(i >> s)
		}
	}
	c := FadeCurve(fad, grayPalette())
	for i := 1; i < len(c); i++ {
		if c[i] >= c[i-1] {
			t.Fatalf("curve not decreasing at %d: %v", i, c)
		}
	}
	if c[0] != 1 {
		t.Errorf("first step = %v, want 1", c[0])
	}
}

func TestFadeCurveShort(t *testing.T) {
	if c := FadeCurve([]byte{1, 2, 3}, grayPalette()); c != nil {
		t.Errorf("truncated table should yield nil, got %v", c)
	}
}

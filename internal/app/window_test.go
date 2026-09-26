package app

import "testing"

// In full screen the pointer rests on the frame's edge, as it did on the
// original's 640x480 screen; left alone, a position beside the frame stays
// beside it, so a window's pointer outside it is on no edge.
func TestHoldToFrame(t *testing.T) {
	cases := []struct {
		name         string
		x, y         float64
		hold         bool
		wantX, wantY float64
	}{
		{"on the frame", 320, 200, true, 320, 200},
		{"left field", -80, 200, true, 0, 200},
		{"right field", 700, 300.5, true, ViewW - 1, 300.5},
		{"below", 320, 500, true, 320, ViewH - 1},
		{"not held", -80, 500, false, -80, 500},
	}
	for _, c := range cases {
		x, y := holdToFrame(c.x, c.y, c.hold)
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: got %v,%v want %v,%v", c.name, x, y, c.wantX, c.wantY)
		}
	}
}

// The tube is on unless config.yml says otherwise; ROBINSON_CRT has the last
// word either way.
func TestTubeOn(t *testing.T) {
	on, off := DefaultConfig(), DefaultConfig()
	off.CRT = false
	t.Setenv("ROBINSON_CRT", "")
	if !tubeOn(on) || tubeOn(off) {
		t.Error("the file's choice is not kept")
	}
	t.Setenv("ROBINSON_CRT", "0")
	if tubeOn(on) {
		t.Error("ROBINSON_CRT=0 left the tube on")
	}
	t.Setenv("ROBINSON_CRT", "1")
	if !tubeOn(off) {
		t.Error("ROBINSON_CRT=1 left the tube off")
	}
}

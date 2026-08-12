package use_cases

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

// scena0Grid mirrors SCENA0's grid parameters (no resources needed).
func scena0Grid() *Grid {
	sc := &types.Scene{
		Size:        [2]int{1024, 400},
		LeftTopGrid: [2]int{15, 150},
		GridSize:    [2]int{144, 36},
		GridShift:   [2]int{46, -99},
		GridLength:  [2]int{8, 5},
		ClosedVert: [][2]int{
			{4, 2},
			{4, 3},
			{5, 2},
			{5, 3},
			{6, 3},
			{5, 4},
			{6, 4},
			{7, 0},
			{7, 1},
			{7, 2},
			{7, 3},
			{7, 4},
		},
	}
	return NewGrid(sc, sc.Size[0], sc.Size[1])
}

func TestToScreen(t *testing.T) {
	g := scena0Grid()
	// anchor = LeftTopGrid + GridShift + cell*GridSize = (61,51) + cell*(144,36).
	cases := []struct {
		gx, gy, x, y int
	}{
		{0, 0, 61, 51},
		{5, 1, 781, 87},
		{2, 3, 349, 159},
	}
	for _, c := range cases {
		if x, y := g.ToScreen(c.gx, c.gy); x != c.x || y != c.y {
			t.Errorf(
				"ToScreen(%d,%d) = (%d,%d), want (%d,%d)",
				c.gx,
				c.gy,
				x,
				y,
				c.x,
				c.y,
			)
		}
	}
}

func TestToCellRoundTrip(t *testing.T) {
	g := scena0Grid()
	// ToCell inverts the bare cell corner, not the GridShifted anchor.
	for gx := 0; gx < 7; gx++ {
		for gy := 0; gy < 5; gy++ {
			x, y := g.Corner(gx, gy)
			if cx, cy := g.ToCell(x, y); cx != gx || cy != gy {
				t.Errorf("ToCell(Corner(%d,%d)) = (%d,%d)", gx, gy, cx, cy)
			}
		}
	}
}

func TestValidAndBounds(t *testing.T) {
	g := scena0Grid()
	if !g.Valid(2, 0) {
		t.Error("(2,0) should be walkable")
	}
	if g.Valid(4, 2) {
		t.Error("(4,2) is closed_vert, must be invalid")
	}
	if g.Valid(8, 0) { // gx == GridLength.x, off the lattice
		t.Error("(8,0) is off-grid, must be invalid")
	}
	if g.Valid(0, 5) { // gy == GridLength.y, off the lattice
		t.Error("(0,5) is off-grid, must be invalid")
	}
}

func TestPath(t *testing.T) {
	g := scena0Grid()
	got := g.Path([2]int{2, 0}, [2]int{5, 1})
	want := [][2]int{{2, 0}, {3, 0}, {4, 0}, {5, 1}}
	if len(got) != len(want) {
		t.Fatalf("path = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path = %v, want %v", got, want)
		}
	}
}

func TestSetVert(t *testing.T) {
	g := scena0Grid()
	// Close an open cell, then reopen it.
	if !g.Valid(2, 0) {
		t.Fatal("(2,0) should start walkable")
	}
	g.SetVert(2, 0, false)
	if g.Valid(2, 0) {
		t.Error("(2,0) should be blocked after SetVert close")
	}
	g.SetVert(2, 0, true)
	if !g.Valid(2, 0) {
		t.Error("(2,0) should be walkable after SetVert open")
	}
	// Open a cell that was closed_vert at load (a door clearing).
	if g.Valid(4, 2) {
		t.Fatal("(4,2) should start blocked")
	}
	g.SetVert(4, 2, true)
	if !g.Valid(4, 2) {
		t.Error("(4,2) should open after SetVert open")
	}
}

func TestScreenToNumpad(t *testing.T) {
	cases := []struct {
		dx, dy float64
		want   int
	}{
		{1, 0, 6},  // east
		{-1, 0, 4}, // west
		{0, 1, 2},  // south (screen y down)
		{0, -1, 8}, // north
		{1, -1, 9}, // north-east
	}
	for _, c := range cases {
		if got := ScreenToNumpad(c.dx, c.dy); got != c.want {
			t.Errorf(
				"ScreenToNumpad(%v,%v) = %d, want %d",
				c.dx,
				c.dy,
				got,
				c.want,
			)
		}
	}
}

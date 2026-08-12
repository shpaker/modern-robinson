package use_cases

import (
	"strings"
	"testing"
)

func TestStepDir(t *testing.T) {
	cases := []struct {
		from, to [2]int
		want     int
	}{
		{[2]int{2, 2}, [2]int{3, 2}, 6}, // east
		{[2]int{2, 2}, [2]int{1, 2}, 4}, // west
		{[2]int{2, 2}, [2]int{2, 3}, 2}, // south (towards the viewer)
		{[2]int{2, 2}, [2]int{2, 1}, 8}, // north
		{[2]int{2, 2}, [2]int{3, 3}, 3}, // south-east
		{[2]int{2, 2}, [2]int{1, 1}, 7}, // north-west
		{[2]int{2, 2}, [2]int{2, 2}, 5}, // standing
	}
	for _, c := range cases {
		if got := StepDir(c.from, c.to); got != c.want {
			t.Errorf("StepDir(%v,%v) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
}

func TestWalkCyclesStraightLine(t *testing.T) {
	// Three steps east: accelerate, two full steps, brake.
	got := WalkCycles([2]int{0, 0}, [][2]int{{1, 0}, {2, 0}, {3, 0}}, false)
	want := []string{"56", "66", "66", "65"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWalkCyclesTurn(t *testing.T) {
	// East then south: the middle cycle carries the turn (6 -> 2).
	got := WalkCycles([2]int{0, 0}, [][2]int{{1, 0}, {1, 1}}, false)
	want := []string{"56", "62", "25"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestWalkCyclesOneStep(t *testing.T) {
	got := WalkCycles([2]int{5, 5}, [][2]int{{5, 4}}, false)
	want := []string{"58", "85"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Every cycle but the first advances one cell, so a route of n cells yields
// n+1 cycles.
func TestWalkCyclesCount(t *testing.T) {
	route := [][2]int{{1, 0}, {2, 0}, {2, 1}, {3, 1}, {3, 2}}
	got := WalkCycles([2]int{0, 0}, route, false)
	if len(got) != len(route)+1 {
		t.Fatalf("cycles = %d for %d cells, want %d",
			len(got), len(route), len(route)+1)
	}
}

func TestWalkCyclesArrowsOnly(t *testing.T) {
	// Friday walks the four arrows: diagonals collapse to north/south.
	got := WalkCycles([2]int{0, 0}, [][2]int{{1, 1}, {2, 0}}, true)
	want := []string{"52", "28", "85"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, c := range got {
		for _, r := range c {
			if r != '5' && !arrowDirs[int(r-'0')] {
				t.Fatalf("cycle %q uses a diagonal", c)
			}
		}
	}
}

func TestWalkCyclesEmpty(t *testing.T) {
	if got := WalkCycles([2]int{1, 1}, nil, false); got != nil {
		t.Fatalf("empty route = %v, want nil", got)
	}
	// A route that never leaves the cell yields nothing to play.
	if got := WalkCycles([2]int{1, 1}, [][2]int{{1, 1}}, false); got != nil {
		t.Fatalf("still route = %v, want nil", got)
	}
}

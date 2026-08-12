// Package use_cases is the Application layer: stateless game logic operating on
// Domain entities through interfaces. See ARCHITECTURE.md.
package use_cases

import (
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// Grid is the scene walk grid: a plain rectangular lattice of GridLength cells
// of GridSize pixels, with screen<->cell mapping and BFS path-finding. It
// implements interfaces.IGrid.
//
// The engine (ROBY.EXE, FonScript::Render) places every sprite at
//
//	anchor(gx,gy) = LeftTopGrid + GridShift + (gx*GridSize.x, gy*GridSize.y)
//
// while object hit-zones and the cell picker are anchored to the bare cell
// corner (LeftTopGrid + cell*GridSize), without GridShift. ToScreen returns the
// anchor; Corner returns the corner; ToCell inverts Corner.
type Grid struct {
	lx, ly, sx, sy, hx, hy, nx, ny int
	blocked                        map[[2]int]bool
}

var _ interfaces.IGrid = (*Grid)(nil)

// NewGrid builds a walk grid from a scene. The bounds arguments are accepted for
// compatibility but no longer clip the lattice: every declared cell is valid.
func NewGrid(sc *types.Scene, _, _ int) *Grid {
	g := &Grid{
		lx: sc.LeftTopGrid[0], ly: sc.LeftTopGrid[1],
		sx: sc.GridSize[0], sy: sc.GridSize[1],
		hx: sc.GridShift[0], hy: sc.GridShift[1],
		nx: sc.GridLength[0], ny: sc.GridLength[1],
		blocked: map[[2]int]bool{},
	}
	if g.sx == 0 {
		g.sx = 1
	}
	if g.sy == 0 {
		g.sy = 1
	}
	for _, c := range sc.ClosedVert {
		g.blocked[c] = true
	}
	return g
}

// ToScreen maps a cell to its sprite anchor: corner plus GridShift.
func (g *Grid) ToScreen(gx, gy int) (int, int) {
	return g.lx + g.hx + gx*g.sx, g.ly + g.hy + gy*g.sy
}

// Corner maps a cell to its bare top-left pixel (no GridShift): the anchor for
// object hit-zones and the inverse of ToCell.
func (g *Grid) Corner(gx, gy int) (int, int) {
	return g.lx + gx*g.sx, g.ly + gy*g.sy
}

// Dims returns the grid dimensions in cells.
func (g *Grid) Dims() (int, int) { return g.nx, g.ny }

// Blocked reports whether a cell is in the scene's closed_vert list.
func (g *Grid) Blocked(gx, gy int) bool { return g.blocked[[2]int{gx, gy}] }

// SetVert toggles a cell's passability at runtime (the SetVert command): open
// makes it walkable, close/closed blocks it. Doors and cleared obstacles use
// this to reshape the walk grid mid-scene.
func (g *Grid) SetVert(gx, gy int, open bool) {
	if open {
		delete(g.blocked, [2]int{gx, gy})
	} else {
		g.blocked[[2]int{gx, gy}] = true
	}
}

// floorDiv divides rounding toward negative infinity, so a point just left of
// the grid maps to a negative cell rather than snapping to column zero.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// ToCell maps a screen point to its cell on the rectangular lattice.
func (g *Grid) ToCell(px, py int) (int, int) {
	return floorDiv(px-g.lx, g.sx), floorDiv(py-g.ly, g.sy)
}

// Valid reports whether a cell is on-grid and unblocked.
func (g *Grid) Valid(gx, gy int) bool {
	if gx < 0 || gx >= g.nx || gy < 0 || gy >= g.ny {
		return false
	}
	return !g.blocked[[2]int{gx, gy}]
}

// NearestFree returns the closest walkable cell to (gx,gy).
func (g *Grid) NearestFree(gx, gy int) (int, int, bool) {
	gx = clamp(gx, 0, g.nx-1)
	gy = clamp(gy, 0, g.ny-1)
	if g.Valid(gx, gy) {
		return gx, gy, true
	}
	best := [2]int{}
	found := false
	bd := 1 << 30
	for y := 0; y < g.ny; y++ {
		for x := 0; x < g.nx; x++ {
			if g.Valid(x, y) {
				d := (x-gx)*(x-gx) + (y-gy)*(y-gy)
				if d < bd {
					bd, best, found = d, [2]int{x, y}, true
				}
			}
		}
	}
	return best[0], best[1], found
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// dirs8 lists the eight step directions with the straight ones first. Both a
// diagonal and a straight route can be the same number of cells, and the search
// keeps whichever it reaches first — trying straight steps first stops it from
// answering with a zig-zag when a level walk exists.
var dirs8 = [8][2]int{
	{-1, 0},
	{1, 0},
	{0, -1},
	{0, 1},
	{-1, -1},
	{1, -1},
	{-1, 1},
	{1, 1},
}

// Path returns cells from start to goal (inclusive) via BFS, or nil if none.
func (g *Grid) Path(start, goal [2]int) [][2]int {
	if start == goal {
		return [][2]int{goal}
	}
	if !g.Valid(goal[0], goal[1]) {
		return nil
	}
	prev := map[[2]int][2]int{start: {-1, -1}}
	queue := [][2]int{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == goal {
			break
		}
		for _, d := range dirs8 {
			nb := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if _, seen := prev[nb]; !seen && g.Valid(nb[0], nb[1]) {
				prev[nb] = cur
				queue = append(queue, nb)
			}
		}
	}
	if _, ok := prev[goal]; !ok {
		return nil
	}
	var rev [][2]int
	for c := goal; c != [2]int{-1, -1}; c = prev[c] {
		rev = append(rev, c)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

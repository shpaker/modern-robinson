// Package use_cases is the Application layer: stateless game logic operating on
// Domain entities through interfaces. See ARCHITECTURE.md.
package use_cases

import (
	"github.com/shpaker/modern-robinson/internal/interfaces"
	"github.com/shpaker/modern-robinson/internal/types"
)

// Grid is the isometric walk grid: screen<->cell mapping and BFS path-finding.
// It implements interfaces.IGrid.
type Grid struct {
	lx, ly, sx, sy, hx, hy, nx, ny int
	blocked                        map[[2]int]bool
	boundsW, boundsH               int
	det                            int
}

var _ interfaces.IGrid = (*Grid)(nil)

// NewGrid builds a walk grid from a scene, clipped to the given screen bounds.
func NewGrid(sc *types.Scene, boundsW, boundsH int) *Grid {
	g := &Grid{
		lx: sc.LeftTopGrid[0], ly: sc.LeftTopGrid[1],
		sx: sc.GridSize[0], sy: sc.GridSize[1],
		hx: sc.GridShift[0], hy: sc.GridShift[1],
		nx: sc.GridLength[0], ny: sc.GridLength[1],
		blocked: map[[2]int]bool{},
		boundsW: boundsW, boundsH: boundsH,
	}
	for _, c := range sc.ClosedVert {
		g.blocked[c] = true
	}
	g.det = g.sx*g.hy - g.hx*g.sy
	if g.det == 0 {
		g.det = 1
	}
	return g
}

// ToScreen maps a cell to its screen anchor point.
func (g *Grid) ToScreen(gx, gy int) (int, int) {
	return g.lx + gx*g.sx + gy*g.hx, g.ly + gx*g.sy + gy*g.hy
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

func round(x float64) int {
	if x < 0 {
		return int(x - 0.5)
	}
	return int(x + 0.5)
}

// ToCell maps a screen point to the nearest cell.
func (g *Grid) ToCell(px, py int) (int, int) {
	dx := float64(px - g.lx)
	dy := float64(py - g.ly)
	det := float64(g.det)
	gx := (dx*float64(g.hy) - float64(g.hx)*dy) / det
	gy := (float64(g.sx)*dy - dx*float64(g.sy)) / det
	return round(gx), round(gy)
}

// Valid reports whether a cell is on-grid, unblocked, and within scene bounds.
func (g *Grid) Valid(gx, gy int) bool {
	if gx < 0 || gx >= g.nx || gy < 0 || gy >= g.ny {
		return false
	}
	if g.blocked[[2]int{gx, gy}] {
		return false
	}
	x, y := g.ToScreen(gx, gy)
	return x >= 0 && x < g.boundsW && y >= 0 && y < g.boundsH
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

var dirs8 = [8][2]int{
	{-1, -1},
	{0, -1},
	{1, -1},
	{-1, 0},
	{1, 0},
	{-1, 1},
	{0, 1},
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

package app

import (
	"math/rand"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The torn sea-chart jigsaw (StartGame 0 -> MapOK). Twelve ragged fragments
// float freely on the mat; there is no printed target — instead adjacent
// fragments snap to each other, aligned groups are picked up and dragged as
// one, and the chart is done when all twelve agree on their relative offsets.
type mapGame struct {
	sprites map[string]*ebiten.Image
	pos     [12][2]int // fragment centre
	rot     [12]int
	z       [12]int
	sel     [12]bool // the group currently carried
	held    bool
	won     bool
	finishT float64
}

// mapTargets are the fragments' offsets within the assembled chart.
var mapTargets = [12][2]int{
	{111, 121},
	{255, 79},
	{163, 0},
	{231, 0},
	{141, 182},
	{221, 161},
	{164, 283},
	{275, 243},
	{0, 167},
	{77, 185},
	{0, 0},
	{10, 0},
}

// mapAdj says which fragments share a torn edge.
var mapAdj = [12][12]bool{
	{
		false,
		true,
		true,
		false,
		true,
		true,
		false,
		false,
		false,
		true,
		true,
		true,
	},
	{
		true,
		false,
		true,
		true,
		false,
		true,
		false,
		false,
		false,
		false,
		false,
		false,
	},
	{
		true,
		true,
		false,
		true,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		true,
	},
	{
		false,
		true,
		true,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
	},
	{
		true,
		false,
		false,
		false,
		false,
		true,
		true,
		false,
		false,
		true,
		false,
		false,
	},
	{
		true,
		true,
		false,
		false,
		true,
		false,
		true,
		true,
		false,
		false,
		false,
		false,
	},
	{
		false,
		false,
		false,
		false,
		true,
		true,
		false,
		true,
		false,
		true,
		false,
		false,
	},
	{
		false,
		false,
		false,
		false,
		false,
		true,
		true,
		false,
		false,
		false,
		false,
		false,
	},
	{
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		true,
		true,
		false,
	},
	{
		true,
		false,
		false,
		false,
		true,
		false,
		true,
		false,
		true,
		false,
		true,
		false,
	},
	{
		true,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		true,
		true,
		false,
		true,
	},
	{
		true,
		false,
		true,
		false,
		false,
		false,
		false,
		false,
		false,
		false,
		true,
		false,
	},
}

var mapExit = houseExit // the same floppy button spot

// newMapGame loads MAP.DAT and scatters the fragments over the whole mat.
func newMapGame(g *Game) minigame {
	m := &mapGame{}
	m.sprites = g.packImages("MAP")
	if m.sprites["DESK"] == nil {
		return nil
	}
	for i := 0; i < 12; i++ {
		m.rot[i] = rand.Intn(3)
		w, h := m.size(i)
		spanX := ViewW - w - 8
		if spanX < 1 {
			spanX = 1
		}
		spanY := ViewH - h - 8
		if spanY < 1 {
			spanY = 1
		}
		m.pos[i] = [2]int{
			rand.Intn(spanX) + 4 + w/2,
			rand.Intn(spanY) + 4 + h/2,
		}
		m.z[i] = i
	}
	return m
}

func (m *mapGame) sprite(i int) *ebiten.Image {
	return m.sprites["M"+strconv.Itoa(i+1)+strconv.Itoa(m.rot[i]+1)]
}

func (m *mapGame) size(i int) (int, int) {
	img := m.sprite(i)
	if img == nil {
		return 1, 1
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

func (m *mapGame) topLeft(i int) (int, int) {
	w, h := m.size(i)
	return m.pos[i][0] - w/2, m.pos[i][1] - h/2
}

// glued reports whether j sits exactly where the chart wants it relative to i.
func (m *mapGame) glued(i, j int) bool {
	if !mapAdj[i][j] || m.rot[j] != 0 {
		return false
	}
	ix, iy := m.topLeft(i)
	jx, jy := m.topLeft(j)
	return jx-ix == mapTargets[j][0]-mapTargets[i][0] &&
		jy-iy == mapTargets[j][1]-mapTargets[i][1]
}

// collectGroup marks every fragment glued (transitively) to i as selected.
func (m *mapGame) collectGroup(i int) {
	m.sel[i] = true
	for j := 0; j < 12; j++ {
		if !m.sel[j] && m.glued(i, j) {
			m.collectGroup(j)
		}
	}
}

// selCount is how many fragments the carried group holds.
func (m *mapGame) selCount() int {
	n := 0
	for _, s := range m.sel {
		if s {
			n++
		}
	}
	return n
}

// update drives pick (with gluing), carry, rotate, snap and the win test.
func (m *mapGame) update(g *Game, dt float64) (bool, int) {
	if m.won {
		m.finishT += dt
		return m.finishT > 3 || clickedThisTick(), 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true, 0
	}
	mx, my := ebiten.CursorPosition()
	if m.held {
		// The whole group follows the cursor via its anchor piece.
		anchor := -1
		for i := 0; i < 12; i++ {
			if m.sel[i] {
				anchor = i
				break
			}
		}
		dx, dy := mx-m.pos[anchor][0], my-m.pos[anchor][1]
		for i := 0; i < 12; i++ {
			if m.sel[i] {
				m.pos[i][0] += dx
				m.pos[i][1] += dy
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) &&
			m.selCount() == 1 {
			// Only a lone fragment can be turned.
			m.rot[anchor] = (m.rot[anchor] + 1) & 3
			g.playSound([]string{"m_turn.wav", "1"})
		}
		if !clickedThisTick() {
			return false, 0
		}
		m.held = false
		m.drop(g)
		for i := range m.sel {
			m.sel[i] = false
		}
		return false, 0
	}
	if !clickedThisTick() {
		return false, 0
	}
	if pointIn(mapExit, mx, my) {
		return true, 0
	}
	// Pick the front-most fragment under the cursor and glue its group.
	best, bestZ := -1, 1<<30
	for i := 0; i < 12; i++ {
		if m.z[i] >= bestZ {
			continue
		}
		x, y := m.topLeft(i)
		if opaqueAt(m.sprite(i), mx-x, my-y) {
			best, bestZ = i, m.z[i]
		}
	}
	if best < 0 {
		return false, 0
	}
	for i := range m.sel {
		m.sel[i] = false
	}
	if m.rot[best] == 0 {
		m.collectGroup(best)
	} else {
		m.sel[best] = true
	}
	for i := 0; i < 12; i++ {
		if m.sel[i] {
			m.z[i] = 0
		} else {
			m.z[i]++
		}
	}
	m.held = true
	g.playSound([]string{"m_take.wav", "1"})
	return false, 0
}

// drop snaps the carried group to any adjacent resting fragment within the
// engine's ten-pixel tolerance, then tests the whole chart.
func (m *mapGame) drop(g *Game) {
	snapped := false
	for a := 0; a < 12 && !snapped; a++ {
		if !m.sel[a] || m.rot[a] != 0 {
			continue
		}
		for b := 0; b < 12; b++ {
			if m.sel[b] || m.rot[b] != 0 || !mapAdj[a][b] {
				continue
			}
			ax, ay := m.topLeft(a)
			bx, by := m.topLeft(b)
			wantX := mapTargets[a][0] - mapTargets[b][0]
			wantY := mapTargets[a][1] - mapTargets[b][1]
			dx := wantX - (ax - bx)
			dy := wantY - (ay - by)
			if abs(dx) < 10 && abs(dy) < 10 {
				for i := 0; i < 12; i++ {
					if m.sel[i] {
						m.pos[i][0] += dx
						m.pos[i][1] += dy
					}
				}
				g.playSound([]string{"m_good.wav", "1"})
				snapped = true
				break
			}
		}
	}
	if !snapped {
		g.playSound([]string{"m_put.wav", "1"})
	}
	// Win: every fragment upright and aligned with fragment 10.
	x10, y10 := m.topLeft(10)
	for i := 0; i < 12; i++ {
		if m.rot[i] != 0 {
			return
		}
		x, y := m.topLeft(i)
		if x-x10 != mapTargets[i][0] || y-y10 != mapTargets[i][1] {
			return
		}
	}
	m.won = true
	g.playSound([]string{"final0.wav", "1"})
}

// draw paints the mat and the fragments back-to-front.
func (m *mapGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, m.sprites["DESK"], 0, 0)
	order := make([]int, 12)
	for i := range order {
		order[i] = i
	}
	for a := 0; a < 12; a++ {
		for b := a + 1; b < 12; b++ {
			if m.z[order[a]] < m.z[order[b]] {
				order[a], order[b] = order[b], order[a]
			}
		}
	}
	for _, i := range order {
		x, y := m.topLeft(i)
		blitAt(screen, m.sprite(i), x, y)
	}
}

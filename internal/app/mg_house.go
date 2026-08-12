package app

import (
	"image"
	"math/rand"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The hut jigsaw (StartGame 1 -> House). Seventeen logs and two leaf bundles
// are scattered on the right half of the screen; they go onto the orange hut
// silhouette in a fixed order, each at its authored spot, upright only.
// H<i+1>1..4 are the four authored rotations of piece i.
type houseGame struct {
	sprites map[string]*ebiten.Image
	pos     [17][2]int // piece centre
	rot     [17]int
	z       [17]int
	placed  [17]bool
	held    int
	fallTo  int // ground the held drop falls to, -1 = not falling
	fallPc  int
	won     bool
	finishT float64
}

// houseTargets are the pieces' authored top-left positions on the silhouette.
var houseTargets = [17][2]int{
	{150, 61},
	{129, 97},
	{77, 138},
	{45, 185},
	{186, 182},
	{41, 231},
	{174, 222},
	{257, 210},
	{38, 257},
	{39, 285},
	{80, 289},
	{40, 328},
	{41, 379},
	{272, 348},
	{273, 285},
	{11, 22},
	{175, 50},
}

// housePrereq lists which pieces must already stand before piece i may go on.
var housePrereq = [17][]int{
	{1},
	{2},
	{3, 4},
	{5, 6},
	{6, 7},
	{8},
	{8},
	{14},
	{9, 10},
	{11},
	{11, 9},
	{12},
	{},
	{12},
	{13},
	{0},
	{0},
}

var houseExit = image.Rect(565, 406, 633, 472)

// newHouseGame loads HOUSE.DAT and scatters the pieces over the right strip.
func newHouseGame(g *Game) minigame {
	h := &houseGame{held: -1, fallTo: -1, fallPc: -1}
	h.sprites = g.packImages("HOUSE")
	if h.sprites["BACK"] == nil {
		return nil
	}
	for i := 0; i < 17; i++ {
		h.rot[i] = rand.Intn(3)
		w, hh := h.size(i)
		spanX := 315 - w - 8
		if spanX < 1 {
			spanX = 1
		}
		spanY := 479 - hh - 8
		if spanY < 1 {
			spanY = 1
		}
		h.pos[i] = [2]int{
			rand.Intn(spanX) + 324 + 4 + w/2,
			rand.Intn(spanY) + 4 + hh/2,
		}
		h.z[i] = i
	}
	return h
}

// sprite returns piece i's image at its current rotation.
func (h *houseGame) sprite(i int) *ebiten.Image {
	return h.sprites["H"+strconv.Itoa(i+1)+strconv.Itoa(h.rot[i]+1)]
}

// size is the piece's current width and height.
func (h *houseGame) size(i int) (int, int) {
	img := h.sprite(i)
	if img == nil {
		return 1, 1
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

// topLeft is where the piece is drawn (it is stored by centre).
func (h *houseGame) topLeft(i int) (int, int) {
	w, hh := h.size(i)
	return h.pos[i][0] - w/2, h.pos[i][1] - hh/2
}

// bringToFront gives piece i the smallest z.
func (h *houseGame) bringToFront(i int) {
	for j := range h.z {
		h.z[j]++
	}
	h.z[i] = 0
}

// prereqsPlaced reports whether piece i's supports already stand.
func (h *houseGame) prereqsPlaced(i int) bool {
	for _, p := range housePrereq[i] {
		if !h.placed[p] {
			return false
		}
	}
	return true
}

// update drives pick, carry, rotate, snap and the drop fall.
func (h *houseGame) update(g *Game, dt float64) (bool, int) {
	if h.won {
		h.finishT += dt
		return h.finishT > 3 || clickedThisTick(), 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true, 0
	}
	mx, my := ebiten.CursorPosition()
	// A dropped piece falls to its resting height.
	if h.fallPc >= 0 {
		h.pos[h.fallPc][1] += 20
		if h.pos[h.fallPc][1] >= h.fallTo {
			h.pos[h.fallPc][1] = h.fallTo
			if h.placed[h.fallPc] {
				g.playSound([]string{"h_good.wav", "1"})
				if h.placed[15] && h.placed[16] {
					h.won = true
					g.playSound([]string{"final1.wav", "1"})
				}
			} else {
				g.playSound([]string{"h_error.wav", "1"})
			}
			h.fallPc = -1
		}
		return false, 0
	}
	if h.held >= 0 {
		h.pos[h.held] = [2]int{mx, my}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			// Rotate about the cursor.
			h.rot[h.held] = (h.rot[h.held] + 1) & 3
			g.playSound([]string{"h_turn.wav", "1"})
		}
		if !clickedThisTick() {
			return false, 0
		}
		i := h.held
		h.held = -1
		if mx >= 324 { // back to the tray
			h.placed[i] = false
			g.playSound([]string{"h_back.wav", "1"})
			return false, 0
		}
		w, hh := h.size(i)
		tx := houseTargets[i][0] + w/2
		ty := houseTargets[i][1] + hh/2
		if h.rot[i] == 0 && h.prereqsPlaced(i) &&
			abs(h.pos[i][0]-tx) <= 20 && h.pos[i][1] <= ty &&
			ty-h.pos[i][1] <= 280 {
			h.pos[i][0] = tx
			h.placed[i] = true
			h.fallPc, h.fallTo = i, ty
		} else {
			h.placed[i] = false
			h.fallPc, h.fallTo = i, 400+rand.Intn(50)
		}
		return false, 0
	}
	if !clickedThisTick() {
		return false, 0
	}
	if pointIn(houseExit, mx, my) {
		return true, 0
	}
	// Pick the front-most piece whose pixel is under the cursor.
	best, bestZ := -1, 1<<30
	for i := 0; i < 17; i++ {
		if h.placed[i] || h.z[i] >= bestZ {
			continue
		}
		x, y := h.topLeft(i)
		if opaqueAt(h.sprite(i), mx-x, my-y) {
			best, bestZ = i, h.z[i]
		}
	}
	if best >= 0 {
		h.held = best
		h.bringToFront(best)
		g.playSound([]string{"h_take.wav", "1"})
	}
	return false, 0
}

// draw paints the silhouette and the pieces back-to-front.
func (h *houseGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, h.sprites["BACK"], 0, 0)
	for z := 16; z >= 0; z-- {
		for i := 0; i < 17; i++ {
			if h.z[i] != z {
				continue
			}
			x, y := h.topLeft(i)
			blitAt(screen, h.sprite(i), x, y)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

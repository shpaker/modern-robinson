package app

import (
	"image"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The bamboo organ (StartGame 4 -> OrganOK), rebuilt to the original's rules:
// this is an assembly puzzle, not a repeat-after-me. The eight tubes start in
// the top row and are carried (click to pick, click to drop) into the mouths on
// the bottom rail. The listen hotspot plays a fifteen-note phrase over the
// mouths; a mouth sounds its tube's note, an empty mouth plays the dud note.
// The organ is solved when the tubes stand in the right order.
type pipeGame struct {
	sprites map[string]*ebiten.Image
	avail   [8]bool // which tubes the hero has (the Tubs mask)
	mouth   [8]int  // tube -> mouth, -1 = at home
	inMouth [8]int  // mouth -> tube, -1 = empty
	held    int     // tube being carried, -1 = none

	playing bool // the phrase is sounding
	noteIdx int
	noteT   float64

	done    bool
	result  int
	finishT float64
}

// Geometry from the engine: tube homes at (80i, 5) in 78x78 cells, mouth m's
// hit rectangle (75m-2, 400)-(75m+73, 470), and per-tube seating offsets.
const (
	pipeHomeY  = 5
	pipeHomeDX = 80
	pipeCell   = 78
)

var pipeSeat = [8][2]int{
	{-2, -14}, {0, -5}, {2, 0}, {2, -4}, {2, -8}, {0, -5}, {9, -4}, {-3, 0},
}

// pipeListen is the "play the melody" hotspot on the drummer.
var pipeListen = image.Rect(201, 199, 264, 302)

// The phrase: which mouth sounds on each beat, and for how many 150 ms ticks.
var (
	pipeTune = [15]int{0, 1, 0, 1, 3, 0, 2, 4, 4, 4, 5, 6, 1, 1, 7}
	pipeBeat = [15]int{2, 2, 2, 2, 2, 2, 4, 2, 2, 2, 1, 1, 2, 2, 4}
)

const pipeTick = 0.150

// The two tube arrangements the engine accepts (tubes 0 and 7 sound alike).
var pipeWins = [2][8]int{
	{2, 0, 1, 3, 4, 5, 6, 7},
	{2, 7, 1, 3, 4, 5, 6, 0},
}

// newPipeGame builds the organ. The Tubs variable is a bit mask: bit 2 grants
// tubes 0-5, bit 1 tube 7, bit 0 tube 6 — the quest reaches 7 (all of them).
func newPipeGame(g *Game) minigame {
	p := &pipeGame{held: -1}
	p.sprites = g.packImages("PIPE")
	if p.sprites["BACK"] == nil {
		return nil
	}
	mask := g.mgParam
	for i := 0; i < 6; i++ {
		p.avail[i] = mask&4 != 0
	}
	p.avail[7] = mask&2 != 0
	p.avail[6] = mask&1 != 0
	for i := range p.mouth {
		p.mouth[i] = -1
		p.inMouth[i] = -1
	}
	return p
}

// mouthRect is mouth m's hit rectangle on the bottom rail.
func mouthRect(m int) image.Rectangle {
	return image.Rect(75*m-2, 400, 75*m+73, 470)
}

// tubeRect is the tube's current 78x78 cell on screen.
func (p *pipeGame) tubeRect(i int) image.Rectangle {
	var x, y int
	if m := p.mouth[i]; m >= 0 {
		x = 75*m - 2 + pipeSeat[i][0]
		y = 400 + pipeSeat[i][1] - pipeCell + 70 // seat the cell on the rail
	} else {
		x, y = pipeHomeDX*i, pipeHomeY
	}
	return image.Rect(x, y, x+pipeCell, y+pipeCell)
}

// note sounds mouth m with whatever tube sits in it (the dud when empty).
func (p *pipeGame) note(g *Game, m int) {
	t := p.inMouth[m]
	name := "pipe00.wav"
	if t >= 0 {
		name = "pipe0" + strconv.Itoa(t+1) + ".wav"
	}
	g.playSound([]string{name, "3"})
}

// solvedNow tests the two accepted arrangements.
func (p *pipeGame) solvedNow() bool {
	for _, w := range pipeWins {
		ok := true
		for m, t := range w {
			if p.inMouth[m] != t {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// update carries tubes and drives the phrase playback.
func (p *pipeGame) update(g *Game, dt float64) (bool, int) {
	if p.done {
		p.finishT += dt
		return p.finishT > 2 || clickedThisTick(), p.result
	}
	if p.playing {
		p.noteT -= dt
		if p.noteT <= 0 {
			if p.noteIdx < len(pipeTune) {
				p.note(g, pipeTune[p.noteIdx])
				p.noteT = pipeTick * float64(pipeBeat[p.noteIdx])
				p.noteIdx++
			} else {
				p.playing = false
				if p.solvedNow() {
					p.done, p.result = true, 1
				}
			}
		}
		return false, 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true, 0
	}
	if !clickedThisTick() {
		return false, 0
	}
	mx, my := ebiten.CursorPosition()
	if p.held < 0 {
		if pointIn(pipeListen, mx, my) {
			p.playing, p.noteIdx, p.noteT = true, 0, 0.4
			return false, 0
		}
		// Pick the tube under the cursor (top row or seated in a mouth).
		for i := 0; i < 8; i++ {
			if !p.avail[i] || !pointIn(p.tubeRect(i), mx, my) {
				continue
			}
			if m := p.mouth[i]; m >= 0 {
				p.inMouth[m] = -1
				p.mouth[i] = -1
			}
			p.held = i
			return false, 0
		}
		return false, 0
	}
	// Carrying a tube: drop it into an empty mouth, else send it home.
	for m := 0; m < 8; m++ {
		if pointIn(mouthRect(m), mx, my) && p.inMouth[m] < 0 {
			p.mouth[p.held] = m
			p.inMouth[m] = p.held
			p.held = -1
			p.note(g, m)
			return false, 0
		}
	}
	p.held = -1
	return false, 0
}

// draw paints the beach, the organ, the tubes and the one in hand.
func (p *pipeGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, p.sprites["BACK"], 0, 0)
	blitAt(screen, p.sprites["PIPE18"], 255, 99) // the organ frame
	for i := 0; i < 8; i++ {
		if !p.avail[i] || i == p.held {
			continue
		}
		r := p.tubeRect(i)
		blitAt(screen, p.sprites["PIPE1"+strconv.Itoa(i)], r.Min.X, r.Min.Y)
	}
	blitAt(screen, p.sprites["PIPE112"], 6, 443) // the rail over seated tubes
	if p.held >= 0 {
		mx, my := ebiten.CursorPosition()
		blitAt(screen, p.sprites["PIPE1"+strconv.Itoa(p.held)],
			mx-pipeCell/2, my-pipeCell/2)
	}
}

package app

import (
	"image"
	"math/rand"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
)

// The bamboo organ (StartGame 4 -> OrganOK). PIPE.DAT draws the beach with the
// chief's organ: eight tube slots along the top and eight playable mouths on the
// rail at the bottom, each sounding PIPE00..PIPE07. The chief plays a phrase and
// the player repeats it; every correct round adds one note.
type pipeGame struct {
	sprites map[string]*ebiten.Image
	seq     []int // the phrase to repeat, grown one note per round
	pos     int   // how much of it the player has echoed back
	round   int

	playing  bool    // the organ is demonstrating the phrase
	playIdx  int     // which note of the demo is due
	timer    float64 // countdown to the next demo note
	pressed  int     // key lit this instant, -1 = none
	pressT   float64
	done     bool
	result   int
	finishT  float64
	rng      *rand.Rand
	keyCount int
}

// pipeRounds is how many phrases must be echoed back to win.
const pipeRounds = 4

// pipeKeyX are the mouths' left edges on the bottom rail; each is 23 px wide.
var pipeKeyX = [8]int{28, 103, 178, 253, 328, 403, 478, 553}

const (
	pipeKeyY = 443
	pipeKeyW = 23
	pipeKeyH = 35
)

// pipeSlotX are the eight item slots along the top (interiors, 74x67).
var pipeSlotX = [8]int{6, 86, 165, 244, 323, 402, 481, 560}

const (
	pipeSlotY = 6
	pipeSlotW = 74
	pipeSlotH = 67
)

// newPipeGame builds the organ puzzle. paramVar (Tubs) counts the tubes the
// hero has fitted, which is how many mouths actually sound.
func newPipeGame(g *Game) minigame {
	p := &pipeGame{
		sprites:  g.packImages("PIPE"),
		pressed:  -1,
		rng:      rand.New(rand.NewSource(int64(g.mgParam)*7919 + 13)),
		keyCount: len(pipeKeyX),
	}
	if p.sprites["BACK"] == nil {
		return nil
	}
	p.nextRound(g)
	return p
}

// nextRound appends a note and starts the demonstration.
func (p *pipeGame) nextRound(g *Game) {
	p.seq = append(p.seq, p.rng.Intn(p.keyCount))
	p.pos, p.round = 0, p.round+1
	p.playing, p.playIdx, p.timer = true, 0, 0.6
	_ = g
}

// note plays a mouth's sound and lights it.
func (p *pipeGame) note(g *Game, i int) {
	if i < 0 || i >= p.keyCount {
		return
	}
	g.playSound([]string{"pipe0" + strconv.Itoa(i) + ".wav", "3"})
	p.pressed, p.pressT = i, 0.25
}

// update demonstrates the phrase, then takes the player's echo.
func (p *pipeGame) update(g *Game, dt float64) (bool, int) {
	if p.pressT > 0 {
		if p.pressT -= dt; p.pressT <= 0 {
			p.pressed = -1
		}
	}
	if p.done {
		p.finishT += dt
		return p.finishT > 2, p.result
	}
	if p.playing {
		p.timer -= dt
		if p.timer <= 0 {
			if p.playIdx < len(p.seq) {
				p.note(g, p.seq[p.playIdx])
				p.playIdx++
				p.timer = 0.75
			} else {
				p.playing = false
			}
		}
		return false, 0
	}
	if !clickedThisTick() {
		return false, 0
	}
	mx, my := ebiten.CursorPosition()
	if pointIn(image.Rect(560, 400, 640, 480), mx, my) {
		p.done, p.result = true, 0 // the floppy corner leaves the game
		return false, 0
	}
	k := p.keyAt(mx, my)
	if k < 0 {
		return false, 0
	}
	p.note(g, k)
	if k != p.seq[p.pos] {
		g.playSound([]string{"rofail.wav", "1"})
		p.done, p.result = true, 0
		return false, 0
	}
	p.pos++
	if p.pos < len(p.seq) {
		return false, 0
	}
	if p.round >= pipeRounds {
		g.playSound([]string{"melody.wav", "1"})
		p.done, p.result = true, 1
		return false, 0
	}
	p.nextRound(g)
	return false, 0
}

// keyAt returns the mouth under the cursor, or -1.
func (p *pipeGame) keyAt(mx, my int) int {
	for i, x := range pipeKeyX {
		if pointIn(
			image.Rect(x, pipeKeyY, x+pipeKeyW, pipeKeyY+pipeKeyH),
			mx,
			my,
		) {
			return i
		}
	}
	return -1
}

// draw paints the beach, the tubes in their slots and the lit mouth.
func (p *pipeGame) draw(g *Game, screen *ebiten.Image) {
	blitAt(screen, p.sprites["BACK"], 0, 0)
	// The tubes the hero has collected sit in the slots; PIPE1<i> are authored
	// in a shared 78x78 cell, so they are centred into each slot interior.
	tubes := g.mgParam
	if tubes > len(pipeSlotX) {
		tubes = len(pipeSlotX)
	}
	for i := 0; i < tubes; i++ {
		img := p.sprites["PIPE1"+strconv.Itoa(i)]
		if img == nil {
			continue
		}
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		x := pipeSlotX[i] + (pipeSlotW-w)/2
		y := pipeSlotY + (pipeSlotH-h)/2
		blitAt(screen, img, x, y)
	}
	if p.pressed >= 0 {
		r := image.Rect(pipeKeyX[p.pressed], pipeKeyY,
			pipeKeyX[p.pressed]+pipeKeyW, pipeKeyY+pipeKeyH)
		strokeRect(screen, r, 3, rgba(255, 230, 80, 255))
	}
	msg := "Повтори мелодию"
	switch {
	case p.playing:
		msg = "Слушай..."
	case p.done && p.result == 1:
		msg = "Мелодия сыграна!"
	case p.done:
		msg = "Не та мелодия"
	}
	adapters.DrawText(screen, msg, 200, 410, rgba(255, 245, 220, 255))
	adapters.DrawText(screen,
		"Круг "+strconv.Itoa(p.round)+"/"+strconv.Itoa(pipeRounds),
		200, 424, rgba(255, 245, 220, 255))
}

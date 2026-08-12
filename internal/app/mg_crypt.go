package app

import (
	"image"
	"math/rand"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/shpaker/modern-robinson/internal/repositories"
)

// The translator puzzle (StartGame 5 -> Translt), rebuilt to the original's
// rules. CRYPT.TXT carries the 30-letter alphabet, its uppercase twin and the
// castaway's message; C1..C30 are the pictograms, R1..R30 the Cyrillic letters,
// S1..S5 the punctuation. The pictogram<->letter permutation is generated at
// runtime — it is not in the data — so every session scrambles differently.
//
// Cell values use the engine's own encoding: 0..n-1 a cipher pictogram,
// n..n+4 punctuation, n+5.. a placed letter, -1 an empty cell.
type cryptGame struct {
	sprites map[string]*ebiten.Image
	n       int   // alphabet size (30)
	perm    []int // letter index -> pictogram index

	pristine []int // the untouched cipher text, row-major 26x10
	working  []int // what is drawn and edited
	used     []bool
	sel      int // letter being carried, -1 = none

	solved  bool
	solvedT float64
}

// The grid and strip geometry, verbatim from the engine: a 26x10 glyph grid
// from (48,84) with a 21x30 pitch, and the letter strip along y 26..46.
const (
	cryptCols   = 26
	cryptRows   = 10
	cryptGridX  = 48
	cryptGridY  = 84
	cryptPitchX = 21
	cryptPitchY = 30
	cryptStripY = 26
	cryptStripH = 21
)

// The toolbar hit rectangles the engine tests.
var (
	cryptEraseBtn = image.Rect(6, 406, 76, 465)
	cryptExitBtn  = image.Rect(550, 406, 633, 465)
)

// newCryptGame loads CRYPT.DAT, scrambles the alphabet and typesets the text.
func newCryptGame(g *Game) minigame {
	raw := g.res.ScreenFile("CRYPT", "CRYPT.TXT")
	if raw == nil {
		return nil
	}
	text := strings.ReplaceAll(repositories.DecodeCP1251(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return nil
	}
	lower := []rune(strings.TrimSpace(lines[0]))
	c := &cryptGame{
		sprites: g.packImages("CRYPT"),
		n:       len(lower),
		sel:     -1,
	}
	if c.n == 0 || c.sprites["CRYPT"] == nil {
		return nil
	}
	c.perm = rand.Perm(c.n)
	c.used = make([]bool, c.n)

	upper := []rune(strings.TrimSpace(lines[1]))
	letterIdx := func(r rune) int {
		for i, l := range lower {
			if r == l || (i < len(upper) && r == upper[i]) {
				return i
			}
		}
		return -1
	}
	// Typeset the message: one text line per row, wrapping at column 26 the
	// way the engine's SetText does (the long line 9 wraps onto row 10).
	c.pristine = make([]int, cryptCols*cryptRows)
	for i := range c.pristine {
		c.pristine[i] = -1
	}
	row := 0
	for _, line := range lines[2:] {
		line = strings.TrimSuffix(line, "~")
		if line == "" || row >= cryptRows {
			continue
		}
		col := 0
		for _, r := range line {
			if col == cryptCols {
				col = 0
				if row++; row >= cryptRows {
					break
				}
			}
			c.pristine[row*cryptCols+col] = c.encode(r, letterIdx(r))
			col++
		}
		row++
	}
	c.working = make([]int, len(c.pristine))
	copy(c.working, c.pristine)
	return c
}

// encode turns a message character into its cell value.
func (c *cryptGame) encode(r rune, letter int) int {
	if letter >= 0 {
		return c.perm[letter] // the pictogram standing for this letter
	}
	switch r {
	case ',':
		return c.n
	case '.':
		return c.n + 1
	case '-':
		return c.n + 2
	case '!':
		return c.n + 3
	default:
		return c.n + 4 // space
	}
}

// stripX is the left edge of the letter strip: centred for n letters.
func (c *cryptGame) stripX() int {
	return (ViewW - cryptPitchX*c.n) / 2
}

// update implements the engine's click logic.
func (c *cryptGame) update(g *Game, dt float64) (bool, int) {
	if c.solved {
		c.solvedT += dt
		if c.solvedT > 3 || clickedThisTick() {
			return true, 1
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
	switch {
	case pointIn(cryptExitBtn, mx, my):
		return true, 0
	case pointIn(cryptEraseBtn, mx, my):
		// The erase button is a full reset, not an undo.
		copy(c.working, c.pristine)
		for i := range c.used {
			c.used[i] = false
		}
		c.sel = -1
		g.playSound([]string{"r_all.wav", "1"})
		return false, 0
	}
	if c.sel < 0 {
		// Take a letter from the strip...
		if my >= cryptStripY && my < cryptStripY+cryptStripH {
			i := (mx - c.stripX()) / cryptPitchX
			if mx >= c.stripX() && i >= 0 && i < c.n && !c.used[i] {
				c.sel = i
				c.used[i] = true
				g.playSound([]string{"r_take.wav", "1"})
				return false, 0
			}
		}
		// ...or pull a placed letter back, reverting all its cells.
		if v, ok := c.cellAt(mx, my); ok && v >= c.n+5 {
			letter := v - c.n - 5
			for i, w := range c.working {
				if w == v {
					c.working[i] = c.pristine[i]
				}
			}
			c.used[letter] = false
			g.playSound([]string{"r_back.wav", "1"})
		}
		return false, 0
	}
	// A letter is in hand: it lands only on a pictogram cell.
	v, ok := c.cellAt(mx, my)
	if ok && v >= 0 && v < c.n {
		placed := c.n + 5 + c.sel
		for i, w := range c.working {
			if w == v {
				c.working[i] = placed
			}
		}
		g.playSound([]string{"r_put.wav", "1"})
		c.sel = -1
		if c.check() {
			c.solved = true
			g.playSound([]string{"final5.wav", "1"})
		}
		return false, 0
	}
	// Dropped anywhere else: the letter goes back to the strip.
	c.used[c.sel] = false
	c.sel = -1
	g.playSound([]string{"r_error.wav", "1"})
	return false, 0
}

// cellAt maps a point to its grid cell value.
func (c *cryptGame) cellAt(mx, my int) (int, bool) {
	if mx < cryptGridX || mx >= cryptGridX+cryptCols*cryptPitchX ||
		my < cryptGridY || my >= cryptGridY+cryptRows*cryptPitchY {
		return 0, false
	}
	col := (mx - cryptGridX) / cryptPitchX
	row := (my - cryptGridY) / cryptPitchY
	return c.working[row*cryptCols+col], true
}

// check is the win test: no pictogram left, and every placed letter is the one
// the permutation says its pristine pictogram stands for.
func (c *cryptGame) check() bool {
	for i, w := range c.working {
		switch {
		case w < 0:
		case w < c.n:
			return false // still enciphered
		case w < c.n+5:
		default:
			if c.perm[w-c.n-5] != c.pristine[i] {
				return false // guessed wrong
			}
		}
	}
	return true
}

// glyphSprite names the sprite for a cell value.
func (c *cryptGame) glyphSprite(v int) string {
	switch {
	case v < 0:
		return ""
	case v < c.n:
		return "C" + strconv.Itoa(v+1)
	case v < c.n+4:
		return "S" + strconv.Itoa(v-c.n+1)
	case v == c.n+4:
		return "" // space (S5 is fully transparent)
	default:
		return "R" + strconv.Itoa(v-c.n-5+1)
	}
}

// draw paints the parchment, the text grid, the letter strip and the letter in
// hand following the cursor.
func (c *cryptGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, c.sprites["CRYPT"], 0, 0)
	for row := 0; row < cryptRows; row++ {
		for col := 0; col < cryptCols; col++ {
			if s := c.glyphSprite(c.working[row*cryptCols+col]); s != "" {
				blitAt(screen, c.sprites[s],
					cryptGridX+col*cryptPitchX, cryptGridY+row*cryptPitchY)
			}
		}
	}
	for i := 0; i < c.n; i++ {
		if !c.used[i] {
			blitAt(screen, c.sprites["R"+strconv.Itoa(i+1)],
				c.stripX()+i*cryptPitchX, cryptStripY)
		}
	}
	if c.sel >= 0 {
		mx, my := ebiten.CursorPosition()
		blitAt(screen, c.sprites["R"+strconv.Itoa(c.sel+1)], mx-10, my-9)
	}
}

package app

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shpaker/modern-robinson/internal/adapters"
	"github.com/shpaker/modern-robinson/internal/repositories"
)

// The translator puzzle (StartGame 5 -> Translt). CRYPT.DAT holds the whole
// game: CRYPT.TXT is the 30-letter alphabet, its uppercase twin and the
// castaway's message; C1..C30 are the pictograms of that alien alphabet, R1..R30
// the Cyrillic letters they stand for, and S1..S5 the punctuation (, . - ! and
// a blank for space). The message is written in pictograms and the player has to
// work out which letter each one is.
type cryptGame struct {
	sprites map[string]*ebiten.Image
	lower   []rune // alphabet, index -> lower-case letter
	upper   []rune
	lines   []string // the message, one screen line each

	guess    map[int]int // pictogram index -> guessed letter index (1-based)
	selected int         // pictogram currently being guessed, 0 = none
	solved   bool
	solvedT  float64
}

// Layout: the message sits in the upper parchment, the letter palette below it,
// and the authored bar (erase / exit) occupies the bottom 80 px.
const (
	cryptCellW  = 20
	cryptCellH  = 19
	cryptLineH  = 26
	cryptTextX  = 60
	cryptTextY  = 40
	cryptPalX   = 60
	cryptPalY   = 300
	cryptPalCol = 15
)

var (
	cryptEraseBtn = image.Rect(5, 405, 74, 471)
	cryptExitBtn  = image.Rect(564, 405, 633, 471)
)

// newCryptGame loads CRYPT.DAT and prepares the puzzle.
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
	c := &cryptGame{
		sprites: g.packImages("CRYPT"),
		lower:   []rune(lines[0]),
		upper:   []rune(lines[1]),
		guess:   map[int]int{},
	}
	for _, l := range lines[2:] {
		l = strings.TrimSuffix(l, "~")
		if l != "" {
			c.lines = append(c.lines, l)
		}
	}
	if len(c.lower) == 0 || len(c.lines) == 0 {
		return nil
	}
	return c
}

// letterIndex maps a message character to its 1-based alphabet index, or 0 when
// it is punctuation (see punctSprite).
func (c *cryptGame) letterIndex(r rune) int {
	for i, l := range c.lower {
		if r == l || (i < len(c.upper) && r == c.upper[i]) {
			return i + 1
		}
	}
	return 0
}

// punctSprite maps the five non-letter characters to their S sprites.
func punctSprite(r rune) string {
	switch r {
	case ',':
		return "S1"
	case '.':
		return "S2"
	case '-':
		return "S3"
	case '!':
		return "S4"
	case ' ':
		return "S5"
	}
	return ""
}

// update handles clicks: pick a pictogram, assign a letter, erase, or give up.
func (c *cryptGame) update(g *Game, dt float64) (bool, int) {
	if c.solved {
		c.solvedT += dt
		return c.solvedT > 2.5, 1
	}
	if !clickedThisTick() {
		return false, 0
	}
	mx, my := ebiten.CursorPosition()
	switch {
	case pointIn(cryptExitBtn, mx, my):
		return true, 0 // gave up: Translt stays 0, the scene offers a retry
	case pointIn(cryptEraseBtn, mx, my):
		if c.selected > 0 {
			delete(c.guess, c.selected)
		}
		c.selected = 0
		return false, 0
	}
	if i := c.pictogramAt(mx, my); i > 0 {
		c.selected = i
		return false, 0
	}
	if l := c.letterAt(mx, my); l > 0 && c.selected > 0 {
		c.guess[c.selected] = l
		c.selected = 0
		g.playSound([]string{"r_put.wav", "1"})
		if c.check() {
			c.solved = true
			g.playSound([]string{"final5.wav", "1"})
		}
	}
	return false, 0
}

// pictogramAt returns the alphabet index of the message glyph under the cursor.
func (c *cryptGame) pictogramAt(mx, my int) int {
	for row, line := range c.lines {
		y := cryptTextY + row*cryptLineH
		if my < y || my >= y+cryptCellH {
			continue
		}
		col := (mx - cryptTextX) / cryptCellW
		runes := []rune(line)
		if col < 0 || col >= len(runes) {
			return 0
		}
		return c.letterIndex(runes[col])
	}
	return 0
}

// letterAt returns the alphabet index of the palette letter under the cursor.
func (c *cryptGame) letterAt(mx, my int) int {
	col := (mx - cryptPalX) / cryptCellW
	row := (my - cryptPalY) / cryptLineH
	if col < 0 || col >= cryptPalCol || row < 0 {
		return 0
	}
	i := row*cryptPalCol + col + 1
	if i > len(c.lower) {
		return 0
	}
	if my >= cryptPalY+row*cryptLineH+cryptCellH {
		return 0
	}
	return i
}

// check reports whether every pictogram used in the message is now guessed
// correctly — the puzzle's win condition.
func (c *cryptGame) check() bool {
	for _, line := range c.lines {
		for _, r := range line {
			i := c.letterIndex(r)
			if i == 0 {
				continue
			}
			if c.guess[i] != i {
				return false
			}
		}
	}
	return true
}

// draw paints the parchment, the message (pictograms turning into letters as
// they are guessed), the letter palette and the selection.
func (c *cryptGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, c.sprites["CRYPT"], 0, 0)
	for row, line := range c.lines {
		y := cryptTextY + row*cryptLineH
		for col, r := range []rune(line) {
			x := cryptTextX + col*cryptCellW
			if s := punctSprite(r); s != "" {
				blitAt(screen, c.sprites[s], x, y)
				continue
			}
			i := c.letterIndex(r)
			if i == 0 {
				continue
			}
			name := "C" + strconv.Itoa(i)
			if c.guess[i] == i {
				name = "R" + strconv.Itoa(i) // solved: show the real letter
			}
			blitAt(screen, c.sprites[name], x, y)
			if i == c.selected {
				strokeRect(
					screen,
					image.Rect(x-1, y-1, x+cryptCellW, y+cryptCellH),
					2,
					rgba(200, 40, 30, 255),
				)
			}
		}
	}
	// The palette of Cyrillic letters to assign.
	for i := 1; i <= len(c.lower); i++ {
		x := cryptPalX + ((i-1)%cryptPalCol)*cryptCellW
		y := cryptPalY + ((i-1)/cryptPalCol)*cryptLineH
		blitAt(screen, c.sprites["R"+strconv.Itoa(i)], x, y)
	}
	hint := "Выбери значок, затем букву"
	if c.selected > 0 {
		hint = "Какая это буква?"
	}
	if c.solved {
		hint = "Послание разгадано!"
	}
	adapters.DrawText(screen, hint, 96, 420, rgba(40, 30, 20, 255))
}

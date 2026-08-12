package app

import (
	"image"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Draughts against the sailor (StartGame 2 -> Dames), on the engine's 6x6
// board. Cell values follow the original: 1 red man (the player, moving up),
// 2 green man, 3 and 4 their kings. Captures are compulsory and kings fly, as
// in the Russian rules the engine implements. The original opponent's search
// could not be recovered from the DLL, so this one plays a straightforward
// greedy game: it takes the longest capture it sees, otherwise a random move.
type chessGame struct {
	sprites map[string]*ebiten.Image
	board   [36]int
	sel     int // selected cell, -1
	chain   bool
	aiWait  float64
	won     bool
	lost    float64 // restart countdown after a loss
	finishT float64
}

const (
	chessBoardX = 364
	chessBoardY = 71
	chessCell   = 43
)

var chessBoard = image.Rect(364, 71, 621, 328)

func newChessGame(g *Game) minigame {
	c := &chessGame{sel: -1}
	c.sprites = g.packImages("CHESS")
	if c.sprites["BACK"] == nil {
		return nil
	}
	c.reset()
	return c
}

// reset lays out the six men a side, exactly as the engine's NewGame does.
func (c *chessGame) reset() {
	c.board = [36]int{}
	for _, i := range []int{1, 3, 5, 6, 8, 10} {
		c.board[i] = 2
	}
	for _, i := range []int{25, 27, 29, 30, 32, 34} {
		c.board[i] = 1
	}
	c.sel = -1
	c.chain = false
}

func red(v int) bool   { return v == 1 || v == 3 }
func green(v int) bool { return v == 2 || v == 4 }
func king(v int) bool  { return v == 3 || v == 4 }

type cmove struct {
	from, to int
	takes    int // captured cell, -1 for a plain move
}

// moves lists a piece's legal moves; capturesOnly narrows to jumps.
func (c *chessGame) moves(i int, capturesOnly bool) []cmove {
	v := c.board[i]
	if v == 0 {
		return nil
	}
	r, col := i/6, i%6
	mine, theirs := red, green
	if green(v) {
		mine, theirs = green, red
	}
	var out []cmove
	dirs := [4][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
	for _, d := range dirs {
		if !king(v) {
			// A man steps one square forward, but captures either way.
			tr, tc := r+d[0], col+d[1]
			if tr >= 0 && tr < 6 && tc >= 0 && tc < 6 {
				t := tr*6 + tc
				fwd := (red(v) && d[0] == -1) || (green(v) && d[0] == 1)
				if c.board[t] == 0 && fwd && !capturesOnly {
					out = append(out, cmove{i, t, -1})
				}
				jr, jc := tr+d[0], tc+d[1]
				if theirs(c.board[t]) && jr >= 0 && jr < 6 &&
					jc >= 0 && jc < 6 && c.board[jr*6+jc] == 0 {
					out = append(out, cmove{i, jr*6 + jc, t})
				}
			}
			continue
		}
		// A king slides; the first enemy on the diagonal can be jumped to
		// any empty square behind it.
		enemy := -1
		for s := 1; ; s++ {
			tr, tc := r+d[0]*s, col+d[1]*s
			if tr < 0 || tr >= 6 || tc < 0 || tc >= 6 {
				break
			}
			t := tr*6 + tc
			switch {
			case c.board[t] == 0 && enemy < 0:
				if !capturesOnly {
					out = append(out, cmove{i, t, -1})
				}
			case c.board[t] == 0:
				out = append(out, cmove{i, t, enemy})
			case theirs(c.board[t]) && enemy < 0:
				enemy = t
			default:
				s = 100 // own piece or second enemy: stop
			}
			if s == 100 {
				break
			}
		}
	}
	_ = mine
	return out
}

// sideMoves lists a side's moves, applying the compulsory-capture rule.
func (c *chessGame) sideMoves(isRed bool) []cmove {
	var plain, jumps []cmove
	for i, v := range c.board {
		if v == 0 || red(v) != isRed {
			continue
		}
		for _, m := range c.moves(i, false) {
			if m.takes >= 0 {
				jumps = append(jumps, m)
			} else {
				plain = append(plain, m)
			}
		}
	}
	if len(jumps) > 0 {
		return jumps
	}
	return plain
}

// apply plays a move; returns true when the same piece must keep capturing.
func (c *chessGame) apply(g *Game, m cmove) bool {
	v := c.board[m.from]
	c.board[m.from] = 0
	c.board[m.to] = v
	if m.takes >= 0 {
		c.board[m.takes] = 0
		g.playSound([]string{"eat.wav", "1"})
	} else if king(v) {
		g.playSound([]string{"movelady.wav", "1"})
	} else {
		g.playSound([]string{"move.wav", "1"})
	}
	// Promotion on the far row.
	if v == 1 && m.to/6 == 0 {
		c.board[m.to] = 3
		g.playSound([]string{"lady.wav", "1"})
	}
	if v == 2 && m.to/6 == 5 {
		c.board[m.to] = 4
		g.playSound([]string{"lady.wav", "1"})
	}
	if m.takes < 0 {
		return false
	}
	// After a capture the same piece keeps jumping while more jumps exist.
	return len(c.moves(m.to, true)) > 0
}

// update runs the player's clicks and the opponent's replies.
func (c *chessGame) update(g *Game, dt float64) (bool, int) {
	if c.won {
		c.finishT += dt
		return c.finishT > 3 || clickedThisTick(), 1
	}
	if c.lost > 0 {
		if c.lost -= dt; c.lost <= 0 {
			c.reset() // the original restarts the board after a loss
		}
		return false, 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true, 0
	}
	if c.aiWait > 0 {
		if c.aiWait -= dt; c.aiWait > 0 {
			return false, 0
		}
		c.aiTurn(g)
		return false, 0
	}
	if !clickedThisTick() {
		return false, 0
	}
	mx, my := ebiten.CursorPosition()
	if !pointIn(chessBoard, mx, my) {
		if pointIn(houseExit, mx, my) {
			return true, 0
		}
		return false, 0
	}
	cell := ((my-chessBoardY)/chessCell)*6 + (mx-chessBoardX)/chessCell
	legal := c.sideMoves(true)
	if c.chain {
		// Mid-capture: only the chaining piece's further jumps are legal.
		legal = c.moves(c.sel, true)
	}
	// Selecting one of my pieces (only ones that may move).
	if red(c.board[cell]) && !c.chain {
		for _, m := range legal {
			if m.from == cell {
				c.sel = cell
				return false, 0
			}
		}
		return false, 0
	}
	if c.sel < 0 {
		return false, 0
	}
	for _, m := range legal {
		if m.from != c.sel || m.to != cell {
			continue
		}
		if c.apply(g, m) {
			c.sel, c.chain = m.to, true // must keep capturing
			return false, 0
		}
		c.sel, c.chain = -1, false
		if c.gameOver(g) {
			return false, 0
		}
		c.aiWait = 0.7
		return false, 0
	}
	return false, 0
}

// aiTurn plays the opponent: the capture chain when one exists, else random.
func (c *chessGame) aiTurn(g *Game) {
	ms := c.sideMoves(false)
	if len(ms) == 0 {
		c.won = true // the sailor cannot move: the player wins
		g.playSound([]string{"final1.wav", "1"})
		return
	}
	m := ms[rand.Intn(len(ms))]
	for c.apply(g, m) {
		next := c.moves(m.to, true)
		m = next[rand.Intn(len(next))]
	}
	c.gameOver(g)
}

// gameOver checks both sides; a player defeat restarts the board.
func (c *chessGame) gameOver(g *Game) bool {
	if len(c.sideMoves(false)) == 0 {
		c.won = true
		g.playSound([]string{"final1.wav", "1"})
		return true
	}
	if len(c.sideMoves(true)) == 0 {
		c.lost = 2
		return true
	}
	return false
}

// draw paints the cabin, the pots and bottles, and the selection frame.
func (c *chessGame) draw(_ *Game, screen *ebiten.Image) {
	blitAt(screen, c.sprites["BACK"], 0, 0)
	names := map[int]string{1: "R1", 2: "G1", 3: "R2", 4: "G2"}
	for i, v := range c.board {
		if v == 0 {
			continue
		}
		r, col := i/6, i%6
		blitAt(screen, c.sprites[names[v]],
			369+chessCell*col, 40+chessCell*r)
	}
	if c.sel >= 0 {
		r, col := c.sel/6, c.sel%6
		blitAt(screen, c.sprites["ACCENT"],
			361+chessCell*col, 68+chessCell*r)
	}
}

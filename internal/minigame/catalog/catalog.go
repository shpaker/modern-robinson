// Package catalog lists the six minigames by the id StartGame passes, the way
// MINIGAME.DLL's dispatch table does (six entries at 0x1002A398).
package catalog

import (
	"github.com/shpaker/modern-robinson/internal/minigame"
	"github.com/shpaker/modern-robinson/internal/minigame/baloon"
	"github.com/shpaker/modern-robinson/internal/minigame/chess"
	"github.com/shpaker/modern-robinson/internal/minigame/crypt"
	"github.com/shpaker/modern-robinson/internal/minigame/house"
	"github.com/shpaker/modern-robinson/internal/minigame/mappuzzle"
	"github.com/shpaker/modern-robinson/internal/minigame/pipe"
)

// Entry is one minigame.
type Entry struct {
	Name string // what it is, for menus and logs
	Pack string // its top-level container, <Pack>.DAT
	// Param is the value of the game's quest variable that the original can
	// be won with: all twelve chart fragments (MapParts), every organ pipe
	// (the Tubs mask 7), the full alphabet (Find6). Of the remakes only the
	// organ reads it; the rest take what they need from their packs.
	Param int
	New   minigame.New
}

// Games is indexed by StartGame's gameId.
var Games = [...]Entry{
	{Name: "Карта-пазл", Pack: "MAP", Param: 12, New: mappuzzle.New},
	{Name: "Хижина", Pack: "HOUSE", New: house.New},
	{Name: "Шашки", Pack: "CHESS", New: chess.New},
	{Name: "Воздушный шар", Pack: "BALOON", New: baloon.New},
	{Name: "Орган", Pack: "PIPE", Param: 7, New: pipe.New},
	{Name: "Переводчик", Pack: "CRYPT", Param: 30, New: crypt.New},
}

// Get returns the game with that id; ok is false for an id the table does not
// have.
func Get(id int) (Entry, bool) {
	if id < 0 || id >= len(Games) {
		return Entry{}, false
	}
	return Games[id], true
}

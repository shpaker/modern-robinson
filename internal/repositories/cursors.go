package repositories

import (
	"github.com/shpaker/modern-robinson/internal/repositories/codec"
	"github.com/shpaker/modern-robinson/internal/types"
)

// Cursors returns the game's mouse cursors, which ship as Win32 resources of
// ROBY.EXE rather than in a pack (Bar::Init 0x402f50 loads them by name):
// one per inventory item, ADV1..ADV4 for the exit arrows and 247 for the
// waiting clock. Without the executable the map is empty.
func (r *Resources) Cursors() map[string]types.Cursor {
	d, err := r.readFileUpper("ROBY.EXE")
	if err != nil {
		return map[string]types.Cursor{}
	}
	c, err := codec.ExeCursors(d)
	if err != nil {
		return map[string]types.Cursor{}
	}
	return c
}

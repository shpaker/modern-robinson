package app

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/shpaker/modern-robinson/internal/repositories"
	"github.com/shpaker/modern-robinson/internal/testutil"
	"github.com/shpaker/modern-robinson/internal/types"
)

// memSaves keeps the slots in memory.
type memSaves map[string][]byte

func (m memSaves) Read(name string) ([]byte, error) {
	if b, ok := m[name]; ok {
		return b, nil
	}
	return nil, errors.New("no such slot")
}

func (m memSaves) Write(name string, data []byte) error {
	m[name] = data
	return nil
}

// saveGame builds a fresh session on the real resources, the way a player
// starts one before loading a slot: a new state and nobody on stage yet.
func saveGame(t *testing.T, saves memSaves) *Game {
	t.Helper()
	res := repositories.NewResources(testutil.GameRoot(t))
	g := &Game{
		res:        res,
		parser:     repositories.SceneParser{},
		gs:         types.NewGameState(),
		audio:      &fakeAudio{},
		cycleCache: map[string]*walkCycle{},
		saves:      saves,
		fridHidden: true,
	}
	g.loadCharacter()
	return g
}

// writeSlot stores sd in slot 0.
func writeSlot(t *testing.T, saves memSaves, sd types.SaveData) {
	t.Helper()
	b, err := json.Marshal(sd)
	if err != nil {
		t.Fatal(err)
	}
	saves[slotName(0)] = b
}

// A party saved together comes back together: Friday keeps her cell, her z
// and her visibility, as the engine's save does (0x41fba0). She used to come
// back hidden, since a load runs no entry script and resetRun hides her.
func TestLoadSlotKeepsFriday(t *testing.T) {
	saves := memSaves{}
	g := saveGame(t, saves)
	g.loadScene("SCENA0", &[2]int{2, 3}, "", "")
	g.gs.SetVar("FridIs", 1)
	g.fridHidden, g.fridCell, g.fridZ = false, [2]int{3, 2}, 4
	writeSlot(t, saves, g.snapshot())

	g = saveGame(t, saves)
	if !g.loadSlot(0) {
		t.Fatal("the slot must load")
	}
	if !g.fridVisible() {
		t.Fatal("Friday must be on stage after the load")
	}
	if g.fridCell != [2]int{3, 2} || g.fridZ != 4 {
		t.Errorf("Friday at %v z=%d, want (3,2) z=4", g.fridCell, g.fridZ)
	}
	x, y := g.grid.ToScreen(3, 2)
	if g.fridPos != [2]float64{float64(x), float64(y)} {
		t.Errorf("fridPos = %v, want her cell's anchor (%d,%d)",
			g.fridPos, x, y)
	}
}

// Before she joins she stays hidden, whatever the save says of her cell.
func TestLoadSlotKeepsFridayHidden(t *testing.T) {
	saves := memSaves{}
	g := saveGame(t, saves)
	g.loadScene("SCENA0", &[2]int{2, 3}, "", "")
	writeSlot(t, saves, g.snapshot())

	g = saveGame(t, saves)
	g.fridHidden = false
	if !g.loadSlot(0) {
		t.Fatal("the slot must load")
	}
	if !g.fridHidden || g.fridVisible() {
		t.Error("Friday must stay hidden: she has not joined yet")
	}
}

// Saves written before Friday was kept know only FridIs. With her joined she
// comes back beside the hero on a walkable cell; without, she stays hidden.
func TestLoadOldSlotPlacesFridayBesideHero(t *testing.T) {
	for _, joined := range []bool{true, false} {
		saves := memSaves{}
		st := types.NewGameState()
		if joined {
			st.SetVar("FridIs", 1)
		}
		writeSlot(t, saves, st.Snapshot("SCENA0", [2]int{2, 3}))

		g := saveGame(t, saves)
		if !g.loadSlot(0) {
			t.Fatal("the slot must load")
		}
		if !joined {
			if g.fridVisible() {
				t.Error("not joined: Friday must stay hidden")
			}
			continue
		}
		if !g.fridVisible() {
			t.Fatal("joined: Friday must be on stage after the load")
		}
		c := g.fridCell
		if !g.grid.Valid(c[0], c[1]) {
			t.Errorf("Friday on %v, want a walkable cell", c)
		}
		if d := abs(c[0]-2) + abs(c[1]-3); d != 1 {
			t.Errorf("Friday on %v, want a neighbour of the hero's (2,3)", c)
		}
	}
}

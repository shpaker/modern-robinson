package types_test

import (
	"testing"

	"github.com/shpaker/modern-robinson/internal/types"
)

func TestVarsCaseInsensitive(t *testing.T) {
	st := types.NewGameState()
	st.SetVar("Findaxe", 1)
	if st.Var("findaxe") != 1 || st.Var("FINDAXE") != 1 {
		t.Fatal("var lookup must be case-insensitive")
	}
	st.AddVar("Coins", 2)
	st.AddVar("coins", 3)
	if st.Var("COINS") != 5 {
		t.Fatalf("coins=%d, want 5", st.Var("COINS"))
	}
}

func TestInventoryDedupeAndRemove(t *testing.T) {
	st := types.NewGameState()
	st.AddItem("axe")
	st.AddItem("Axe") // duplicate (case-insensitive)
	st.AddItem("rope")
	if len(st.Inventory) != 2 {
		t.Fatalf("inventory=%v, want 2 unique", st.Inventory)
	}
	st.DelItem("AXE")
	if st.HasItem("axe") {
		t.Fatal("axe should be removed")
	}
	if !st.HasItem("rope") {
		t.Fatal("rope should remain")
	}
}

func TestWorldGonePersistsPerScene(t *testing.T) {
	st := types.NewGameState()
	st.MarkGone("SCENA0", "axe")
	if !st.IsGone("scena0", "AXE") {
		t.Fatal("gone must persist case-insensitively")
	}
	if st.IsGone("SCENA1", "axe") {
		t.Fatal("removal must be scoped to its scene")
	}
}

func TestWorldSpawnAndCancelGone(t *testing.T) {
	st := types.NewGameState()
	st.MarkGone("CAVE", "torch")
	st.MarkSpawn("CAVE", "torch", 3, 4) // recreating cancels the removal
	if st.IsGone("CAVE", "torch") {
		t.Fatal("spawn must cancel a prior removal")
	}
	sp := st.Spawns("cave")
	if len(sp) != 1 || sp[0].Obj != "torch" || sp[0].GX != 3 || sp[0].GY != 4 {
		t.Fatalf("spawns=%v", sp)
	}
	// Re-spawning the same object updates its cell, not appends.
	st.MarkSpawn("CAVE", "torch", 5, 6)
	if sp := st.Spawns("CAVE"); len(sp) != 1 || sp[0].GX != 5 {
		t.Fatalf("spawns=%v, want single updated", sp)
	}
}

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

// Deleting the item in hand puts the hand back in it: the crab released into
// the pool (DeleteItem crb) must not leave the empty hat acting as if full.
func TestDelItemInHandSelectsHand(t *testing.T) {
	st := types.NewGameState()
	st.AddItem("hand")
	st.AddItem("hat")
	st.AddItem("crb")
	st.Active = "crb"
	st.DelItem("hat")
	if st.Active != "crb" {
		t.Fatalf("deleting another item moved the hand: %q", st.Active)
	}
	st.DelItem("CRB")
	if st.Active != "hand" {
		t.Fatalf("Active=%q after deleting it, want hand", st.Active)
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

// A removal cancels the spawn the same way a spawn cancels a removal: the crab
// created back on the beach and caught again must not come back on the next
// visit while it sits in the hat.
func TestWorldGoneCancelsSpawn(t *testing.T) {
	st := types.NewGameState()
	st.MarkSpawn("SCENA0", "crb", 4, 3)
	st.MarkSpawn("SCENA0", "pool", 5, 2)
	st.MarkGone("scena0", "CRB")
	if _, ok := st.SpawnAt("SCENA0", "crb"); ok {
		t.Fatal("a removed object must lose its spawn")
	}
	if sp, ok := st.SpawnAt("SCENA0", "pool"); !ok || sp.GX != 5 {
		t.Fatalf("other spawns must stay: %v %v", sp, ok)
	}
	st.MarkSpawn("SCENA0", "crb", 4, 3)
	if sp, ok := st.SpawnAt("scena0", "CRB"); !ok || sp.GY != 3 ||
		st.IsGone("SCENA0", "crb") {
		t.Fatalf("respawn: %v %v gone=%v", sp, ok,
			st.IsGone("SCENA0", "crb"))
	}
}

// Saves written before the fix may carry an object both removed and spawned;
// the removal is the newer one (a spawn would have cleared it), so loading
// drops the spawn.
func TestRestoreDropsSpawnsOfGoneObjects(t *testing.T) {
	sd := types.SaveData{
		Gone: map[string][]string{"scena0": {"crb"}},
		Spawned: map[string][]types.Spawn{
			"scena0": {{Obj: "crb", GX: 4, GY: 3}, {Obj: "fire", GX: 1, GY: 3}},
		},
	}
	st := types.Restore(sd)
	if _, ok := st.SpawnAt("SCENA0", "crb"); ok || !st.IsGone("SCENA0", "crb") {
		t.Fatal("a stale spawn of a removed object must be dropped")
	}
	if _, ok := st.SpawnAt("SCENA0", "fire"); !ok {
		t.Fatal("a live spawn must survive the load")
	}
}

// A save taken mid-script may carry the mouse switched off; the engine forces
// it back on at the end of every load (0x4213cf), so a restored game can never
// come back deaf to clicks.
func TestRestoreForcesTheMouseOn(t *testing.T) {
	st := types.NewGameState()
	st.UI["mouse"] = false
	sd := st.Snapshot("SCENA0", [2]int{1, 2})
	if got := types.Restore(sd); !got.UI["mouse"] {
		t.Error("a restored game must have the mouse on")
	}
	// Saves from builds that predate the flag carry no "mouse" key at all.
	sd.UI = map[string]bool{"bar": true}
	if got := types.Restore(sd); !got.UI["mouse"] {
		t.Error("an old save without the key must not come back deaf")
	}
}

// InvRev ticks on every real change to the bar's list, so the bar can scroll
// back to its first slot the way the engine's rebuild does; a no-op leaves it.
func TestInvRevCountsInventoryChanges(t *testing.T) {
	st := types.NewGameState()
	st.AddItem("hand")
	st.AddItem("hat")
	rev := st.InvRev
	st.AddItem("HAT")   // already held
	st.DelItem("stick") // not held
	if st.InvRev != rev {
		t.Fatalf("no-ops moved InvRev %d -> %d", rev, st.InvRev)
	}
	st.DelItem("hat")
	st.AddItem("crb")
	if st.InvRev != rev+2 {
		t.Fatalf("InvRev = %d, want %d", st.InvRev, rev+2)
	}
}

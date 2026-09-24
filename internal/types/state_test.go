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
	if len(st.Inventory()) != 2 {
		t.Fatalf("inventory=%v, want 2 unique", st.Inventory())
	}
	st.DelItem("AXE")
	if st.HasItem("axe") {
		t.Fatal("axe should be removed")
	}
	if !st.HasItem("rope") {
		t.Fatal("rope should remain")
	}
}

// The hand is a slot of the bar (DeleteItem 0x405190). Deleting the item in
// it goes back to the first slot: the crab released into the pool must not
// leave the empty hat acting as if full. Deleting one to the right leaves the
// hand alone; one to the left keeps the slot, so the next item slides in, and
// a slot left past the end falls back to the first.
func TestDelItemKeepsTheSelectedSlot(t *testing.T) {
	st := types.NewGameState()
	for _, it := range []string{"hand", "hat", "crb", "axe", "rope"} {
		st.AddItem(it)
	}
	st.Active = "crb"
	st.DelItem("rope")
	if st.Active != "crb" {
		t.Fatalf("deleting to the right moved the hand: %q", st.Active)
	}
	st.DelItem("hat")
	if st.Active != "axe" {
		t.Fatalf("Active=%q, want axe slid into the slot", st.Active)
	}
	st.DelItem("CRB")
	if st.Active != "hand" {
		t.Fatalf("Active=%q past the end, want the first slot", st.Active)
	}
	st.Active = "axe"
	st.DelItem("axe")
	if st.Active != "hand" {
		t.Fatalf("Active=%q after deleting it, want hand", st.Active)
	}
}

// Each character has his own list and the bar shows the controlled one's.
// Handing over control keeps the bar's selected slot (0x404877): the hero's
// second slot becomes Friday's second, her first when she has fewer.
func TestItemsArePerCharacter(t *testing.T) {
	st := types.NewGameState()
	st.AddItemTo("Roby", "hand")
	st.AddItemTo("Roby", "hat")
	st.AddItemTo("Roby", "axe")
	st.AddItemTo("Frid", "handfr")
	st.Active = "hat"
	st.AddItemTo("frid", "confr") // AddItem Frid, confr
	if st.HasItem("confr") || st.Active != "hat" {
		t.Fatal("Friday's condom must not reach the hero's bar")
	}
	st.SetActiveChar("Frid")
	if st.Active != "confr" || len(st.Inventory()) != 2 {
		t.Fatalf("Friday holds %q of %v, want slot 1 = confr",
			st.Active, st.Inventory())
	}
	st.DelItem("confr") // FRCONFIR: her own script, her own list
	if st.Active != "handfr" || len(st.InventoryOf("Roby")) != 3 {
		t.Fatalf("Active=%q, Roby=%v", st.Active, st.InventoryOf("Roby"))
	}
	st.Active = "handfr"
	st.SetActiveChar("Roby")
	if st.Active != "hand" {
		t.Fatalf("back to the hero with %q, want hand", st.Active)
	}
	st.Active = "axe"
	st.SetActiveChar("Frid")
	if st.Active != "handfr" {
		t.Fatalf("slot 2 past Friday's list gave %q, want handfr", st.Active)
	}
}

// A save from before the split holds one shared list: Friday's own items go
// back to her, behind her bare hand.
func TestRestoreSplitsASharedInventory(t *testing.T) {
	st := types.Restore(types.SaveData{
		Inventory: []string{"hand", "hat", "confr", "axe"},
		Active:    "axe",
	})
	roby, frid := st.InventoryOf("Roby"), st.InventoryOf("Frid")
	if len(roby) != 3 || roby[2] != "axe" || len(frid) != 2 ||
		frid[0] != "handfr" || frid[1] != "confr" {
		t.Fatalf("roby=%v frid=%v", roby, frid)
	}
	if st.ActiveChar != "Roby" || st.Active != "axe" {
		t.Fatalf("%s holds %q", st.ActiveChar, st.Active)
	}
	back := types.Restore(st.Snapshot("SCENA0", [2]int{}))
	if len(back.InventoryOf("Frid")) != 2 {
		t.Fatalf("round trip lost Friday's list: %v", back.Items)
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

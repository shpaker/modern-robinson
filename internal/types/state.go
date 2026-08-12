package types

import "strings"

// Spawn is an object created at runtime (CreateObject) that must reappear each
// time its scene is entered.
type Spawn struct {
	Obj    string
	GX, GY int
}

// GameState is the mutable quest state: flag variables, string (dialogue)
// variables, the inventory, the active tool/character, and the persistent
// world edits (objects taken or created per scene). Variable and item names
// are case-insensitive (the NGI scripts are), so keys are folded to lower case.
// It carries no engine or resource dependencies.
type GameState struct {
	Vars      map[string]int    // SetVar/AddVar quest flags and counters
	CharVars  map[string]string // SetCharVar dialogue-variant selectors
	Inventory []string          // AddItem/DeleteItem, ordered for the bar
	Active    string            // SetActive: current tool or character

	gone    map[string]map[string]bool // scene -> object -> removed (DelObject)
	spawned map[string][]Spawn         // scene -> objects added (CreateObject)

	UI map[string]bool // SetMouse/SetMap/LockBar/SetBar/ShowCursor/Interrupt toggles
}

// NewGameState returns an empty state with initialised maps. UI toggles that
// default on: mouse input, the bar, and the cursor; the island map opens later
// (SetMap ON).
func NewGameState() *GameState {
	return &GameState{
		Vars:     map[string]int{},
		CharVars: map[string]string{},
		gone:     map[string]map[string]bool{},
		spawned:  map[string][]Spawn{},
		UI:       map[string]bool{"mouse": true, "bar": true, "cursor": true},
	}
}

// Var returns the flag's value, or 0 if it was never set.
func (g *GameState) Var(
	name string,
) int {
	return g.Vars[strings.ToLower(name)]
}

// SetVar assigns a flag.
func (g *GameState) SetVar(
	name string,
	v int,
) {
	g.Vars[strings.ToLower(name)] = v
}

// AddVar increments a counter by d (SetVar with 0 first if unset).
func (g *GameState) AddVar(
	name string,
	d int,
) {
	g.Vars[strings.ToLower(name)] += d
}

// CharVar returns the dialogue-variant selector, or "" if unset.
func (g *GameState) CharVar(
	name string,
) string {
	return g.CharVars[strings.ToLower(name)]
}

// SetCharVar assigns a dialogue-variant selector.
func (g *GameState) SetCharVar(
	name, v string,
) {
	g.CharVars[strings.ToLower(name)] = v
}

// HasItem reports whether the item is in the inventory.
func (g *GameState) HasItem(item string) bool {
	item = strings.ToLower(item)
	for _, it := range g.Inventory {
		if strings.ToLower(it) == item {
			return true
		}
	}
	return false
}

// AddItem appends an item, ignoring duplicates.
func (g *GameState) AddItem(item string) {
	if item == "" || g.HasItem(item) {
		return
	}
	g.Inventory = append(g.Inventory, item)
}

// DelItem removes an item if present.
func (g *GameState) DelItem(item string) {
	item = strings.ToLower(item)
	for i, it := range g.Inventory {
		if strings.ToLower(it) == item {
			g.Inventory = append(g.Inventory[:i], g.Inventory[i+1:]...)
			return
		}
	}
}

// MarkGone records that an object was removed from a scene, so it stays gone
// across revisits.
func (g *GameState) MarkGone(scene, obj string) {
	scene, obj = strings.ToLower(scene), strings.ToLower(obj)
	if g.gone[scene] == nil {
		g.gone[scene] = map[string]bool{}
	}
	g.gone[scene][obj] = true
}

// IsGone reports whether an object was removed from a scene.
func (g *GameState) IsGone(scene, obj string) bool {
	return g.gone[strings.ToLower(scene)][strings.ToLower(obj)]
}

// MarkSpawn records an object created in a scene, so it reappears on revisit.
// A spawn cancels a prior removal of the same object.
func (g *GameState) MarkSpawn(scene, obj string, gx, gy int) {
	key := strings.ToLower(scene)
	if m := g.gone[key]; m != nil {
		delete(m, strings.ToLower(obj))
	}
	for i, s := range g.spawned[key] {
		if strings.EqualFold(s.Obj, obj) {
			g.spawned[key][i] = Spawn{Obj: obj, GX: gx, GY: gy}
			return
		}
	}
	g.spawned[key] = append(g.spawned[key], Spawn{Obj: obj, GX: gx, GY: gy})
}

// Spawns returns the objects created in a scene.
func (g *GameState) Spawns(scene string) []Spawn {
	return g.spawned[strings.ToLower(scene)]
}

// SaveData is a serialisable snapshot of the quest state plus the party's
// location — the whole save file.
type SaveData struct {
	Scene     string              `json:"scene"`
	Saved     string              `json:"saved"` // human-readable timestamp
	Cell      [2]int              `json:"cell"`
	Vars      map[string]int      `json:"vars"`
	CharVars  map[string]string   `json:"charVars"`
	Inventory []string            `json:"inventory"`
	Active    string              `json:"active"`
	UI        map[string]bool     `json:"ui"`
	Gone      map[string][]string `json:"gone"`
	Spawned   map[string][]Spawn  `json:"spawned"`
}

// Snapshot captures the full state for saving.
func (g *GameState) Snapshot(scene string, cell [2]int) SaveData {
	sd := SaveData{
		Scene: scene, Cell: cell,
		Vars: g.Vars, CharVars: g.CharVars,
		Inventory: g.Inventory, Active: g.Active, UI: g.UI,
		Gone: map[string][]string{}, Spawned: g.spawned,
	}
	for sc, m := range g.gone {
		for obj, v := range m {
			if v {
				sd.Gone[sc] = append(sd.Gone[sc], obj)
			}
		}
	}
	return sd
}

// Restore rebuilds a GameState from a snapshot.
func Restore(sd SaveData) *GameState {
	g := NewGameState()
	if sd.Vars != nil {
		g.Vars = sd.Vars
	}
	if sd.CharVars != nil {
		g.CharVars = sd.CharVars
	}
	g.Inventory = sd.Inventory
	g.Active = sd.Active
	if sd.UI != nil {
		g.UI = sd.UI
	}
	for sc, objs := range sd.Gone {
		for _, o := range objs {
			g.MarkGone(sc, o)
		}
	}
	if sd.Spawned != nil {
		g.spawned = sd.Spawned
	}
	return g
}

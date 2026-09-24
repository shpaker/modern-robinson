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
	Vars     map[string]int    // SetVar/AddVar quest flags and counters
	CharVars map[string]string // SetCharVar dialogue-variant selectors
	// Items is each character's inventory, keyed by lower-case name, in bar
	// order. The engine keeps one list per character (char+0x5a8, count
	// +0x738) and the bar shows the controlled one's: Friday's condom (AddItem
	// Frid, confr) is not the hero's to pick up.
	Items  map[string][]string
	Active string // SetActive: the item in hand (never a character)
	// ActiveChar is who the player controls. SetActive addresses either kind:
	// a character name switches control, anything else is picked up, and each
	// character has his own items (hand/handfr, condom/confr), so the two must
	// be tracked apart or Friday can never hold anything.
	ActiveChar string
	// InvRev counts the bar rebuilds: an item added or removed, or control
	// handed to the other character. The engine scrolls the rebuilt bar back to
	// its first slot (0x4044e0), so the bar watches this. Never saved.
	InvRev int

	gone    map[string]map[string]bool // scene -> object -> removed (DelObject)
	spawned map[string][]Spawn         // scene -> objects added (CreateObject)
	verts   map[string][]Vert          // scene -> SetVert overrides

	UI map[string]bool // SetMouse/SetMap/LockBar/SetBar/ShowCursor/Interrupt toggles
}

// NewGameState returns an empty state with initialised maps. UI toggles that
// default on: mouse input, the bar, and the cursor; the island map opens later
// (SetMap ON).
func NewGameState() *GameState {
	return &GameState{
		Vars:       map[string]int{},
		CharVars:   map[string]string{},
		Items:      map[string][]string{},
		ActiveChar: "Roby",
		gone:       map[string]map[string]bool{},
		spawned:    map[string][]Spawn{},
		verts:      map[string][]Vert{},
		UI:         map[string]bool{"mouse": true, "bar": true, "cursor": true},
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

// indexFold is the position of an item in a list, or -1.
func indexFold(list []string, item string) int {
	for i, it := range list {
		if strings.EqualFold(it, item) {
			return i
		}
	}
	return -1
}

// Inventory returns the controlled character's items, in bar order.
func (g *GameState) Inventory() []string {
	return g.InventoryOf(g.ActiveChar)
}

// InventoryOf returns a character's items, in bar order.
func (g *GameState) InventoryOf(char string) []string {
	return g.Items[strings.ToLower(char)]
}

// HasItem reports whether the controlled character carries the item.
func (g *GameState) HasItem(item string) bool {
	return indexFold(g.Inventory(), item) >= 0
}

// AddItem gives the controlled character an item (AddItem item): the script's
// performer, and every performer in the data is the one in control.
func (g *GameState) AddItem(item string) {
	g.AddItemTo(g.ActiveChar, item)
}

// AddItemTo appends an item to a character's list (AddItem char,item),
// ignoring duplicates. It never moves the hand: the new item lands after it.
func (g *GameState) AddItemTo(char, item string) {
	k := strings.ToLower(char)
	if item == "" || indexFold(g.Items[k], item) >= 0 {
		return
	}
	if g.Items == nil {
		g.Items = map[string][]string{}
	}
	g.Items[k] = append(g.Items[k], item)
	g.InvRev++
}

// DelItem takes an item from the controlled character (DeleteItem item).
func (g *GameState) DelItem(item string) {
	g.DelItemFrom(g.ActiveChar, item)
}

// DelItemFrom removes an item from a character's list. The hand is a slot of
// the bar, not an item (DeleteItem 0x405190): taking the item it points at
// sends it back to the first slot, the hand itself; taking one to its left
// leaves the slot alone, so the next item slides into the hand, and a slot
// past the end falls back to the first (the rebuild, 0x404877).
func (g *GameState) DelItemFrom(char, item string) {
	k := strings.ToLower(char)
	list := g.Items[k]
	i := indexFold(list, item)
	if i < 0 {
		return
	}
	sel := -1
	if strings.EqualFold(char, g.ActiveChar) {
		sel = indexFold(list, g.Active)
	}
	list = append(list[:i], list[i+1:]...)
	g.Items[k] = list
	g.InvRev++
	if sel < i {
		return // another character's list, or an item right of the hand
	}
	if sel == i || sel >= len(list) {
		sel = 0
	}
	g.Active = "hand"
	if len(list) > 0 {
		g.Active = list[sel]
	}
}

// SetActiveChar hands control to a character (SetActive Roby|Frid, the
// portrait). The bar is rebuilt from his list and keeps its selected slot, so
// the hand holds whatever sits there, or the first item when his list is
// shorter (0x404877).
func (g *GameState) SetActiveChar(char string) {
	sel := indexFold(g.Inventory(), g.Active)
	g.ActiveChar = char
	list := g.Inventory()
	if sel < 0 || sel >= len(list) {
		sel = 0
	}
	if len(list) > 0 {
		g.Active = list[sel]
	}
	g.InvRev++
}

// MarkGone records that an object was removed from a scene, so it stays gone
// across revisits. A removal cancels a prior spawn of the same object, or the
// next visit would create it again (the crab caught a second time).
func (g *GameState) MarkGone(scene, obj string) {
	scene, obj = strings.ToLower(scene), strings.ToLower(obj)
	if g.gone[scene] == nil {
		g.gone[scene] = map[string]bool{}
	}
	g.gone[scene][obj] = true
	kept := g.spawned[scene][:0]
	for _, s := range g.spawned[scene] {
		if !strings.EqualFold(s.Obj, obj) {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		delete(g.spawned, scene)
	} else {
		g.spawned[scene] = kept
	}
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

// SpawnAt returns where an object was last created in a scene, if it was.
func (g *GameState) SpawnAt(scene, obj string) (Spawn, bool) {
	for _, s := range g.spawned[strings.ToLower(scene)] {
		if strings.EqualFold(s.Obj, obj) {
			return s, true
		}
	}
	return Spawn{}, false
}

// MarkVert records a SetVert so the reshaped walk grid survives revisits: the
// scene is rebuilt from its .SCN on every entry, and without this the cells a
// script closed (the hut it just built) would open again.
func (g *GameState) MarkVert(scene string, gx, gy int, open bool) {
	key := strings.ToLower(scene)
	for i, v := range g.verts[key] {
		if v.GX == gx && v.GY == gy {
			g.verts[key][i].Open = open
			return
		}
	}
	g.verts[key] = append(g.verts[key], Vert{GX: gx, GY: gy, Open: open})
}

// Verts returns the SetVert overrides recorded for a scene.
func (g *GameState) Verts(scene string) []Vert {
	return g.verts[strings.ToLower(scene)]
}

// Vert is one runtime passability override recorded by SetVert.
type Vert struct {
	GX   int  `json:"gx"`
	GY   int  `json:"gy"`
	Open bool `json:"open"`
}

// SaveData is a serialisable snapshot of the quest state plus the party's
// location — the whole save file.
type SaveData struct {
	Scene      string              `json:"scene"`
	Saved      string              `json:"saved"` // human-readable timestamp
	Cell       [2]int              `json:"cell"`
	Vars       map[string]int      `json:"vars"`
	CharVars   map[string]string   `json:"charVars"`
	Items      map[string][]string `json:"items,omitempty"`
	Inventory  []string            `json:"inventory,omitempty"` // pre-split
	Active     string              `json:"active"`
	ActiveChar string              `json:"active_char,omitempty"`
	UI         map[string]bool     `json:"ui"`
	Gone       map[string][]string `json:"gone"`
	Spawned    map[string][]Spawn  `json:"spawned"`
	Verts      map[string][]Vert   `json:"verts,omitempty"`
	// Frid is Friday's own place on the scene. The engine writes every
	// character's cell, z and hidden flag into the save (0x41fba0: +0x254,
	// +0x258, +0x25c, +0x73c); saves made before the remake kept it have none.
	Frid *CharSave `json:"frid,omitempty"`
}

// CharSave is a character's place and visibility as a save records them.
type CharSave struct {
	Cell   [2]int `json:"cell"`
	Z      int    `json:"z"`
	Hidden bool   `json:"hidden"`
}

// Snapshot captures the full state for saving.
func (g *GameState) Snapshot(scene string, cell [2]int) SaveData {
	sd := SaveData{
		Scene: scene, Cell: cell,
		Vars: g.Vars, CharVars: g.CharVars,
		Items: g.Items, Active: g.Active, ActiveChar: g.ActiveChar,
		UI:   g.UI,
		Gone: map[string][]string{}, Spawned: g.spawned, Verts: g.verts,
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
	switch {
	case sd.Items != nil:
		g.Items = sd.Items
	case sd.Inventory != nil:
		g.Items = splitShared(sd.Inventory)
	}
	g.Active = sd.Active
	g.ActiveChar = sd.ActiveChar
	if g.ActiveChar == "" {
		g.ActiveChar = "Roby" // saves written before the split
	}
	if sd.UI != nil {
		g.UI = sd.UI
	}
	// The engine forces the mouse back on at the end of every load (0x4213cf),
	// so a save taken mid-script can never come back deaf.
	g.UI["mouse"] = true
	if sd.Spawned != nil {
		g.spawned = sd.Spawned
	}
	// A spawn clears its object's removal, so a save carrying both was written
	// by a build that let a later DelObject keep the spawn: the removal is the
	// newer of the two, and replaying it drops the stale spawn.
	for sc, objs := range sd.Gone {
		for _, o := range objs {
			g.MarkGone(sc, o)
		}
	}
	if sd.Verts != nil {
		g.verts = sd.Verts
	} else {
		g.verts = map[string][]Vert{}
	}
	return g
}

// fridOwn are the items that are Friday's in a save from before the split,
// when both characters shared one list: FRID.CHR starts her with handfr, and
// the only item a script hands her is her condom (AddItem Frid, confr).
var fridOwn = []string{"handfr", "confr"}

// splitShared rebuilds the per-character lists from a pre-split save's shared
// one: Friday's own items go to her, behind her bare hand, the rest stay the
// hero's.
func splitShared(shared []string) map[string][]string {
	roby := []string{}
	frid := []string{fridOwn[0]}
	for _, it := range shared {
		switch i := indexFold(fridOwn, it); {
		case i == 0: // her hand is already first
		case i > 0:
			frid = append(frid, it)
		default:
			roby = append(roby, it)
		}
	}
	return map[string][]string{"roby": roby, "frid": frid}
}

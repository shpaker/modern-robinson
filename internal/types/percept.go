package types

// Percept is the world as the hero takes it in: where he is, what is around
// him and what he carries, in the names the game itself gives them — object
// captions from TEXT.DAT, item names from BAR.BAR. It is everything a driver
// outside the game learns about the world: no quest variables, no scripts, no
// cells or internal keys.
type Percept struct {
	Where string `json:"where"` // one of the Where* values
	// Busy is what is under way ("идёт сцена", "ты идёшь"); "" when the hero
	// is free to act.
	Busy   string  `json:"busy,omitempty"`
	Around []Thing `json:"around,omitempty"` // what can be acted on
	Exits  []Thing `json:"exits,omitempty"`  // the ways out of the place
	Hands  string  `json:"hands,omitempty"`  // the item in hand; "Рука" is none
	// Carry is the rest of what the hero has on him, in bar order.
	Carry  []string   `json:"carry,omitempty"`
	Friday *Companion `json:"friday,omitempty"` // Friday, while she is with him
	Map    bool       `json:"map"`              // the island map can be opened
	// Hearing is the line on screen right now.
	Hearing string `json:"hearing,omitempty"`
}

// Where the hero is, as he would put it.
const (
	WhereIsland = "остров"
	WhereMap    = "карта острова"
	WherePuzzle = "головоломка"
	WhereScene  = "заставка"
	WherePause  = "пауза"
)

// Which side of the hero something is on.
const (
	SideLeft  = "слева"
	SideRight = "справа"
	SideNear  = "рядом"
)

// Thing is something the hero sees and can go to: its name and which side of
// him it is on. Things sharing a name are numbered from the left ("Камни 2").
type Thing struct {
	Name string `json:"name"`
	Side string `json:"side,omitempty"`
}

// Companion is Friday as the hero sees her.
type Companion struct {
	Side  string   `json:"side,omitempty"`
	Hands string   `json:"hands,omitempty"`
	Carry []string `json:"carry,omitempty"`
}

// Outcome is what came of an action: the lines shown while it played, whether
// the world answered at all, whether the hero may act again, and the world
// afterwards.
type Outcome struct {
	Said []string `json:"said,omitempty"`
	// Reacted is false when nothing happened: the game ignores an action it
	// has no answer for, and the player sees just that.
	Reacted bool `json:"reacted"`
	// Ready is false when the action is still playing out when the wait ends.
	Ready bool    `json:"ready"`
	Look  Percept `json:"look"`
}
